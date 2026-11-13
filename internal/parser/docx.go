package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Docx 直接解析 docx：读 word/document.xml，按段落抽取文本，
// 不依赖外部进程，便于离线与信创环境部署。
type Docx struct{}

// Name 返回实现名。
func (d *Docx) Name() string { return "docx" }

// Parse 抽取段落文本。
func (d *Docx) Parse(_ context.Context, path string) ([]Block, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("打开 docx: %w", err)
	}
	defer zr.Close()

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("docx 中缺少 word/document.xml")
	}
	rc, err := docFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	blocks, err := decodeDocument(rc)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("docx 未解析出文本")
	}
	return blocks, nil
}

func decodeDocument(r io.Reader) ([]Block, error) {
	dec := xml.NewDecoder(r)

	var (
		blocks   []Block
		cur      strings.Builder
		inText   bool
		inPara   bool
		inCell   bool
		cellDone bool
	)

	flush := func() {
		text := strings.TrimSpace(cur.String())
		if text == "" {
			return
		}
		blocks = append(blocks, Block{Text: text})
		cur.Reset()
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析 document.xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inPara = true
				if !inCell {
					cur.Reset()
				}
			case "t":
				inText = true
			case "tab":
				if inPara {
					cur.WriteString("\t")
				}
			case "br", "cr":
				if inPara {
					cur.WriteString("\n")
				}
			case "tc":
				inCell = true
				cellDone = false
				cur.Reset()
			case "tr":
				cellDone = true
			}
		case xml.CharData:
			if inText && inPara {
				cur.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				inPara = false
				if !inCell {
					flush()
				}
			case "tc":
				inCell = false
				text := strings.TrimSpace(cur.String())
				if text != "" {
					blocks = append(blocks, Block{Text: text})
				}
				cur.Reset()
			case "tr":
				cellDone = true
				_ = cellDone
			}
		}
	}
	return blocks, nil
}
