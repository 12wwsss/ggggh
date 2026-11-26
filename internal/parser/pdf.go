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

// Parse 抽取 PDF 文本，按行切成块。
//
// 这里刻意保持“一行一块”：条款切分（parser.Split）需要看到每一行才能识别
// “第X条”边界并把折行并回上一条；如果在这里先按段落合并，条款号就只剩
// 首行能匹配，整份规程会退化成两三个大块。
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

// textBlocks 把纯文本按行切块，去掉空行与分页符。
func textBlocks(text string) []Block {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\f", "\n")

	lines := strings.Split(text, "\n")
	blocks := make([]Block, 0, len(lines))
	for _, ln := range lines {
		if t := strings.TrimSpace(ln); t != "" {
			blocks = append(blocks, Block{Text: t})
		}
	}
	return blocks
}
