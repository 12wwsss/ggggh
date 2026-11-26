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

func TestSplitWithoutClauseFallsBackToParagraphs(t *testing.T) {
	// 建设方案七（二）：矿上安全管理制度、各岗位操作规程、典型事故案例
	// 大多没有“第X条”，也要能进知识库，不能被整体拒收。
	got, err := Split(blocks(
		"某某煤矿安全管理制度",
		"一、入井管理",
		"所有入井人员必须佩戴安全帽、矿灯和自救器，入井前接受检身。",
		"二、交接班管理",
		"交接班必须现场进行，交清设备运行、隐患处理和注意事项。",
	))
	if err != nil {
		t.Fatalf("无条款结构文档应按段落切分，实际报错: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("期望至少 2 个段落切片，实际 %d", len(got))
	}
	if got[0].ClauseNo != "" {
		t.Errorf("段落切片不应编造条款号，实际 %q", got[0].ClauseNo)
	}
	if got[0].Chapter == "" || got[0].Chapter == "正文" {
		t.Errorf("小标题应作为章节上下文，实际 %q", got[0].Chapter)
	}
	for i, c := range got {
		if c.Seq != i+1 {
			t.Errorf("第 %d 片 seq = %d", i, c.Seq)
		}
	}
}

func TestSplitFallsBackWhenTextIsProse(t *testing.T) {
	got, err := Split(blocks("事故案例：某矿 2023 年 5 月发生顶板事故，原因是未敲帮问顶。",
		"教训：进入作业地点前必须敲帮问顶，发现离层立即撤人。"))
	if err != nil {
		t.Fatalf("纯段落文本应可切分: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("短文本应合成 1 片，实际 %d", len(got))
	}
	if !strings.Contains(got[0].Content, "敲帮问顶") {
		t.Errorf("内容丢失: %q", got[0].Content)
	}
}

func TestSplitEmptyDocumentStillFails(t *testing.T) {
	if _, err := Split(blocks("   ", "\n")); err == nil {
		t.Fatal("空文档仍应报错，避免产生空文档")
	}
}

func TestSplitFAQBlocks(t *testing.T) {
	got, err := Split([]Block{
		{Text: "瓦斯超限怎么办？\n立即停电撤人并汇报调度。", Level: BlockLevelFAQ},
		{Text: "停风了怎么办？\n立即撤人、断电、设栅栏。", Level: BlockLevelFAQ},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("期望 2 条 FAQ 切片，实际 %d", len(got))
	}
	if got[0].ClauseNo != "FAQ-1" || got[1].ClauseNo != "FAQ-2" {
		t.Errorf("FAQ 条款号不符: %q, %q", got[0].ClauseNo, got[1].ClauseNo)
	}
	if !strings.HasPrefix(got[0].Content, "问：瓦斯超限怎么办？") {
		t.Errorf("FAQ 内容格式不符: %q", got[0].Content)
	}
	if got[0].Chapter != "高频问题" {
		t.Errorf("FAQ 章节 = %q", got[0].Chapter)
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
