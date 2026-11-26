package parser

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FAQ 解析高频问答文档（CSV / JSON），每条记录生成一条切片。
//
// 建设方案七（二）要求知识库「支持导入 Word、PDF、FAQ 等文档」。
// Word 走 Docx/LibreOffice，CSV/JSON 走这里：
//
//	CSV：表头含「问题 / 答案」（或 question / answer），也接受无表头的两列；
//	JSON：[{"q":"…","a":"…"}]，键名兼容 question/answer 与中文。
type FAQ struct{}

// Name 返回实现名。
func (f *FAQ) Name() string { return "faq" }

// Parse 读取 FAQ 文件为 FAQ 块（BlockLevelFAQ）。
func (f *FAQ) Parse(_ context.Context, path string) ([]Block, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 FAQ 文件: %w", err)
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return parseFAQJSON(raw)
	default:
		return parseFAQCSV(raw)
	}
}

func parseFAQCSV(raw []byte) ([]Block, error) {
	// 去 BOM：Excel 导出的 CSV 带 UTF-8 BOM，会把表头读成 "\ufeff问题"。
	text := strings.TrimPrefix(string(raw), "\ufeff")
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("读取 FAQ 表格: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("FAQ 文件为空")
	}
	qi, ai, start := 0, 1, 0
	if isFAQHeader(records[0]) {
		qi, ai = -1, -1
		for i, c := range records[0] {
			switch strings.ToLower(strings.TrimSpace(c)) {
			case "问题", "提问", "q", "question":
				qi = i
			case "答案", "回答", "a", "answer":
				ai = i
			}
		}
		if qi < 0 || ai < 0 {
			qi, ai = 0, 1
		}
		start = 1
	}
	blocks := make([]Block, 0, len(records))
	for _, rec := range records[start:] {
		if len(rec) <= qi || len(rec) <= ai {
			continue
		}
		q := strings.TrimSpace(rec[qi])
		a := strings.TrimSpace(rec[ai])
		if q == "" || a == "" {
			continue
		}
		blocks = append(blocks, Block{Text: q + "\n" + a, Level: BlockLevelFAQ})
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("FAQ 表格里没有有效问答（需要「问题」「答案」两列）")
	}
	return blocks, nil
}

func isFAQHeader(rec []string) bool {
	for _, c := range rec {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "问题", "提问", "q", "question", "答案", "回答", "a", "answer":
			return true
		}
	}
	return false
}

func parseFAQJSON(raw []byte) ([]Block, error) {
	raw = []byte(strings.TrimPrefix(string(raw), "\ufeff"))
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("FAQ JSON 需为数组：[{\"q\":\"…\",\"a\":\"…\"}]")
	}
	blocks := make([]Block, 0, len(rows))
	for _, m := range rows {
		q := firstString(m, "q", "question", "问题", "提问")
		a := firstString(m, "a", "answer", "答案", "回答")
		if q == "" || a == "" {
			continue
		}
		blocks = append(blocks, Block{Text: q + "\n" + a, Level: BlockLevelFAQ})
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("FAQ JSON 里没有有效问答（需要 q / a 两个字段）")
	}
	return blocks, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
