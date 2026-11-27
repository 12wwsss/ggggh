package export

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// Sheet 是一张表：表名 + 表头 + 行。
type Sheet struct {
	Name    string
	Headers []string
	Rows    [][]string
}

// XLSX 把一张表写成最小可用的 xlsx（内联字符串，无样式）。
func XLSX(w io.Writer, sheet string, headers []string, rows [][]string) error {
	return XLSXBook(w, []Sheet{{Name: sheet, Headers: headers, Rows: rows}})
}

// XLSXBook 把多张表写进同一个 xlsx，用于「一人一档」这类需要分 sheet 的导出。
func XLSXBook(w io.Writer, sheets []Sheet) error {
	if len(sheets) == 0 {
		sheets = []Sheet{{Name: "Sheet1"}}
	}
	zw := zip.NewWriter(w)
	defer zw.Close()

	write := func(name, body string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}

	var types, wbSheets, wbRels strings.Builder
	types.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	wbSheets.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>`)
	wbRels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)

	for i, sh := range sheets {
		n := i + 1
		name := sh.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Sheet%d", n)
		}
		fmt.Fprintf(&types, "\n<Override PartName=\"/xl/worksheets/sheet%d.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml\"/>", n)
		if n == 1 {
			fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d" r:id="rId1"/>`, xmlEsc(name), n)
		} else {
			fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlEsc(name), n, n)
		}
		fmt.Fprintf(&wbRels, "\n<Relationship Id=\"rId%d\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet\" Target=\"worksheets/sheet%d.xml\"/>", n, n)

		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
		writeRow := func(r int, cells []string) {
			sb.WriteString(fmt.Sprintf(`<row r="%d">`, r))
			for ci, v := range cells {
				sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
					colName(ci), r, xmlEsc(v)))
			}
			sb.WriteString(`</row>`)
		}
		if len(sh.Headers) > 0 {
			writeRow(1, sh.Headers)
		}
		for ri, row := range sh.Rows {
			writeRow(ri+2, row)
		}
		sb.WriteString(`</sheetData></worksheet>`)
		if err := write(fmt.Sprintf("xl/worksheets/sheet%d.xml", n), sb.String()); err != nil {
			return err
		}
	}
	types.WriteString("\n</Types>")
	wbSheets.WriteString(`</sheets></workbook>`)
	wbRels.WriteString("\n</Relationships>")

	if err := write("[Content_Types].xml", types.String()); err != nil {
		return err
	}
	if err := write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`); err != nil {
		return err
	}
	if err := write("xl/workbook.xml", wbSheets.String()); err != nil {
		return err
	}
	return write("xl/_rels/workbook.xml.rels", wbRels.String())
}

func colName(i int) string {
	out := ""
	n := i + 1
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
