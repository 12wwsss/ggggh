package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/store"
)

func (s *Server) listPapers(c *gin.Context) {
	p, size, offset := pageParams(c)
	status := c.Query("status")
	cl := claims(c)
	if cl.Role == model.RoleEmployee {
		status = "published"
	}
	var dept *int64
	if sc := scopeOf(c); sc != "all" && cl.DeptID > 0 {
		dept = &cl.DeptID
	}
	list, total, err := s.Store.ListPapers(c.Request.Context(), status, dept, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询试卷失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) getPaper(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	paper, err := s.Store.GetPaper(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "试卷不存在")
		return
	}
	questions, err := s.Store.ListQuizQuestions(c.Request.Context(), nil, paper.QuestionIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题目失败")
		return
	}
	if claims(c).Role == model.RoleEmployee {
		for i := range questions {
			questions[i].Answer = ""
			questions[i].Analysis = ""
		}
	}
	ok(c, gin.H{"paper": paper, "questions": questions})
}

type paperReq struct {
	Title       string  `json:"title" binding:"required"`
	DeptID      *int64  `json:"dept_id"`
	QuestionIDs []int64 `json:"question_ids" binding:"required"`
	PassScore   int     `json:"pass_score"`
	DurationMin int     `json:"duration_min"`
	Status      string  `json:"status"`
}

func (s *Server) createPaper(c *gin.Context) {
	var req paperReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.QuestionIDs) == 0 {
		fail(c, http.StatusBadRequest, "试卷标题与题目不能为空")
		return
	}
	if req.PassScore <= 0 {
		req.PassScore = 60
	}
	if req.DurationMin <= 0 {
		req.DurationMin = 60
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	cl := claims(c)
	id, err := s.Store.CreatePaper(c.Request.Context(), &model.ExamPaper{
		Title: req.Title, DeptID: req.DeptID, QuestionIDs: req.QuestionIDs,
		TotalScore: 100, PassScore: req.PassScore, DurationMin: req.DurationMin,
		Status: status, CreatedBy: &cl.UserID,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "新建试卷失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) publishPaper(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.SetPaperStatus(c.Request.Context(), id, "published"); err != nil {
		fail(c, http.StatusNotFound, "试卷不存在")
		return
	}
	ok(c, gin.H{"status": "published"})
}

func (s *Server) startExam(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	paper, err := s.Store.GetPaper(ctx, id)
	if err != nil {
		fail(c, http.StatusNotFound, "试卷不存在")
		return
	}
	if paper.Status != "published" {
		fail(c, http.StatusConflict, "试卷尚未发布")
		return
	}
	now := time.Now()
	if paper.StartAt != nil && now.Before(*paper.StartAt) {
		fail(c, http.StatusConflict, "考试尚未开始")
		return
	}
	if paper.EndAt != nil && now.After(*paper.EndAt) {
		fail(c, http.StatusConflict, "考试已结束")
		return
	}
	cl := claims(c)
	rec, err := s.Store.StartExam(ctx, id, cl.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "开始考试失败")
		return
	}
	if rec.Status != "in_progress" {
		fail(c, http.StatusConflict, "已交卷，不能重复作答")
		return
	}
	questions, err := s.Store.ListQuizQuestions(ctx, nil, paper.QuestionIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题目失败")
		return
	}
	for i := range questions {
		questions[i].Answer = ""
		questions[i].Analysis = ""
	}
	deadline := rec.StartedAt.Add(time.Duration(paper.DurationMin) * time.Minute)
	ok(c, gin.H{
		"record":           rec,
		"paper":            paper,
		"questions":        questions,
		"server_time":      now,
		"deadline":         deadline,
		"remaining_second": int(time.Until(deadline).Seconds()),
	})
}

func (s *Server) submitExam(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Answers map[string]string `json:"answers" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "answers 不能为空")
		return
	}
	ctx := c.Request.Context()
	cl := claims(c)
	paper, err := s.Store.GetPaper(ctx, id)
	if err != nil {
		fail(c, http.StatusNotFound, "试卷不存在")
		return
	}
	rec, err := s.Store.GetExamRecordByPaper(ctx, id, cl.UserID)
	if err != nil {
		fail(c, http.StatusConflict, "尚未开始考试")
		return
	}
	if rec.Status != "in_progress" {
		fail(c, http.StatusConflict, "已交卷，不能重复提交")
		return
	}
	questions, err := s.Store.ListQuizQuestions(ctx, nil, paper.QuestionIDs)
	if err != nil || len(questions) == 0 {
		fail(c, http.StatusInternalServerError, "试卷题目缺失")
		return
	}

	per := paper.TotalScore / len(questions)
	remainder := paper.TotalScore % len(questions)
	score := 0
	for i, q := range questions {
		if strings.EqualFold(strings.TrimSpace(req.Answers[strconv.FormatInt(q.ID, 10)]), q.Answer) {
			score += per
			if i < remainder {
				score++
			}
		}
	}
	passed := score >= paper.PassScore
	if err := s.Store.SubmitExam(ctx, rec.ID, req.Answers, score, passed); err != nil {
		if err == store.ErrConflict {
			fail(c, http.StatusConflict, "已交卷，不能重复提交")
			return
		}
		fail(c, http.StatusInternalServerError, "提交试卷失败")
		return
	}
	ok(c, gin.H{"score": score, "total_score": paper.TotalScore, "passed": passed, "pass_score": paper.PassScore})
}

func (s *Server) listExamRecords(c *gin.Context) {
	p, size, offset := pageParams(c)
	cl := claims(c)
	var dept, user *int64
	switch scopeOf(c) {
	case "dept":
		d := cl.DeptID
		dept = &d
	case "self":
		u := cl.UserID
		user = &u
	}
	var paperID *int64
	if v := c.Query("paper_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			paperID = &id
		}
	}
	list, total, err := s.Store.ListExamRecords(c.Request.Context(), paperID, dept, user, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询考试记录失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) exportExam(c *gin.Context) {
	cl := claims(c)
	var dept, user *int64
	switch scopeOf(c) {
	case "dept":
		d := cl.DeptID
		dept = &d
	case "self":
		u := cl.UserID
		user = &u
	}
	list, _, err := s.Store.ListExamRecords(c.Request.Context(), nil, dept, user, 10000, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="exam-records.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"部门", "姓名", "试卷", "得分", "是否通过", "状态", "交卷时间"})
	for _, r := range list {
		submitted := ""
		if r.SubmittedAt != nil {
			submitted = r.SubmittedAt.Format("2006-01-02 15:04:05")
		}
		_ = w.Write([]string{r.DeptName, r.UserName, r.PaperTitle, strconv.Itoa(r.Score),
			map[bool]string{true: "通过", false: "未通过"}[r.Passed], r.Status, submitted})
	}
	w.Flush()
}
