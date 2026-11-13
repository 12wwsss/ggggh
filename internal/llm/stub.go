package llm

import (
	"context"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"coalrag/internal/config"
	"coalrag/internal/vector"
)

// Stub 是开发期确定性桩模型：不联网、不占显存，用于跑通检索、
// 引用校验、留痕与评测链路。它按提示词里的【片段】结构作答，
// 因此走的是与真实模型完全相同的提示词与后处理路径。
type Stub struct {
	model string
	dim   int
}

// NewStubChat 构造对话桩。
func NewStubChat(c config.LLM) *Stub { return &Stub{model: c.Model} }

// NewStubEmbedder 构造嵌入桩。
func NewStubEmbedder(c config.Embed) *Stub { return &Stub{model: c.Model, dim: c.Dim} }

// Name 返回实现名。
func (s *Stub) Name() string {
	if s.dim > 0 {
		return "stub-embed:" + s.model
	}
	return "stub-chat:" + s.model
}

// Dim 返回向量维度。
func (s *Stub) Dim() int { return s.dim }

// Ping 桩实现永远可用。
func (s *Stub) Ping(context.Context) error { return nil }

var reFragmentHead = regexp.MustCompile(`(?s)^(\d+)】文档：《(.*?)》\s*条款：([^\n]*)\n内容：(.*)$`)

// fragment 是提示词中的一条资料片段。
type fragment struct {
	doc     string
	clause  string
	content string
}

// parseFragments 按【片段N】标记切分提示词。Go 的 regexp 不支持前瞻，
// 因此先用字符串定位切块，再用正则解析每块的头部。
func parseFragments(text string) []fragment {
	parts := strings.Split(text, "【片段")
	out := make([]fragment, 0, len(parts)-1)
	for _, p := range parts[1:] {
		if i := strings.Index(p, "\n规则："); i >= 0 {
			p = p[:i]
		}
		m := reFragmentHead.FindStringSubmatch(strings.TrimRight(p, "\n"))
		if m == nil {
			continue
		}
		out = append(out, fragment{
			doc:     strings.TrimSpace(m[2]),
			clause:  strings.TrimSpace(m[3]),
			content: strings.TrimSpace(m[4]),
		})
	}
	return out
}

// Chat 依据提示词中的片段生成结构化答案。
func (s *Stub) Chat(_ context.Context, messages []Message) (string, error) {
	var prompt strings.Builder
	for _, m := range messages {
		prompt.WriteString(m.Content)
		prompt.WriteString("\n")
	}
	text := prompt.String()

	frags := parseFragments(text)
	if len(frags) == 0 {
		return "资料中未找到明确依据，建议查阅《煤矿安全规程》原文或向通风管理部门核实。", nil
	}

	doc := frags[0].doc
	clause := frags[0].clause

	// 条文按句/分号拆成可执行动作，模拟“模型按编号输出处置步骤”的格式，
	// 让前端的动作清单与真实模型走同一条解析路径。
	steps := actionSteps(frags[0].content)
	var b strings.Builder
	fmt.Fprintf(&b, "根据《%s》%s的规定，现场按以下顺序执行：\n", doc, clause)
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "依据：[%s]", clause)
	return b.String(), nil
}

// actionSteps 把条文拆成不超过四条的可执行动作。
func actionSteps(content string) []string {
	content = strings.ReplaceAll(content, "\r", "")
	parts := strings.FieldsFunc(content, func(r rune) bool {
		return r == '。' || r == '；' || r == '\n'
	})
	steps := make([]string, 0, 4)
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, "，、, "))
		if len([]rune(p)) < 6 {
			continue
		}
		steps = append(steps, p)
		if len(steps) == 4 {
			break
		}
	}
	if len(steps) == 0 {
		steps = append(steps, firstSentences(content, 120))
	}
	return steps
}

func firstSentences(s string, limit int) string {
	s = strings.ReplaceAll(s, "\r", "")
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	cut := limit
	for i := limit; i > limit/2; i-- {
		if r[i-1] == '。' || r[i-1] == '；' || r[i-1] == '\n' {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(r[:cut])) + "……"
}

// Embed 用字符 n-gram 哈希生成确定性向量。
//
// 这不是语义嵌入，但保留了中文的字面相似性：同一段落与其改写问句
// 会有明显高于无关文本的余弦相似度，足以在无模型环境下验证检索与
// 阈值逻辑。只取 2/3-gram 而不取单字，是因为单字在中文里几乎处处
// 重合，会把无关文本的相似度抬到阈值之上。
func (s *Stub) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = s.embedOne(t)
	}
	return out, nil
}

func (s *Stub) embedOne(text string) []float32 {
	dim := s.dim
	if dim <= 0 {
		dim = 1024
	}
	vec := make([]float32, dim)

	// 带符号哈希：同一个 n-gram 的哈希高位决定正负号，使不同文本在
	// 同一下标上的碰撞相互抵消，而不是无差别叠加。
	add := func(ng string, w float32) {
		h := hash64(ng)
		if h&(1<<63) != 0 {
			vec[h%uint64(dim)] -= w
		} else {
			vec[h%uint64(dim)] += w
		}
	}

	runes := []rune(strings.ToLower(strings.Join(strings.Fields(text), "")))
	for i := 0; i+1 < len(runes); i++ {
		add(string(runes[i:i+2]), 1.0)
	}
	for i := 0; i+2 < len(runes); i++ {
		add(string(runes[i:i+3]), 0.5)
	}
	return vector.Normalize(vec)
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
