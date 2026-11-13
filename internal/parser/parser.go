// Package parser 负责把上传文件解析成文本块并按条款切分。
//
// 方案约定：Parser 接口默认 LibreOffice 子进程实现，docx 直解作为
// 回退；同一份文档在两种实现下切分结果一致。
package parser

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Block 是一段文本块。
type Block struct {
	Text  string `json:"text"`
	Level int    `json:"level"` // 1=章 2=节 3=条 0=正文
}

// Parser 把文件解析为文本块。
type Parser interface {
	Parse(ctx context.Context, path string) ([]Block, error)
	Name() string
}

// Pick 依据扩展名与外部工具可用性选择解析器。
func Pick(path string) (Parser, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx":
		if sofficeAvailable() {
			return &LibreOffice{}, nil
		}
		return &Docx{}, nil
	case ".txt", ".md", ".csv":
		return &Text{}, nil
	case ".doc", ".pdf", ".wps":
		if sofficeAvailable() {
			return &LibreOffice{}, nil
		}
		return nil, fmt.Errorf("解析 %s 需要安装 LibreOffice（当前环境未找到 soffice）", ext)
	default:
		return nil, fmt.Errorf("不支持的文件类型 %s", ext)
	}
}

// ByName 按名字返回解析器，供评测对比两种实现的切分结果。
func ByName(name string) (Parser, error) {
	switch name {
	case "docx":
		return &Docx{}, nil
	case "text":
		return &Text{}, nil
	case "libreoffice":
		if !sofficeAvailable() {
			return nil, fmt.Errorf("当前环境未找到 soffice")
		}
		return &LibreOffice{}, nil
	default:
		return nil, fmt.Errorf("未知解析器 %s", name)
	}
}
