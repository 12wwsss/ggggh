package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

// compatQaTransfer 小程序问答页「转人工」。
func (s *Server) compatQaTransfer(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
		LogID    int64  `json:"log_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "缺少问题内容"})
		return
	}
	t := store.QaTicket{
		UserID:   &u.ID,
		DeptID:   u.DeptID,
		Question: truncateRunes(req.Question, 500),
		Answer:   truncateRunes(req.Answer, 1000),
	}
	if req.LogID > 0 {
		id := req.LogID
		t.LogID = &id
	}
	id, err := s.Store.CreateQaTicket(c.Request.Context(), &t)
	if err != nil {
		s.Log.Error("提交转人工失败", "err", err, "user_id", u.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "提交失败，请稍后再试"})
		return
	}
	t.ID = id
	s.notifyQaStaff(c, &t)
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "status": "pending"})
}

// compatMyQaTickets 小程序查看自己的转人工记录与回复。
func (s *Server) compatMyQaTickets(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	list, _, err := s.Store.ListQaTickets(c.Request.Context(), "", "", &u.ID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, t := range list {
		items = append(items, gin.H{
			"id": t.ID, "question": t.Question, "answer": t.Answer, "status": t.Status,
			"reply": t.Reply, "assignee": t.Assignee,
			"createdAt": t.CreatedAt.Format("01-02 15:04"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}
