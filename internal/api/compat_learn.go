package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/store"
)

func letterIndex(ans string) int {
	ans = strings.ToUpper(strings.TrimSpace(ans))
	if ans == "" {
		return 0
	}
	idx := int(ans[0] - 'A')
	if idx < 0 || idx > 25 {
		return 0
	}
	return idx
}

func questionJSON(q model.QuizQuestion, withAnswer bool) gin.H {
	h := gin.H{
		"id": q.ID,
		"q":  q.Stem,
		"o":  store.StripOptionPrefix(q.Options),
	}
	if withAnswer {
		h["a"] = letterIndex(q.Answer)
		h["e"] = q.Analysis
		h["c"] = ""
		h["job"] = ""
	}
	return h
}

// compatQuestionBank 小程序题库：/api/questions?job=&count=&all=1&c=
func (s *Server) compatQuestionBank(c *gin.Context) {
	job := strings.TrimSpace(c.Query("job"))
	if job == "全部" || job == "通用" {
		job = ""
	}
	count, _ := strconv.Atoi(c.Query("count"))
	if count <= 0 {
		count = 5
	}
	all := c.Query("all") == "1"
	list, err := s.Store.ListCompatQuestions(c.Request.Context(), job, strings.TrimSpace(c.Query("c")), all, count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "题库加载失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, q := range list {
		items = append(items, gin.H{
			"id": q.ID, "q": q.Stem, "o": q.Options, "a": letterIndex(q.Answer),
			"e": q.Analysis, "c": q.Category, "job": q.Job,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatQuestionGroups 按岗位分组。
func (s *Server) compatQuestionGroups(c *gin.Context) {
	groups, err := s.Store.CompatQuestionGroups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "分组加载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": groups, "items": groups})
}

// compatQuizSubmit 记录一次练题成绩。
func (s *Server) compatQuizSubmit(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		Score int               `json:"score"`
		Total int               `json:"total"`
		Job   string            `json:"job"`
		Wrong []json.RawMessage `json:"wrong"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	raw := []byte("[]")
	if len(req.Wrong) > 0 {
		b, _ := json.Marshal(req.Wrong)
		raw = b
	}
	if err := s.Store.AddQuizAttempt(c.Request.Context(), u.ID, req.Job, req.Score, req.Total, raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "score": req.Score, "total": req.Total})
}

// compatQuizStats 答题统计。
func (s *Server) compatQuizStatsReal(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	attempts, best, err := s.Store.QuizAttemptStats(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "统计失败"})
		return
	}
	rate := 0.0
	if attempts > 0 {
		rate = float64(best) / 5.0
		if rate > 1 {
			rate = 1
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "mine": gin.H{"attempts": attempts, "best": best},
		"byTeam": []any{}, "passRate": rate, "total": attempts,
	})
}

// compatExamList 考试列表（含本人状态）。
func (s *Server) compatExamList(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var dept *int64
	if u.DeptID != nil {
		dept = u.DeptID
	}
	papers, _, err := s.Store.ListPapers(c.Request.Context(), "published", dept, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	records, _ := s.Store.ListMyExamRecords(c.Request.Context(), u.ID)
	byPaper := map[int64][]model.ExamRecord{}
	for _, r := range records {
		byPaper[r.PaperID] = append(byPaper[r.PaperID], r)
	}
	items := make([]gin.H, 0, len(papers))
	for _, p := range papers {
		recs := byPaper[p.ID]
		attempts := len(recs)
		status := "todo"
		lastScore := 0
		passed := false
		for _, r := range recs {
			if r.Status != "in_progress" {
				lastScore = r.Score
				if r.Passed {
					passed = true
				}
			}
		}
		maxAttempts := p.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 1
		}
		if passed || attempts >= maxAttempts {
			status = "done"
		}
		items = append(items, gin.H{
			"id": p.ID, "title": p.Title, "job": "", "questions": len(p.QuestionIDs),
			"minutes": p.DurationMin, "pass": p.PassScore, "total_score": p.TotalScore,
			"status": status, "attempts": attempts, "max_attempts": maxAttempts,
			"score": lastScore, "passed": passed,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatExamStart 开始考试，返回 sessionId 与题目。
func (s *Server) compatExamStart(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ExamID int64  `json:"examId"`
		Job    string `json:"job"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ExamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	paper, err := s.Store.GetPaper(c.Request.Context(), req.ExamID)
	if err != nil || paper.Status != "published" {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "试卷不存在或未发布"})
		return
	}
	if len(paper.DeptIDs) > 0 {
		okDept := false
		for _, d := range paper.DeptIDs {
			if u.DeptID != nil && d == *u.DeptID {
				okDept = true
			}
		}
		if !okDept {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "这份试卷不是发给你所在区队的"})
			return
		}
	}
	rec, err := s.Store.StartExam(c.Request.Context(), paper.ID, u.ID, paper.MaxAttempts)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "考试次数已用完"})
		return
	}
	if rec.Status != "in_progress" {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "这份试卷已交卷"})
		return
	}
	qs, err := s.Store.ListQuizQuestions(c.Request.Context(), nil, paper.QuestionIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "题目加载失败"})
		return
	}
	questions := make([]gin.H, 0, len(qs))
	for _, q := range qs {
		questions = append(questions, questionJSON(q, false))
	}
	deadline := rec.StartedAt.Add(time.Duration(paper.DurationMin) * time.Minute)
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "sessionId": rec.ID, "title": paper.Title,
		"questions": questions, "total": len(questions),
		"minutes": paper.DurationMin, "deadline": deadline.UnixMilli(),
	})
}

