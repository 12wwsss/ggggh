package parser

import (
	"strings"
	"testing"
)

func blocks(lines ...string) []Block {
	out := make([]Block, 0, len(lines))
	for _, l := range lines {
		out = append(out, Block{Text: l})
	}
	return out
}

func TestSplitClauses(t *testing.T) {
	got, err := Split(blocks(
		"煤矿安全规程",
		"目　录",
		"第一章 总则 ................................ 1",
		"第一章 总则",
		"第一条 为保障煤矿安全生产，制定本规程。",
		"第二条 煤矿企业必须建立安全生产责任制。",
		"第二章 顶板管理",
		"第一节 一般规定",
		"第五条 进入作业地点前必须敲帮问顶。",
		"第六条 采煤工作面上下安全出口必须畅通。",
	))
	if err != nil {
		t.Fatalf("Split 返回错误: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("期望 4 个切片（4 条正文），实际 %d", len(got))
	}
	want := []string{"第一条", "第二条", "第五条", "第六条"}
	for i, w := range want {
		if got[i].ClauseNo != w {
			t.Errorf("第 %d 个切片条款号 = %q，期望 %q", i, got[i].ClauseNo, w)
		}
	}
	if !strings.Contains(got[2].Chapter, "第二章") {
		t.Errorf("第五条所属章 = %q，期望包含第二章", got[2].Chapter)
	}
	if !strings.Contains(got[2].Chapter, "第一节") {
		t.Errorf("第五条所属节 = %q，期望包含第一节", got[2].Chapter)
	}
	if got[0].Seq != 1 || got[3].Seq != 4 {
		t.Errorf("seq 未按顺序编号: %d, %d", got[0].Seq, got[3].Seq)
	}
	if got[0].Part != 1 {
		t.Errorf("普通条款 part 应为 1，实际 %d", got[0].Part)
	}
}

func TestSplitDropsTOCAndPageNumbers(t *testing.T) {
	got, err := Split(blocks(
		"目　录",
		"第一章 总则 ............ 1",
		"- 12 -",
		"第一条 正文内容。",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("期望只保留 1 条正文，实际 %d 条: %+v", len(got), got)
	}
	if strings.Contains(got[0].Content, "....") || strings.Contains(got[0].Content, "12") {
		t.Errorf("目录行或页码未被过滤: %q", got[0].Content)
	}
}

func TestSplitLongClauseIntoSeveralParts(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("第九条 ")
	for i := 0; i < 40; i++ {
		sb.WriteString("本条规定的作业现场必须严格执行安全检查制度，发现隐患立即处理。")
	}
	got, err := Split(blocks(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("超长条款应切成多段，实际 %d 段", len(got))
	}
	for i, c := range got {
		if c.ClauseNo != "第九条" {
			t.Errorf("第 %d 段条款号 = %q，应与原文一致", i, c.ClauseNo)
		}
		if c.Part != i+1 {
			t.Errorf("第 %d 段 part = %d，期望 %d", i, c.Part, i+1)
		}
		if c.CharCount > maxChunkRunes {
			t.Errorf("第 %d 段长度 %d 超过上限 %d", i, c.CharCount, maxChunkRunes)
		}
	}
}

func TestSplitWithoutClauseFails(t *testing.T) {
	_, err := Split(blocks("这是一段没有条款结构的说明文字。", "第二段说明。"))
	if err == nil {
		t.Fatal("缺少“第X条”结构时应返回错误，避免把非规程文档发布成可检索资料")
	}
}

func TestSplitMergesParagraphsIntoOneClause(t *testing.T) {
	got, err := Split(blocks(
		"第一条 第一款内容。",
		"续行内容，仍属同一条。",
		"第二条 下一条内容。",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(got))
	}
	if !strings.Contains(got[0].Content, "续行内容") {
		t.Errorf("续行未并入第一条: %q", got[0].Content)
	}
}
