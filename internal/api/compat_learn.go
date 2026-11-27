package api

import (
	"encoding/json"
	"math"
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

// optionText 取选项文字（去前缀，越界返回空）。
func optionText(opts []string, idx int) string {
	clean := store.StripOptionPrefix(opts)
	if idx < 0 || idx >= len(clean) {
		return ""
	}
	return clean[idx]
}

// examDetail 交卷/回看用的逐题结果，带“你选”和“正确”的文字。
func examDetail(q model.QuizQuestion, letter string) gin.H {
	d := questionJSON(q, true)
	ok := letter != "" && strings.EqualFold(letter, q.Answer)
	d["ok"] = ok
	d["your"] = letter
	right := optionText(q.Options, letterIndex(q.Answer))
	d["right"] = right
	picked := ""
	if letter != "" {
		picked = optionText(q.Options, letterIndex(letter))
	}
	d["picked"] = picked
	return d
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
		Score  int               `json:"score"`
		Total  int               `json:"total"`
		Job    string            `json:"job"`
		Notice json.RawMessage   `json:"notice"`
		Wrong  []json.RawMessage `json:"wrong"`
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
	var broadcastID *int64
	if len(req.Notice) > 0 {
		raw := strings.Trim(strings.TrimSpace(string(req.Notice)), `"`)
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			broadcastID = &id
		}
	}
	if err := s.Store.AddQuizAttempt(c.Request.Context(), u.ID, req.Job, req.Score, req.Total, raw, broadcastID); err != nil {
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
	attempts, best, bestPct, avgPct, passRate, err := s.Store.QuizAttemptStatsFull(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "统计失败"})
		return
	}
	mine := gin.H{
		"attempts": attempts, "best": best, "bestPct": bestPct,
		"avg": int(math.Round(avgPct)), "passRate": int(math.Round(passRate)),
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "mine": mine,
		"byTeam": []any{}, "passRate": passRate / 100, "total": attempts,
	})
}

