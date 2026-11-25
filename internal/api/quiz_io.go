package api

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/export"
)

// exportQuiz 题库导出 xlsx。
func (s *Server) exportQuiz(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	list, err := s.Store.QuizQuestionsForExport(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	rows := make([][]string, 0, len(list))
	for _, q := range list {
		opts := make([]string, 4)
		for i := 0; i < 4 && i < len(q.Options); i++ {
			opts[i] = q.Options[i]
		}
		status := map[string]string{"published": "已通过", "pending": "待审核", "rejected": "已驳回"}[q.Status]
		rows = append(rows, []string{status, q.Job, q.Category, q.Stem,
			opts[0], opts[1], opts[2], opts[3], q.Answer, q.Analysis})
	}
	xlsxHeader(c, "题库")
	_ = export.XLSX(c.Writer, "题库",
		[]string{"状态", "岗位", "分类", "题干", "选项A", "选项B", "选项C", "选项D", "答案", "解析"}, rows)
}

// importQuiz 题库批量导入：CSV（UTF-8），表头
// 岗位,分类,题干,选项A,选项B,选项C,选项D,答案,解析
func (s *Server) importQuiz(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择 CSV 文件")
		return
	}
	if file.Size > 8<<20 {
		fail(c, http.StatusBadRequest, "文件不能超过 8MB")
		return
	}
	f, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取文件失败")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		fail(c, http.StatusBadRequest, "CSV 为空或格式不正确")
		return
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	get := func(rec []string, name string) string {
		i, ok := idx[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	if _, ok := idx["题干"]; !ok {
		fail(c, http.StatusBadRequest, "缺少表头：题干")
		return
	}
	created, skipped := 0, 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}
		stem := get(rec, "题干")
		if stem == "" {
			skipped++
			continue
		}
		opts := []string{}
		for _, k := range []string{"选项A", "选项B", "选项C", "选项D"} {
			if v := get(rec, k); v != "" {
				opts = append(opts, v)
			}
		}
		if len(opts) < 2 {
			skipped++
			continue
		}
		answer := strings.ToUpper(get(rec, "答案"))
		if answer == "" {
			skipped++
			continue
		}
		if _, err := s.Store.ImportQuizQuestion(c.Request.Context(), stem, opts, answer,
			get(rec, "解析"), get(rec, "岗位"), get(rec, "分类")); err != nil {
			skipped++
			continue
		}
		created++
	}
	ok(c, gin.H{"created": created, "skipped": skipped})
}

var _ = strconv.Itoa
