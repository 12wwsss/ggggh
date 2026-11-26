package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFAQCSVWithHeader(t *testing.T) {
	p := writeTemp(t, "faq.csv", "\ufeff问题,答案\n瓦斯超限怎么办？,立即停电撤人并汇报调度。\n停风了怎么办？,立即撤人、断电、设栅栏。\n")
	blocks, err := (&FAQ{}).Parse(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("期望 2 条 FAQ，实际 %d", len(blocks))
	}
	if blocks[0].Level != BlockLevelFAQ {
		t.Errorf("FAQ 块缺少标记，level=%d", blocks[0].Level)
	}
	if blocks[0].Text != "瓦斯超限怎么办？\n立即停电撤人并汇报调度。" {
		t.Errorf("FAQ 文本 = %q", blocks[0].Text)
	}
}

func TestFAQCSVWithoutHeader(t *testing.T) {
	p := writeTemp(t, "faq.csv", "什么是敲帮问顶,进入作业地点前先敲击顶板听声音判断是否离层。\n")
	blocks, err := (&FAQ{}).Parse(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("无表头两列也应识别，实际 %d 条", len(blocks))
	}
}

func TestFAQJSON(t *testing.T) {
	p := writeTemp(t, "faq.json", `[{"q":"瓦斯超限怎么办？","a":"停电撤人汇报"},{"question":"停风怎么办","answer":"撤人断电"}]`)
	blocks, err := (&FAQ{}).Parse(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(blocks))
	}
}

func TestFAQJSONInvalid(t *testing.T) {
	p := writeTemp(t, "bad.json", `{"a":1}`)
	if _, err := (&FAQ{}).Parse(context.Background(), p); err == nil {
		t.Fatal("非数组 JSON 应报错")
	}
}

func TestFAQEmptyCSV(t *testing.T) {
	p := writeTemp(t, "empty.csv", "问题,答案\n,,\n")
	if _, err := (&FAQ{}).Parse(context.Background(), p); err == nil {
		t.Fatal("没有有效问答应报错")
	}
}
