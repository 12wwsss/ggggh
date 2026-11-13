package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
)

type askReq struct {
	Question string `json:"question" binding:"required"`
}

func (s *Server) ask(c *gin.Context) {
	var req askReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		fail(c, http.StatusBadRequest, "问题不能为空")
		return
	}
	cl := claims(c)
	user := &model.User{ID: cl.UserID, Role: cl.Role, Username: cl.Username}
	if cl.DeptID > 0 {
		dept := cl.DeptID
		user.DeptID = &dept
	}
	res, err := s.Svc.Ask(c.Request.Context(), user, req.Question, c.ClientIP())
	if err != nil {
		s.Log.Error("问答失败", "err", err)
		s.Metrics.Inc("qa_ask_total", map[string]string{"result": "error"})
		fail(c, http.StatusInternalServerError, "问答处理失败："+err.Error())
		return
	}
	result := "fallback"
	if res.Answered {
		result = "answered"
	}
	s.Metrics.Inc("qa_ask_total", map[string]string{"result": result})
	ok(c, res)
}

type searchReq struct {
	Question string  `json:"question" binding:"required"`
	TopK     int     `json:"top_k"`
	MinScore float64 `json:"min_score"`
}

func (s *Server) search(c *gin.Context) {
	var req searchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "问题不能为空")
		return
	}
	if req.TopK <= 0 {
		req.TopK = s.Cfg.Retrieval.TopK
	}
	if req.MinScore <= 0 {
		req.MinScore = 0
	}
	hits, err := s.Svc.Search(c.Request.Context(), req.Question, req.TopK, req.MinScore)
	if err != nil {
		fail(c, http.StatusInternalServerError, "检索失败："+err.Error())
		return
	}
	ok(c, gin.H{"hits": hits, "count": len(hits)})
}

func (s *Server) listQALogs(c *gin.Context) {
	p, size, offset := pageParams(c)
	cl := claims(c)
	var answered *bool
	switch c.Query("answered") {
	case "true":
		v := true
		answered = &v
	case "false":
		v := false
		answered = &v
	}
	list, total, err := s.Store.ListQALogs(c.Request.Context(), scopeOf(c), cl.UserID, cl.DeptID, answered, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询问答记录失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) getQALog(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	log, err := s.Store.GetQALog(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	cl := claims(c)
	switch scopeOf(c) {
	case "self":
		if log.UserID == nil || *log.UserID != cl.UserID {
			fail(c, http.StatusForbidden, "无权查看该记录")
			return
		}
	case "dept":
		if log.DeptID == nil || *log.DeptID != cl.DeptID {
			fail(c, http.StatusForbidden, "无权查看该记录")
			return
		}
	}
	ok(c, log)
}

func (s *Server) qaFeedback(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Feedback string `json:"feedback" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "反馈内容必填")
		return
	}
	switch req.Feedback {
	case "useful", "useless", "wrong_clause":
	default:
		fail(c, http.StatusBadRequest, "反馈只能是 useful、useless、wrong_clause")
		return
	}
	if err := s.Store.SetQALogFeedback(c.Request.Context(), id, req.Feedback); err != nil {
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}
