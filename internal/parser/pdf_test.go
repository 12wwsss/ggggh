package parser

import (
	"strings"
	"testing"
)

func TestTextBlocksByBlankLine(t *testing.T) {
	text := "第一条 为保障煤矿安全生产，制定本规程。\n\n第二条 煤矿企业必须建立安全生产责任制。\n\n第三条 入井人员必须佩戴安全帽。"
	blocks := textBlocks(text)
	if len(blocks) != 3 {
		t.Fatalf("期望 3 段，实际 %d: %+v", len(blocks), blocks)
	}
	if !strings.Contains(blocks[1].Text, "第二条") {
		t.Errorf("第二段内容 = %q", blocks[1].Text)
	}
}

func TestTextBlocksGroupsContinuousLines(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("这是没有空行分隔的连续文本行，用于测试按行数切块。\n")
	}
	blocks := textBlocks(sb.String())
	if len(blocks) < 2 {
		t.Fatalf("连续行应切成多块，实际 %d 块", len(blocks))
	}
	for _, b := range blocks {
		if len(strings.Split(b.Text, "\n")) > 8 {
			t.Errorf("单块超过 8 行: %q", b.Text)
		}
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
