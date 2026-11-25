package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
)

func (s *Server) listLearnItems(c *gin.Context) {
	p, size, offset := pageParams(c)
	status := c.Query("status")
	var dept *int64
	cl := claims(c)
	if cl.Role == model.RoleEmployee {
		status = "published"
	}
	if sc := scopeOf(c); sc != "all" && cl.DeptID > 0 {
		dept = &cl.DeptID
	}
	list, total, err := s.Store.ListLearnItems(c.Request.Context(), status, dept, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询学习条目失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) getLearnItem(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	item, err := s.Store.GetLearnItem(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "学习条目不存在")
		return
	}
	cl := claims(c)
	rec, _ := s.Store.GetLearnRecord(c.Request.Context(), id, cl.UserID)
	ok(c, gin.H{"item": item, "record": rec})
}

type learnItemReq struct {
	Title      string   `json:"title" binding:"required"`
	Category   string   `json:"category"`
	Post       string   `json:"post"`
	DeptID     *int64   `json:"dept_id"`
	Content    string   `json:"content" binding:"required"`
	ClauseRefs []string `json:"clause_refs"`
	DocID      *int64   `json:"doc_id"`
	Sort       int      `json:"sort"`
	Required   bool     `json:"required"`
	Status     string   `json:"status"`
}

