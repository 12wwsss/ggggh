package service

import (
	"strings"
	"testing"
)

func TestNormalizePointKeepsLeadingWord(t *testing.T) {
	cases := map[string]string{
		"1. 入井必须佩戴安全帽":   "入井必须佩戴安全帽",
		"（一）严禁酒后入井":      "严禁酒后入井",
		"一、瓦斯超限必须撤人":     "瓦斯超限必须撤人",
		"一切作业必须先检查顶板":    "一切作业必须先检查顶板",
		"  \t 交接班必须现场进行": "交接班必须现场进行",
	}
	for in, want := range cases {
		if got := normalizePoint(in); got != want {
			t.Errorf("normalizePoint(%q) = %q, 期望 %q", in, got, want)
		}
	}
	if got := normalizePoint("短"); got != "" {
		t.Errorf("过短文本应被丢弃，实际 %q", got)
	}
}

func TestRuleDraftSplitsSentences(t *testing.T) {
	d := ruleDraft("新版规程", "入井必须佩戴安全帽。严禁酒后入井；瓦斯超限必须停电撤人。")
	if len(d.Points) != 3 {
		t.Fatalf("期望 3 条要点，实际 %d: %v", len(d.Points), d.Points)
	}
	if !strings.Contains(d.Content, "新版规程") {
		t.Errorf("正文应带标题: %q", d.Content)
	}
	for _, p := range d.Points {
		if len([]rune(p)) > 42 {
			t.Errorf("要点超过 42 字: %q", p)
		}
	}
}

func TestRuleDraftHandlesSingleBlob(t *testing.T) {
	blob := strings.Repeat("现场作业必须严格执行敲帮问顶制度并确认支护完好", 20)
	d := ruleDraft("", blob)
	if len(d.Points) < 2 {
		t.Fatalf("无标点长文也应切出多条要点，实际 %d", len(d.Points))
	}
}

func TestParseDraftJSONRejectsFallback(t *testing.T) {
	if _, ok := parseDraftJSON(FallbackAnswer); ok {
		t.Fatal("兜底话术不应被解析成要点")
	}
	if _, ok := parseDraftJSON("```json\n{\"points\":[\"严禁任何形式的空顶作业\",\"作业前必须先完成临时支护\"],\"content\":\"\"}\n```"); !ok {
		t.Fatal("带代码块的 JSON 应能解析")
	}
}