// compatExamSubmit 交卷判分。
func (s *Server) compatExamSubmit(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		SessionID int64 `json:"sessionId"`
		Answers   []int `json:"answers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	rec, err := s.Store.GetExamRecordByID(c.Request.Context(), req.SessionID)
	if err != nil || rec.UserID != u.ID {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "考试记录不存在"})
		return
	}
	if rec.Status != "in_progress" {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "已交卷，不能重复提交"})
		return
	}
	paper, err := s.Store.GetPaper(c.Request.Context(), rec.PaperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "试卷不存在"})
		return
	}
	qs, err := s.Store.ListQuizQuestions(c.Request.Context(), nil, paper.QuestionIDs)
	if err != nil || len(qs) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "题目缺失"})
		return
	}
	per := paper.TotalScore / len(qs)
	remainder := paper.TotalScore % len(qs)
	score := 0
	answers := make(map[string]string, len(qs))
	detail := make([]gin.H, 0, len(qs))
	for i, q := range qs {
		letter := ""
		if i < len(req.Answers) && req.Answers[i] >= 0 && req.Answers[i] < 6 {
			letter = string(rune('A' + req.Answers[i]))
			answers[strconv.FormatInt(q.ID, 10)] = letter
		}
		ok := strings.EqualFold(letter, q.Answer) && letter != ""
		if ok {
			score += per
			if i < remainder {
				score++
			}
		}
		d := questionJSON(q, true)
		d["ok"] = ok
		detail = append(detail, d)
	}
	passed := score >= paper.PassScore
	if err := s.Store.SubmitExam(c.Request.Context(), rec.ID, answers, score, passed); err != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "交卷失败，可能已提交"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"result": gin.H{
			"score": score, "total_score": paper.TotalScore, "passed": passed,
			"pass_score": paper.PassScore, "attempt_no": rec.AttemptNo,
		},
		"detail": detail,
	})
}

// compatLearnRead 记录学习完成。
func (s *Server) compatLearnRead(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ID      string `json:"id"`
		Minutes int    `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	// id 可能是 "art:8" 这类前缀，取数字部分
	idStr := req.ID
	if i := strings.Index(idStr, ":"); i >= 0 {
		idStr = idStr[i+1:]
	}
	itemID, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || itemID <= 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "recorded": false})
		return
	}
	_ = s.Store.MarkLearnRead(c.Request.Context(), u.ID, itemID, req.Minutes)
	c.JSON(http.StatusOK, gin.H{"ok": true, "recorded": true})
}
