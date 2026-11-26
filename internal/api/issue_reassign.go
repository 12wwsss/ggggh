package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
)

// reassignIssue 更换整改负责人（已受理状态，管理员/安全科/本队队长）。
func (s *Server) reassignIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		AssigneeID int64  `json:"assignee_id"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AssigneeID <= 0 {
		fail(c, http.StatusBadRequest, "请选择新的负责人")
		return
	}
	cl := claims(c)
	if !isIssueManager(cl.Role) {
		fail(c, http.StatusForbidden, "没有更换负责人的权限")
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权操作该问题")
		return
	}
	if issue.Status != model.IssueAccepted {
		fail(c, http.StatusConflict, "只有已受理的问题可以更换负责人")
		return
	}
	u, err := s.Store.GetUserByID(c.Request.Context(), req.AssigneeID)
	if err != nil || u.Status != "active" {
		fail(c, http.StatusBadRequest, "负责人不存在或已停用")
		return
	}
	if issue.DeptID != nil && (u.DeptID == nil || *u.DeptID != *issue.DeptID) {
		fail(c, http.StatusBadRequest, "只能指派本队人员")
		return
	}
	comment := strings.TrimSpace(req.Reason)
	if comment == "" {
		comment = "更换整改负责人"
	}
	updated, err := s.Store.ReassignIssue(c.Request.Context(), id, cl.UserID, req.AssigneeID, comment)
	if err != nil {
		fail(c, http.StatusConflict, "更换失败，请刷新后重试")
		return
	}
	ok(c, updated)
}
