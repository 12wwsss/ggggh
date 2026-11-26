package parser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Text 解析纯文本。
type Text struct{}

// Name 返回实现名。
func (t *Text) Name() string { return "text" }

// Parse 按行切块。
func (t *Text) Parse(_ context.Context, path string) ([]Block, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var blocks []Block
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		blocks = append(blocks, Block{Text: line})
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("文件无有效文本")
	}
	return blocks, nil
}

// LibreOffice 通过 headless 子进程把任意文档转成 UTF-8 文本后解析。
type LibreOffice struct{}

// Name 返回实现名。
func (l *LibreOffice) Name() string { return "libreoffice" }

var sofficeCandidates = []string{
	`C:\Program Files\LibreOffice\program\soffice.exe`,
	`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
	`/usr/bin/soffice`,
	`/usr/lib/libreoffice/program/soffice`,
	`/Applications/LibreOffice.app/Contents/MacOS/soffice`,
}

func sofficePath() string {
	for _, p := range sofficeCandidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("soffice"); err == nil {
		return p
	}
	return ""
}

func sofficeAvailable() bool { return sofficePath() != "" }

// Parse 转换后按文本解析。
func (l *LibreOffice) Parse(ctx context.Context, path string) ([]Block, error) {
	bin := sofficePath()
	if bin == "" {
		return nil, fmt.Errorf("未找到 soffice")
	}
	outDir, err := os.MkdirTemp("", "coalrag-lo-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	cmd := exec.CommandContext(ctx, bin, "--headless", "--norestore",
		// 服务账号的 HOME（/opt/coalrag）通常是只读的，必须把 LibreOffice 的
		// 用户配置目录指到临时目录，否则启动时报 "User installation could not be completed"。
		"-env:UserInstallation=file://"+filepath.ToSlash(filepath.Join(outDir, "lo-profile")),
		"--convert-to", "txt:Text (encoded):UTF8", "--outdir", outDir, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("LibreOffice 转换失败: %v: %s", err, strings.TrimSpace(string(out)))
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	txtPath := filepath.Join(outDir, base+".txt")
	if _, err := os.Stat(txtPath); err != nil {
		return nil, fmt.Errorf("转换结果缺失: %w", err)
	}
	return (&Text{}).Parse(ctx, txtPath)
}
