package service

import (
	"strings"
	"testing"
)

func hitsFixture() []Retrieved {
	return []Retrieved{
		{
			ChunkID: 11, DocID: 1, DocTitle: "煤矿安全规程（节选）", ClauseNo: "第一百三十六条",
			Content: "采掘工作面风流中瓦斯浓度达到 1.5% 时，必须停止工作、切断电源、撤出人员，并报告矿调度室。",
			Score:   0.42,
		},
		{
			ChunkID: 12, DocID: 1, DocTitle: "煤矿安全规程（节选）", ClauseNo: "第七条",
			Content: "采煤工作面的上下安全出口必须保持畅通。",
			Score:   0.12,
		},
	}
}

func TestVerifyCitationsRejectsUnknownClause(t *testing.T) {
	raw := "根据规定：瓦斯超限必须撤人。\n依据：[第一百三十六条]"
	answer, cites, ok := verifyCitations(raw, hitsFixture())
	if !ok {
		t.Fatal("合法引用应通过校验")
	}
	if len(cites) != 1 || cites[0].ClauseNo != "第一百三十六条" {
		t.Fatalf("引用列表异常: %+v", cites)
	}
	if cites[0].ChunkID != 11 {
		t.Errorf("引用应带可反查原文的 chunk_id，实际 %d", cites[0].ChunkID)
	}
	if !strings.Contains(answer, "第一百三十六条") {
		t.Errorf("答案原文应保留条款号: %q", answer)
	}
}

func TestVerifyCitationsDropsFabricatedClause(t *testing.T) {
	raw := "依据：[第九十九条]"
	_, cites, ok := verifyCitations(raw, hitsFixture())
	if ok {
		t.Fatal("检索结果中不存在的条款号必须被判定为编造")
	}
	if len(cites) != 0 {
		t.Fatalf("编造引用不应进入引用列表: %+v", cites)
	}
}

func TestVerifyCitationsMixedKeepsOnlyValid(t *testing.T) {
	raw := "依据：[第一百三十六条]、[第九十九条]"
	_, cites, ok := verifyCitations(raw, hitsFixture())
	if !ok || len(cites) != 1 || cites[0].ClauseNo != "第一百三十六条" {
		t.Fatalf("应只保留可校验的条款，实际 ok=%v cites=%+v", ok, cites)
	}
}

func TestVerifyCitationsFallbackText(t *testing.T) {
	_, _, ok := verifyCitations("资料中未找到明确依据", hitsFixture())
	if ok {
		t.Fatal("兜底话术不应被当作有效回答")
	}
	if _, _, ok := verifyCitations("", hitsFixture()); ok {
		t.Fatal("空回答应判定为无依据")
	}
}

func TestBuildPromptContainsFragmentsAndQuestion(t *testing.T) {
	p := buildPrompt("瓦斯超限怎么处理？", hitsFixture(), 6000)
	for _, want := range []string{"【片段1】", "《煤矿安全规程（节选）》", "第一百三十六条", "问题：瓦斯超限怎么处理？"} {
		if !strings.Contains(p, want) {
			t.Errorf("提示词缺少 %q:\n%s", want, p)
		}
	}
}

func TestBuildPromptRespectsContextBudget(t *testing.T) {
	long := make([]Retrieved, 0, 4)
	for i := 0; i < 4; i++ {
		long = append(long, Retrieved{
			ClauseNo: "第一条",
			Content:  strings.Repeat("很长的条文内容", 100),
		})
	}
	p := buildPrompt("问题？", long, 300)
	if strings.Count(p, "【片段") > 1 {
		t.Errorf("上下文超出预算时不应继续拼接片段:\n%s", p)
	}
}

func TestSnippetTruncatesByRune(t *testing.T) {
	s := snippetOf(strings.Repeat("安", 200), 10)
	if len([]rune(s)) != 12 { // 10 字 + 省略号
		t.Errorf("截断长度异常: %d", len([]rune(s)))
	}
}

func TestSplitActionsExtractsNumberedSteps(t *testing.T) {
	raw := "根据《煤矿安全规程》第一百三十六条的规定，现场按以下顺序执行：\n" +
		"1. 停止用电钻打眼\n" +
		"2. 停止工作、切断电源、撤出人员\n" +
		"3. 报告矿调度室\n\n" +
		"依据：[第一百三十六条]"
	body, actions := splitActions(raw)
	if len(actions) != 3 {
		t.Fatalf("应抽出 3 条动作，实际 %d: %+v", len(actions), actions)
	}
	if actions[1] != "停止工作、切断电源、撤出人员" {
		t.Errorf("动作内容被改动: %q", actions[1])
	}
	if strings.Contains(body, "1. ") || !strings.Contains(body, "依据：") {
		t.Errorf("正文应保留说明与依据、去掉编号行: %q", body)
	}
	if strings.Count(body, "\n\n\n") > 0 {
		t.Errorf("正文空行未压缩: %q", body)
	}
}

func TestSplitActionsKeepsProseWhenSingleItem(t *testing.T) {
	raw := "1. 只有一条编号时按普通正文处理"
	body, actions := splitActions(raw)
	if actions != nil {
		t.Fatalf("单条编号不应拆成动作清单: %+v", actions)
	}
	if body != raw {
		t.Errorf("正文被改动: %q", body)
	}
}

func TestSplitActionsHandlesCircledNumbers(t *testing.T) {
	raw := "① 停止作业\n② 撤出人员\n③ 报告调度室"
	_, actions := splitActions(raw)
	if len(actions) != 3 || actions[0] != "停止作业" {
		t.Fatalf("带圈编号解析失败: %+v", actions)
	}
}