func (s *Server) createLearnItem(c *gin.Context) {
	var req learnItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "标题与内容必填")
		return
	}
	status := req.Status
	if status == "" {
		status = "published"
	}
	if req.Category == "" {
		req.Category = "规程"
	}
	id, err := s.Store.CreateLearnItem(c.Request.Context(), &model.LearnItem{
		Title: req.Title, Category: req.Category, Post: req.Post, DeptID: req.DeptID,
		Content: req.Content, ClauseRefs: req.ClauseRefs, DocID: req.DocID,
		Sort: req.Sort, Status: status, Required: req.Required,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "新建学习条目失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) updateLearnItem(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req learnItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "标题与内容必填")
		return
	}
	status := req.Status
	if status == "" {
		status = "published"
	}
	err := s.Store.UpdateLearnItem(c.Request.Context(), &model.LearnItem{
		ID: id, Title: req.Title, Category: req.Category, Post: req.Post, DeptID: req.DeptID,
		Content: req.Content, ClauseRefs: req.ClauseRefs, DocID: req.DocID,
		Sort: req.Sort, Status: status, Required: req.Required,
	})
	if err != nil {
		fail(c, http.StatusNotFound, "学习条目不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}

func (s *Server) saveLearnProgress(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Progress    int `json:"progress"`
		DurationSec int `json:"duration_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	if req.Progress < 0 || req.Progress > 100 {
		fail(c, http.StatusBadRequest, "progress 取值范围 0-100")
		return
	}
	if req.DurationSec < 0 || req.DurationSec > 86400 {
		fail(c, http.StatusBadRequest, "duration_sec 不合法")
		return
	}
	if _, err := s.Store.GetLearnItem(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, "学习条目不存在")
		return
	}
	cl := claims(c)
	rec, err := s.Store.SaveLearnProgress(c.Request.Context(), id, cl.UserID, req.Progress, req.DurationSec)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存学习进度失败")
		return
	}
	ok(c, rec)
}

func (s *Server) listLearnRecords(c *gin.Context) {
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
	var itemID *int64
	if v := c.Query("item_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			itemID = &id
		}
	}
	list, total, err := s.Store.ListLearnRecords(c.Request.Context(), dept, user, itemID, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询学习记录失败")
		return
	}
	paged(c, list, total, p, size)
}

// exportLearn 导出学习完成情况。
func (s *Server) exportLearn(c *gin.Context) {
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
	list, _, err := s.Store.ListLearnRecords(c.Request.Context(), dept, user, nil, 10000, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="learn-records.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"姓名", "条目ID", "状态", "进度", "累计时长(秒)", "开始时间", "完成时间"})
	for _, r := range list {
		start, done := "", ""
		if r.StartedAt != nil {
			start = r.StartedAt.Format("2006-01-02 15:04:05")
		}
		if r.CompletedAt != nil {
			done = r.CompletedAt.Format("2006-01-02 15:04:05")
		}
		_ = w.Write([]string{r.UserName, strconv.FormatInt(r.ItemID, 10), r.Status,
			strconv.Itoa(r.Progress), strconv.Itoa(r.DurationSec), start, done})
	}
	w.Flush()
}

// listAllQuestions 返回题库全部题目（管理端组卷用），可按 item_id 过滤。
func (s *Server) listAllQuestions(c *gin.Context) {
	var itemID *int64
	if v := c.Query("item_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			itemID = &id
		}
	}
	list, err := s.Store.ListQuizQuestionsAdmin(c.Request.Context(), itemID, c.Query("status"))
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题库失败")
		return
	}
	ok(c, list)
}

func (s *Server) listQuizQuestions(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	q := id
	list, err := s.Store.ListQuizQuestions(c.Request.Context(), &q, nil)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询题目失败")
		return
	}
	cl := claims(c)
	if cl.Role == model.RoleEmployee {
		for i := range list {
			list[i].Answer = ""
			list[i].Analysis = ""
		}
	}
	ok(c, list)
}

type quizReq struct {
	Stem       string   `json:"stem" binding:"required"`
	Options    []string `json:"options" binding:"required"`
	Answer     string   `json:"answer" binding:"required"`
	Analysis   string   `json:"analysis"`
	Difficulty string   `json:"difficulty"`
}

func (s *Server) setQuizQuestionStatus(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	if req.Status != "published" && req.Status != "rejected" && req.Status != "pending" {
		fail(c, http.StatusBadRequest, "状态不合法")
		return
	}
	if err := s.Store.SetQuizQuestionStatus(c.Request.Context(), id, req.Status); err != nil {
		fail(c, http.StatusNotFound, "题目不存在")
		return
	}
	ok(c, gin.H{"id": id, "status": req.Status})
}
func (s *Server) createQuizQuestion(c *gin.Context) {
	var req quizReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Options) < 2 {
		fail(c, http.StatusBadRequest, "题干、选项、答案必填，选项至少两个")
		return
	}
	if req.Difficulty == "" {
		req.Difficulty = "normal"
	}
	itemID, _ := strconv.ParseInt(c.Query("item_id"), 10, 64)
	var ptr *int64
	if itemID > 0 {
		ptr = &itemID
	}
	id, err := s.Store.CreateQuizQuestion(c.Request.Context(), &model.QuizQuestion{
		ItemID: ptr, Stem: req.Stem, Options: req.Options, Answer: req.Answer,
		Analysis: req.Analysis, Difficulty: req.Difficulty, Status: "pending",
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "新建题目失败")
		return
	}
	ok(c, gin.H{"id": id})
}

// submitQuiz 提交小测并返回逐题结果。
func (s *Server) submitQuiz(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Answers map[string]string `json:"answers" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Answers) == 0 {
		fail(c, http.StatusBadRequest, "answers 不能为空")
		return
	}
	itemID := id
	questions, err := s.Store.ListQuizQuestions(c.Request.Context(), &itemID, nil)
	if err != nil || len(questions) == 0 {
		fail(c, http.StatusNotFound, "该学习条目没有小测题")
		return
	}
	type result struct {
		QuestionID int64  `json:"question_id"`
		Correct    bool   `json:"correct"`
		Answer     string `json:"answer"`
		YourAnswer string `json:"your_answer"`
		Analysis   string `json:"analysis,omitempty"`
	}
	correct := 0
	out := make([]result, 0, len(questions))
	for _, q := range questions {
		your := strings.TrimSpace(req.Answers[strconv.FormatInt(q.ID, 10)])
		okAns := strings.EqualFold(your, q.Answer)
		if okAns {
			correct++
		}
		out = append(out, result{QuestionID: q.ID, Correct: okAns, Answer: q.Answer,
			YourAnswer: your, Analysis: q.Analysis})
	}
	score := 0
	if len(questions) > 0 {
		score = correct * 100 / len(questions)
	}
	ok(c, gin.H{
		"score":     score,
		"correct":   correct,
		"total":     len(questions),
		"passed":    score >= 60,
		"questions": out,
	})
}