// compatTraining 新员工带培计划。
func (s *Server) compatTraining(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	plan, err := s.Store.GetTrainingPlan(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "plans": []any{}})
		return
	}
	stages := make([]gin.H, 0, len(plan.Stages))
	for _, st := range plan.Stages {
		stages = append(stages, gin.H{
			"name": st.Week, "goal": st.Goal, "tasks": st.Tasks, "done": st.Done,
		})
	}
	weak := []any{}
	if n, err := s.Store.WrongCount(c.Request.Context(), u.ID); err == nil && n > 0 {
		weak = append(weak, gin.H{"title": "错题待巩固", "from": "错题本 " + strconv.Itoa(n) + " 道"})
	}
	// 师傅/培训员/管理员在手机上也能写小结；本人只看
	canWrite := u.Role == model.RoleAdmin || u.Role == model.RoleSafety || u.Role == model.RoleTrainer ||
		(plan.MentorUserID != nil && *plan.MentorUserID == u.ID)
	notes := []gin.H{}
	if list, err := s.Store.ListTrainingNotes(c.Request.Context(), plan.ID); err == nil {
		for _, n := range list {
			notes = append(notes, gin.H{
				"id": n.ID, "kind": n.Kind, "period": n.Period, "content": n.Content,
				"author": n.Author, "at": n.CreatedAt.Format("01-02 15:04"),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "plans": []gin.H{{
		"id": plan.ID, "title": plan.Title, "mentor": plan.Mentor,
		"mentorUserId": plan.MentorUserID, "totalWeeks": plan.TotalWeeks, "target": plan.Target,
		"stages": stages, "weakPoints": weak, "canWrite": canWrite, "notes": notes,
	}}})
}

// compatWrongList 错题本列表。
func (s *Server) compatWrongList(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	list, err := s.Store.ListWrongQuestions(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "错题本加载失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, w := range list {
		items = append(items, gin.H{
			"id": w.ID, "q": w.Stem, "o": w.Options, "a": letterIndex(w.Answer),
			"e": w.Analysis, "c": w.Category, "job": w.Job, "times": w.Times,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatWrongSet 答错收录 / 答对移出。
func (s *Server) compatWrongSet(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ID int64 `json:"id"`
		OK bool  `json:"ok"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	if err := s.Store.SetWrongQuestion(c.Request.Context(), u.ID, req.ID, req.OK); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// compatExamRecords 我的历次考试（按试卷）。
func (s *Server) compatExamRecords(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var paperID *int64
	if v := c.Query("paperId"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			paperID = &id
		}
	}
	recs, _, err := s.Store.ListExamRecords(c.Request.Context(), paperID, nil, &u.ID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	items := make([]gin.H, 0, len(recs))
	for _, r := range recs {
		if r.Status == "in_progress" {
			continue
		}
		at := ""
		if r.SubmittedAt != nil {
			at = r.SubmittedAt.Format("01-02 15:04")
		}
		items = append(items, gin.H{
			"id": r.ID, "paper_id": r.PaperID, "score": r.Score, "passed": r.Passed,
			"attempt_no": r.AttemptNo, "submittedAt": at,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatExamReview 回看某次答卷。
func (s *Server) compatExamReview(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	rec, err := s.Store.GetExamRecordByID(c.Request.Context(), id)
	if err != nil || rec.UserID != u.ID {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "记录不存在"})
		return
	}
	paper, err := s.Store.GetPaper(c.Request.Context(), rec.PaperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "试卷不存在"})
		return
	}
	qs, err := s.Store.ListQuizQuestions(c.Request.Context(), nil, paper.QuestionIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "题目缺失"})
		return
	}
	detail := make([]gin.H, 0, len(qs))
	for _, q := range qs {
		letter := rec.Answers[strconv.FormatInt(q.ID, 10)]
		d := examDetail(q, letter)
		detail = append(detail, d)
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "result": gin.H{
			"score": rec.Score, "passed": rec.Passed, "attempt_no": rec.AttemptNo,
			"total_score": paper.TotalScore, "pass_score": paper.PassScore,
		},
		"detail": detail,
	})
}

// compatLearnItems 学习清单（含本人完成状态）。
func (s *Server) compatLearnItems(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var dept *int64
	if u.DeptID != nil {
		dept = u.DeptID
	}
	list, _, err := s.Store.ListLearnItems(c.Request.Context(), "published", dept, 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "学习清单加载失败"})
		return
	}
	recs, _, _ := s.Store.ListLearnRecords(c.Request.Context(), nil, &u.ID, nil, 500, 0)
	byItem := map[int64]model.LearnRecord{}
	for _, r := range recs {
		byItem[r.ItemID] = r
	}
	assigns, _ := s.Store.LearnAssignmentsForUser(c.Request.Context(), u.Post, u.DeptID)
	today := time.Now().Format("2006-01-02")
	items := make([]gin.H, 0, len(list))
	for _, it := range list {
		r := byItem[it.ID]
		due := assigns[it.ID]
		required := it.Required || due != ""
		overdue := due != "" && due < today && r.Status != "completed"
		items = append(items, gin.H{
			"id": it.ID, "title": it.Title, "category": it.Category, "post": it.Post,
			"content": it.Content, "clause_refs": it.ClauseRefs,
			"status": r.Status, "progress": r.Progress, "required": required,
			"due_at": due, "overdue": overdue,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatLearnItemDetail 学习详情 + 本人进度。
func (s *Server) compatLearnItemDetail(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	it, err := s.Store.GetLearnItem(c.Request.Context(), id)
	if err != nil || it.Status != "published" {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "学习条目不存在"})
		return
	}
	rec, _ := s.Store.GetLearnRecord(c.Request.Context(), id, u.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "item": it, "record": rec})
}

// compatLearnProgress 保存学习进度（100 即完成）。
func (s *Server) compatLearnProgress(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	var req struct {
		Progress    int `json:"progress"`
		DurationSec int `json:"duration_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	rec, err := s.Store.SaveLearnProgress(c.Request.Context(), id, u.ID, req.Progress, req.DurationSec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "record": rec})
}

// compatNoticeList 制度宣贯：以 broadcast_task 为准，带本人签收状态。
func (s *Server) compatNoticeList(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	list, err := s.Store.ListMyBroadcasts(c.Request.Context(), u.ID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "宣贯加载失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, t := range list {
		deadline := ""
		if t.Deadline != nil {
			deadline = t.Deadline.Format("01-02")
		}
		acked := t.MyStatus == "ack"
		rate := 0
		if acked {
			rate = 100
		}
		items = append(items, gin.H{
			"id": t.ID, "tag": "宣贯", "title": t.Title, "deadline": deadline,
			"body": []string{t.Content}, "points": []string{},
			"done":  map[bool]int{true: 1, false: 0}[acked],
			"total": 1, "rate": rate, "teams": []any{}, "questions": 0,
			"require_ack": t.RequireAck, "my_status": t.MyStatus,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatNoticeAck 宣贯签收。
func (s *Server) compatNoticeAck(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	if err := s.Store.AckBroadcast(c.Request.Context(), req.ID, u.ID, true); err != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "签收失败，可能不是这条宣贯的接收人"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "ack"})
}

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
		attempts := 0
		status := "todo"
		lastScore := 0
		passed := false
		submittedAt := ""
		for _, r := range recs {
			if r.Status == "in_progress" {
				continue
			}
			attempts++
			lastScore = r.Score
			if r.SubmittedAt != nil {
				submittedAt = r.SubmittedAt.Format("01-02 15:04")
			}
			if r.Passed {
				passed = true
			}
		}
		maxAttempts := p.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 1
		}
		if attempts >= maxAttempts {
			status = "done"
		}
		items = append(items, gin.H{
			"id": p.ID, "title": p.Title, "job": "", "questions": len(p.QuestionIDs),
			"minutes": p.DurationMin, "pass": p.PassScore, "total_score": p.TotalScore,
			"status": status, "attempts": attempts, "max_attempts": maxAttempts,
			"score": lastScore, "passed": passed, "mine": true,
			"submittedAt": submittedAt,
			"note":        "已考 " + strconv.Itoa(attempts) + "/" + strconv.Itoa(maxAttempts) + " 次",
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
		d := examDetail(q, letter)
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
