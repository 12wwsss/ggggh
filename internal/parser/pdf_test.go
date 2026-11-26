package parser

import (
	"strings"
	"testing"
)

func TestTextBlocksOneLineEach(t *testing.T) {
	text := "煤矿安全规程（节选）\n\n第一条 为保障煤矿安全生产，制定本规程。\n第二条 煤矿企业必须建立安全生产责任制。"
	blocks := textBlocks(text)
	if len(blocks) != 3 {
		t.Fatalf("期望 3 行（空行丢弃），实际 %d: %+v", len(blocks), blocks)
	}
	if !strings.HasPrefix(blocks[1].Text, "第一条") {
		t.Errorf("第二块内容 = %q", blocks[1].Text)
	}
}

func TestTextBlocksDropsFormFeedAndBlanks(t *testing.T) {
	blocks := textBlocks("\f\n\n第一条 正文。\f\n")
	if len(blocks) != 1 {
		t.Fatalf("期望 1 块，实际 %d: %+v", len(blocks), blocks)
	}
	if strings.ContainsAny(blocks[0].Text, "\f") {
		t.Errorf("未清理分页符: %q", blocks[0].Text)
	}
}

func TestTextBlocksEmpty(t *testing.T) {
	if blocks := textBlocks("  \n\n\f\n "); len(blocks) != 0 {
		t.Fatalf("空白 PDF 不应产生块，实际 %d", len(blocks))
	}
}

// PDF 抽出的行要能走条款切分：折行并回上一条，而不是退化成段落。
func TestPDFLinesSplitIntoClauses(t *testing.T) {
	blocks := textBlocks(`煤矿安全规程（节选）
第一章 总则
第一条 为保障煤矿安全生产和从业人员的人身安全与健康，防止煤矿事故与
职业危害，根据《安全生产法》《矿山安全法》等，制定本规程。
第二条 煤矿企业必须建立安全生产责任制。
`)
	chunks, err := Split(blocks)
	if err != nil {
		t.Fatalf("Split 失败: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("期望 2 条切片，实际 %d: %+v", len(chunks), chunks)
	}
	if chunks[0].ClauseNo != "第一条" {
		t.Errorf("首条条款号 = %q", chunks[0].ClauseNo)
	}
	if !strings.Contains(chunks[0].Content, "职业危害") {
		t.Errorf("折行未并回第一条: %q", chunks[0].Content)
	}
}
