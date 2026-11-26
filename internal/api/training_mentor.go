package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// changeTrainingMentor 更换带培师傅（管理员/安全科/本队队长）。
func (s *Server) changeTrainingMentor(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	plan, err := s.Store.GetTrainingPlanByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "计划不存在")
		return
	}
	cl := claims(c)
	allowed := cl.Role == "admin" || cl.Role == "safety" || cl.Role == "trainer" ||
		(cl.Role == "dept_leader" && cl.DeptID > 0 && plan.UserDeptID != nil && *plan.UserDeptID == cl.DeptID)
	if !allowed {
		fail(c, http.StatusForbidden, "没有更换师傅的权限")
		return
	}
	var req struct {
		MentorUserID int64 `json:"mentor_user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MentorUserID <= 0 {
		fail(c, http.StatusBadRequest, "请选择新师傅")
		return
	}
	if err := s.Store.UpdatePlanMentor(c.Request.Context(), id, req.MentorUserID); err != nil {
		fail(c, http.StatusBadRequest, "师傅不存在或已停用")
		return
	}
	updated, _ := s.Store.GetTrainingPlanByID(c.Request.Context(), id)
	ok(c, updated)
}
