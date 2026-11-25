package export

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// XLSX 把一张表写成最小可用的 xlsx（内联字符串，无样式）。
func XLSX(w io.Writer, sheet string, headers []string, rows [][]string) error {
	if sheet == "" {
		sheet = "Sheet1"
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

	if err := write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`); err != nil {
		return err
	}
	if err := write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`); err != nil {
		return err
	}
	if err := write("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="`+xmlEsc(sheet)+`" sheetId="1" r:id="rId1"/></sheets>
</workbook>`); err != nil {
		return err
	}
	if err := write("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	writeRow := func(r int, cells []string) {
		sb.WriteString(fmt.Sprintf(`<row r="%d">`, r))
		for i, v := range cells {
			sb.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
				colName(i), r, xmlEsc(v)))
		}
		sb.WriteString(`</row>`)
	}
	writeRow(1, headers)
	for i, row := range rows {
		writeRow(i+2, row)
	}
	sb.WriteString(`</sheetData></worksheet>`)
	return write("xl/worksheets/sheet1.xml", sb.String())
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
