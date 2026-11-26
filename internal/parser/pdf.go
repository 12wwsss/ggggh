package parser

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Pdf 用 poppler 的 pdftotext 抽文本。
//
// 选择它而不是 LibreOffice：PDF 转 txt 走 Draw 常常丢段落、甚至直接失败；
// pdftotext 只做文本抽取，依赖也小（poppler-utils 约几 MB）。
// 服务器没有 pdftotext 时，Pick 会退回 LibreOffice；两者都没有才报错。
type Pdf struct{}

// Name 返回实现名。
func (p *Pdf) Name() string { return "pdftotext" }

var pdftotextCandidates = []string{
	"/usr/bin/pdftotext",
	"/usr/local/bin/pdftotext",
	`C:\Program Files\poppler\Library\bin\pdftotext.exe`,
	`C:\poppler\bin\pdftotext.exe`,
}

// pdftotextPath 返回 pdftotext 可执行文件路径，找不到返回空串。
func pdftotextPath() string {
	for _, p := range pdftotextCandidates {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("pdftotext"); err == nil {
		return p
	}
	return ""
}

// pdftotextAvailable 供 Pick 判断能否解析 PDF。
func pdftotextAvailable() bool { return pdftotextPath() != "" }

// Parse 抽取 PDF 文本并按段落切块；扫描件（无文字层）会明确报错，提示走 OCR。
func (p *Pdf) Parse(ctx context.Context, path string) ([]Block, error) {
	bin := pdftotextPath()
	if bin == "" {
		return nil, fmt.Errorf("未找到 pdftotext（poppler-utils）")
	}
	cmd := exec.CommandContext(ctx, bin, "-enc", "UTF-8", "-nopgbrk", path, "-")
	out, err := cmd.Output()
	if err != nil {
		msg := ""
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("pdftotext 转换失败: %v: %s", err, msg)
	}
	blocks := textBlocks(string(out))
	if len(blocks) == 0 {
		return nil, fmt.Errorf("PDF 未解析出文本（可能是扫描件或图片版，需要 OCR）")
	}
	return blocks, nil
}

// textBlocks 把纯文本切成段落块：空行分段；连续多行没有空行时每 8 行成一块，
// 避免整份 PDF 变成一个巨大段落、检索时只能整篇召回。
func textBlocks(text string) []Block {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\f", "\n\n")

	paras := make([]string, 0, 16)
	cur := make([]string, 0, 8)
	flush := func() {
		if s := strings.TrimSpace(strings.Join(cur, "\n")); s != "" {
			paras = append(paras, s)
		}
		cur = cur[:0]
	}

	lines := strings.Split(text, "\n")
	kept := 0
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			flush()
			kept = 0
			continue
		}
		cur = append(cur, t)
		kept++
		if kept >= 8 {
			flush()
			kept = 0
		}
	}
	flush()

	out := make([]Block, 0, len(paras))
	for _, p := range paras {
		out = append(out, Block{Text: p})
	}
	return out
}
