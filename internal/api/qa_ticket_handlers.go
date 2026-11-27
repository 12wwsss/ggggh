package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/store"
)

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func qaStaffAllowed(role string) bool {
	return role == model.RoleAdmin || role == model.RoleSafety
}

// notifyQaStaff 把转人工通知发给安全科（没有安全科账号时发给管理员）。
func (s *Server) notifyQaStaff(c *gin.Context, t *store.QaTicket) {
	ids, err := s.Store.UsersWithRoles(c.Request.Context(), model.RoleSafety)
	if err != nil || len(ids) == 0 {
		ids, _ = s.Store.UsersWithRoles(c.Request.Context(), model.RoleAdmin)
	}
	title := "转人工：" + truncateRunes(t.Question, 22)
	body := t.Question
	if t.Answer != "" {
		body += "\n\nAI 原回答：" + truncateRunes(t.Answer, 120)
	}
	body += "\n\n请到后台「问答转人工」回复，员工会在消息中心看到。"
	for _, uid := range ids {
		_ = s.Store.CreateMessage(c.Request.Context(), uid, "qa", title, body, "qa_ticket", &t.ID)
	}
}

// transferToHuman 员工在问答里点「转人工」。
func (s *Server) transferToHuman(c *gin.Context) {
	cl := claims(c)
	var t store.QaTicket
	t.UserID = &cl.UserID
	dept := cl.DeptID
	t.DeptID = &dept
	var req struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
		LogID    int64  `json:"log_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		fail(c, http.StatusBadRequest, "缺少问题内容")
		return
	}
	t.Question = truncateRunes(req.Question, 500)
	t.Answer = truncateRunes(req.Answer, 1000)
	if req.LogID > 0 {
		id := req.LogID
		t.LogID = &id
	}
	id, err := s.Store.CreateQaTicket(c.Request.Context(), &t)
	if err != nil {
		fail(c, http.StatusInternalServerError, "提交失败，请稍后再试")
		return
	}
	t.ID = id
	s.notifyQaStaff(c, &t)
	ok(c, gin.H{"id": id, "status": "pending"})
}

// listQaTickets 后台工单列表（管理员/安全科）。
func (s *Server) listQaTickets(c *gin.Context) {
	cl := claims(c)
	if !qaStaffAllowed(cl.Role) {
		fail(c, http.StatusForbidden, "只有管理员和安全科可以查看转人工工单")
		return
	}
	p, size, offset := pageParams(c)
	list, total, err := s.Store.ListQaTickets(c.Request.Context(),
		strings.TrimSpace(c.Query("status")), strings.TrimSpace(c.Query("keyword")), nil, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	pending, _ := s.Store.CountPendingQaTickets(c.Request.Context())
	paged(c, list, total, p, size)
	_ = pending
}

// replyQaTicket 安全科回复。
func (s *Server) replyQaTicket(c *gin.Context) {
	cl := claims(c)
	if !qaStaffAllowed(cl.Role) {
		fail(c, http.StatusForbidden, "没有回复权限")
		return
	}
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Reply string `json:"reply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Reply) == "" {
		fail(c, http.StatusBadRequest, "回复内容不能为空")
		return
	}
	t, err := s.Store.GetQaTicket(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "工单不存在")
		return
	}
	reply := truncateRunes(req.Reply, 1000)
	if err := s.Store.ReplyQaTicket(c.Request.Context(), id, cl.UserID, reply); err != nil {
		fail(c, http.StatusInternalServerError, "回复失败")
		return
	}
	if t.UserID != nil {
		body := reply + "\n\n（原问题：" + truncateRunes(t.Question, 60) + "）"
		_ = s.Store.CreateMessage(c.Request.Context(), *t.UserID, "qa", "安全科回复了你的提问", body, "qa_ticket", &id)
	}
	ok(c, gin.H{"id": id, "status": "replied"})
}

// closeQaTicket 关闭工单。
func (s *Server) closeQaTicket(c *gin.Context) {
	if !qaStaffAllowed(claims(c).Role) {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.CloseQaTicket(c.Request.Context(), id); err != nil {
		fail(c, http.StatusInternalServerError, "关闭失败")
		return
	}
	ok(c, gin.H{"id": id, "status": "closed"})
}

// myQaTickets 员工查看自己提交的转人工及回复。
func (s *Server) myQaTickets(c *gin.Context) {
	cl := claims(c)
	list, _, err := s.Store.ListQaTickets(c.Request.Context(), "", "", &cl.UserID, 50, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"list": list})
}
