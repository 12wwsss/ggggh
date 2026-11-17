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
	DeptIDs     []int64 `json:"dept_ids"`
	QuestionIDs []int64 `json:"question_ids" binding:"required"`
	PassScore   int     `json:"pass_score"`
	DurationMin int     `json:"duration_min"`
	Status      string  `json:"status"`
	MaxAttempts int     `json:"max_attempts"`
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
	if req.MaxAttempts <= 0 {
		req.MaxAttempts = 1
	}
	if req.MaxAttempts > 10 {
		fail(c, http.StatusBadRequest, "考试次数最多 10 次")
		return
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	cl := claims(c)
	var firstDept *int64
	if len(req.DeptIDs) > 0 {
		firstDept = &req.DeptIDs[0]
	} else {
		firstDept = req.DeptID
	}
	id, err := s.Store.CreatePaper(c.Request.Context(), &model.ExamPaper{
		Title: req.Title, DeptID: firstDept, DeptIDs: req.DeptIDs, QuestionIDs: req.QuestionIDs,
		TotalScore: 100, PassScore: req.PassScore, DurationMin: req.DurationMin,
		Status: status, CreatedBy: &cl.UserID, MaxAttempts: req.MaxAttempts,
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
	// 发布时可以顺带设置考试次数与发布部门
	var req struct {
		MaxAttempts *int     `json:"max_attempts"`
		DeptIDs     *[]int64 `json:"dept_ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.MaxAttempts != nil {
		if *req.MaxAttempts <= 0 || *req.MaxAttempts > 10 {
			fail(c, http.StatusBadRequest, "考试次数需在 1–10 之间")
			return
		}
		if err := s.Store.SetPaperMaxAttempts(c.Request.Context(), id, *req.MaxAttempts); err != nil {
			fail(c, http.StatusConflict, "只有草稿状态可以修改考试次数")
			return
		}
	}
	if req.DeptIDs != nil {
		ids := *req.DeptIDs
		if len(ids) > 20 {
			fail(c, http.StatusBadRequest, "部门数量过多")
			return
		}
		if err := s.Store.SetPaperDepts(c.Request.Context(), id, ids); err != nil {
			fail(c, http.StatusConflict, "只有草稿状态可以修改发布部门")
			return
		}
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
	rec, err := s.Store.StartExam(ctx, id, cl.UserID, paper.MaxAttempts)
	if err != nil {
		if err == store.ErrConflict {
			fail(c, http.StatusConflict, "考试次数已用完")
			return
		}
		fail(c, http.StatusInternalServerError, "开始考试失败")
		return
	}
	questions, err := s.Store.ListQuizQuestions(ctx, nil, paper.QuestionIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题目失败")
		return
	}

	used, _ := s.Store.CountExamAttempts(ctx, id, cl.UserID)
	remaining := paper.MaxAttempts - used
	if remaining < 0 {
		remaining = 0
	}

	// 已交卷：不再返回 409，而是作为“查看成绩与答案”返回。
	// 题目保留 answer/analysis，供复习；只有本人能取到自己的记录。
	if rec.Status != "in_progress" {
		ok(c, gin.H{
			"record":             rec,
			"paper":              paper,
			"questions":          questions,
			"submitted":          true,
			"max_attempts":       paper.MaxAttempts,
			"used_attempts":      used,
			"remaining_attempts": remaining,
			"server_time":        now,
			"remaining_second":   0,
		})
		return
	}

	for i := range questions {
		questions[i].Answer = ""
		questions[i].Analysis = ""
	}
	deadline := rec.StartedAt.Add(time.Duration(paper.DurationMin) * time.Minute)
	ok(c, gin.H{
		"record":             rec,
		"paper":              paper,
		"questions":          questions,
		"max_attempts":       paper.MaxAttempts,
		"used_attempts":      used,
		"remaining_attempts": remaining,
		"server_time":        now,
		"deadline":           deadline,
		"remaining_second":   int(time.Until(deadline).Seconds()),
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
		fail(c, http.StatusConflict, "没有进行中的考试（可能已交卷）")
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

// getExamRecord 查看本人成绩与答案（不新建考试次数）。
// 带 ?attempt=N 看第 N 次；不带则看最近一次。返回全部历次成绩摘要。
func (s *Server) getExamRecord(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	cl := claims(c)
	paper, err := s.Store.GetPaper(ctx, id)
	if err != nil {
		fail(c, http.StatusNotFound, "试卷不存在")
		return
	}
	attemptNo := 0
	if v := c.Query("attempt"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			attemptNo = n
		}
	}
	var rec *model.ExamRecord
	if attemptNo > 0 {
		rec, err = s.Store.GetExamAttempt(ctx, id, cl.UserID, attemptNo)
		if err != nil {
			fail(c, http.StatusNotFound, "没有这次考试记录")
			return
		}
	} else {
		rec, err = s.Store.GetLatestExamRecord(ctx, id, cl.UserID)
		if err != nil {
			fail(c, http.StatusNotFound, "还没有参加考试")
			return
		}
	}
	questions, err := s.Store.ListQuizQuestions(ctx, nil, paper.QuestionIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题目失败")
		return
	}
	if rec.Status == "in_progress" {
		for i := range questions {
			questions[i].Answer = ""
			questions[i].Analysis = ""
		}
	}
	attempts, err := s.Store.ListExamAttempts(ctx, id, cl.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询历次成绩失败")
		return
	}
	used, _ := s.Store.CountExamAttempts(ctx, id, cl.UserID)
	remaining := paper.MaxAttempts - used
	if remaining < 0 {
		remaining = 0
	}
	ok(c, gin.H{
		"record":             rec,
		"paper":              paper,
		"questions":          questions,
		"attempts":           attempts,
		"max_attempts":       paper.MaxAttempts,
		"used_attempts":      used,
		"remaining_attempts": remaining,
	})
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
