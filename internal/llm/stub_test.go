package llm

import (
	"context"
	"math"
	"strings"
	"testing"

	"coalrag/internal/config"
)

const promptFixture = `你是煤矿安全员智能助手。请只依据下面给出的资料片段回答，禁止编造条款号。

资料片段：
【片段1】文档：《煤矿安全规程（节选）》 条款：第一百三十六条
内容：采掘工作面风流中瓦斯浓度达到 1.5% 时，必须停止工作、切断电源、撤出人员。
【片段2】文档：《煤矿安全规程（节选）》 条款：第七条
内容：采煤工作面的上下安全出口必须保持畅通。
规则：
1. 只能使用上述片段中的内容；
问题：瓦斯浓度达到1.5%时怎么办？`

func TestParseFragments(t *testing.T) {
	frags := parseFragments(promptFixture)
	if len(frags) != 2 {
		t.Fatalf("期望解析出 2 个片段，实际 %d: %+v", len(frags), frags)
	}
	if frags[0].doc != "煤矿安全规程（节选）" || frags[0].clause != "第一百三十六条" {
		t.Errorf("片段头解析错误: %+v", frags[0])
	}
	if !strings.Contains(frags[0].content, "1.5%") {
		t.Errorf("片段正文解析错误: %q", frags[0].content)
	}
	if strings.Contains(frags[1].content, "规则") || strings.Contains(frags[1].content, "问题") {
		t.Errorf("片段正文混入了提示词尾部: %q", frags[1].content)
	}
}

func TestStubChatAnswersWithFirstFragment(t *testing.T) {
	s := NewStubChat(config.Default().LLM)
	got, err := s.Chat(context.Background(), []Message{{Role: "user", Content: promptFixture}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "第一百三十六条") {
		t.Errorf("答案应引用首个片段条款号: %q", got)
	}
	if !strings.Contains(got, "依据：") {
		t.Errorf("答案应包含“依据：”行: %q", got)
	}
	if !strings.Contains(got, "《煤矿安全规程（节选）》") {
		t.Errorf("答案应包含文档名: %q", got)
	}
}

func TestStubChatFallbackWithoutFragments(t *testing.T) {
	s := NewStubChat(config.Default().LLM)
	got, _ := s.Chat(context.Background(), []Message{{Role: "user", Content: "问题：你好"}})
	if !strings.Contains(got, "资料中未找到明确依据") {
		t.Fatalf("无片段时应返回兜底话术: %q", got)
	}
}

func TestStubEmbedIsDeterministicAndNormalized(t *testing.T) {
	s := NewStubEmbedder(config.Default().Embed)
	a, err := s.Embed(context.Background(), []string{"采煤工作面上下安全出口必须畅通"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Embed(context.Background(), []string{"采煤工作面上下安全出口必须畅通"})
	if len(a[0]) != s.Dim() {
		t.Fatalf("维度 = %d，期望 %d", len(a[0]), s.Dim())
	}
	var norm float64
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatal("同一文本的向量应完全一致（确定性）")
		}
		norm += float64(a[0][i]) * float64(a[0][i])
	}
	if math.Abs(norm-1) > 1e-4 {
		t.Fatalf("向量应做 L2 归一化，模长 = %f", norm)
	}
}

func TestStubEmbedRanksRelatedTextHigher(t *testing.T) {
	s := NewStubEmbedder(config.Default().Embed)
	vecs, _ := s.Embed(context.Background(), []string{
		"瓦斯浓度达到1.5%时怎么办？",
		"采掘工作面风流中瓦斯浓度达到 1.5% 时，必须停止工作、切断电源、撤出人员。",
		"采煤工作面的上下安全出口必须保持畅通。",
	})
	related := dot(vecs[0], vecs[1])
	unrelated := dot(vecs[0], vecs[2])
	if related <= unrelated {
		t.Fatalf("相关文本相似度 %.3f 应高于无关文本 %.3f", related, unrelated)
	}
	if related < 0.15 {
		t.Fatalf("相关文本相似度过低: %.3f", related)
	}
}

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}
