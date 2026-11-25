package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/export"
	"coalrag/internal/model"
)

func xlsxHeader(c *gin.Context, name string) {
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name)+".xlsx")
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func tsp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ts(*t)
}

func exportAllowed(c *gin.Context) bool {
	role := claims(c).Role
	if role != "admin" && role != "safety" && role != "dept_leader" {
		fail(c, http.StatusForbidden, "没有导出权限")
		return false
	}
	return true
}

// exportIssues 隐患台账。
func (s *Server) exportIssues(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	cl := claims(c)
	var dept, reporter *int64
	if cl.Role == "dept_leader" && cl.DeptID > 0 {
		d := cl.DeptID
		dept = &d
	}
	list, _, err := s.Store.ListIssues(c.Request.Context(), "", dept, reporter, "", 5000, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	rows := make([][]string, 0, len(list))
	for _, it := range list {
		overdue := "否"
		if it.Overdue {
			overdue = "是"
		}
		assignee := ""
		if it.Assignee != "" {
			assignee = it.Assignee
		}
		rows = append(rows, []string{
			it.Code, it.Title, it.CateName, compatIssueStatusText(it.Status), it.DeptName,
			it.Reporter, assignee, it.Severity, ts(it.CreatedAt), tsp(it.Deadline), tsp(it.ClosedAt), overdue,
		})
	}
	xlsxHeader(c, "隐患台账")
	_ = export.XLSX(c.Writer, "隐患台账",
		[]string{"编号", "标题", "类型", "状态", "区队", "上报人", "负责人", "严重度", "上报时间", "截止时间", "闭环时间", "是否超时"},
		rows)
}

// exportReports 报表导出：type=learning|wrong|issues|dept。
func (s *Server) exportReports(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	typ := c.DefaultQuery("type", "learning")
	switch typ {
	case "learning":
		rep, err := s.Store.LearningReport(c.Request.Context(), nil)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		rows := make([][]string, 0, len(rep.Rows))
		for _, r := range rep.Rows {
			rows = append(rows, []string{r.Name, r.Dept, strconv.Itoa(r.Completed), strconv.Itoa(r.Hours), r.LastAt})
		}
		xlsxHeader(c, "学习档案")
		_ = export.XLSX(c.Writer, "学习档案", []string{"姓名", "区队", "完成条目", "学时", "最近学习"}, rows)
	case "wrong":
		rep, err := s.Store.WrongReport(c.Request.Context(), 500)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		rows := make([][]string, 0, len(rep.Rows))
		for _, r := range rep.Rows {
			rows = append(rows, []string{r.Stem, r.Job, r.Category, strconv.Itoa(r.Times), strconv.Itoa(r.Users)})
		}
		xlsxHeader(c, "错题统计")
		_ = export.XLSX(c.Writer, "错题统计", []string{"题目", "岗位", "分类", "答错次数", "人数"}, rows)
	case "issues":
		rep, err := s.Store.IssueReport(c.Request.Context(), 30)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		rows := make([][]string, 0, len(rep.Daily))
		for _, d := range rep.Daily {
			rows = append(rows, []string{d.Date, strconv.Itoa(d.Reported), strconv.Itoa(d.Closed)})
		}
		xlsxHeader(c, "隐患趋势")
		_ = export.XLSX(c.Writer, "隐患趋势", []string{"日期", "上报数", "闭环数"}, rows)
	case "dept":
		rep, err := s.Store.IssueReport(c.Request.Context(), 14)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		rows := make([][]string, 0, len(rep.ByDept))
		for _, d := range rep.ByDept {
			rows = append(rows, []string{d.Dept, strconv.Itoa(d.Total), strconv.Itoa(d.Closed), strconv.Itoa(d.Open)})
		}
		xlsxHeader(c, "区队隐患统计")
		_ = export.XLSX(c.Writer, "区队隐患统计", []string{"区队", "总数", "已闭环", "未闭环"}, rows)
	default:
		fail(c, http.StatusBadRequest, "未知的导出类型")
	}
}

// exportExam 考试成绩。
func (s *Server) exportExamSheet(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	id, _ := strconv.ParseInt(c.Query("paper_id"), 10, 64)
	var recs []model.ExamRecord
	var err error
	if id > 0 {
		recs, err = s.Store.ExamRecordsForExport(c.Request.Context(), id)
	} else {
		recs, _, err = s.Store.ListExamRecords(c.Request.Context(), nil, nil, nil, 10000, 0)
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	title := "考试成绩"
	rows := make([][]string, 0, len(recs))
	for _, r := range recs {
		if r.PaperTitle != "" {
			title = r.PaperTitle
		}
		status := "已交卷"
		if r.Status == "in_progress" {
			status = "未交卷"
		}
		passed := "否"
		if r.Passed {
			passed = "是"
		}
		at := ""
		if r.SubmittedAt != nil {
			at = ts(*r.SubmittedAt)
		}
		rows = append(rows, []string{r.UserName, r.DeptName, strconv.Itoa(r.Score), passed,
			"第" + strconv.Itoa(r.AttemptNo) + "次", status, at})
	}
	xlsxHeader(c, title+"成绩")
	_ = export.XLSX(c.Writer, "成绩", []string{"姓名", "区队", "分数", "是否通过", "次数", "状态", "交卷时间"}, rows)
}

// exportAudit 审计日志。
func (s *Server) exportAuditSheet(c *gin.Context) {
	if !exportAllowed(c) {
		return
	}
	logs, _, err := s.Store.ListAudit(c.Request.Context(), c.Query("resource"), c.Query("action"), 5000, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	rows := make([][]string, 0, len(logs))
	for _, l := range logs {
		detail, _ := json.Marshal(l.Detail)
		rows = append(rows, []string{ts(l.CreatedAt), l.Username, l.Action, l.Resource, l.ResourceID, l.IP, string(detail)})
	}
	xlsxHeader(c, "审计日志")
	_ = export.XLSX(c.Writer, "审计日志", []string{"时间", "用户", "动作", "资源", "资源ID", "IP", "详情"}, rows)
}

var _ = model.Issue{}
