package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/store"
)

// canManagePlan 管理员/安全科、本队队长、带培师傅可以确认阶段。
func canManagePlan(cl *auth.Claims, plan *store.TrainingPlan) bool {
	if cl == nil || plan == nil {
		return false
	}
	switch cl.Role {
	case "admin", "safety":
		return true
	case "dept_leader":
		return cl.DeptID > 0 && plan.UserDeptID != nil && *plan.UserDeptID == cl.DeptID
	}
	return cl.Name != "" && plan.Mentor != "" && plan.Mentor == cl.Name
}

func (s *Server) listTrainingPlans(c *gin.Context) {
	cl := claims(c)
	p, size, offset := pageParams(c)
	var dept *int64
	mentor := ""
	switch cl.Role {
	case "admin", "safety":
		list, total, err := s.Store.ListTrainingPlans(c.Request.Context(), size, offset)
		if err != nil {
			fail(c, http.StatusInternalServerError, "查询失败")
			return
		}
		paged(c, list, total, p, size)
		return
	case "dept_leader":
		if cl.DeptID <= 0 && cl.Name == "" {
			fail(c, http.StatusForbidden, "没有可管理的带培计划")
			return
		}
		if cl.DeptID > 0 {
			d := cl.DeptID
			dept = &d
		}
		mentor = cl.Name
	default:
		// 师傅：按名字匹配计划里的 mentor
		mentor = cl.Name
		if mentor == "" {
			fail(c, http.StatusForbidden, "没有可管理的带培计划")
			return
		}
	}
	list, total, err := s.Store.ListTrainingPlansScoped(c.Request.Context(), dept, mentor, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) getTrainingPlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	plan, err := s.Store.GetTrainingPlanByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "计划不存在")
		return
	}
	if !canManagePlan(claims(c), plan) {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	ok(c, plan)
}

func (s *Server) setTrainingStage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	planID, err := s.Store.GetPlanIDByStage(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "阶段不存在")
		return
	}
	plan, err := s.Store.GetTrainingPlanByID(c.Request.Context(), planID)
	if err != nil {
		fail(c, http.StatusNotFound, "计划不存在")
		return
	}
	if !canManagePlan(claims(c), plan) {
		fail(c, http.StatusForbidden, "没有权限确认这个阶段")
		return
	}
	var req struct {
		Done bool `json:"done"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	if err := s.Store.SetStageDoneByID(c.Request.Context(), id, req.Done); err != nil {
		fail(c, http.StatusNotFound, "阶段不存在")
		return
	}
	ok(c, gin.H{"id": id, "done": req.Done})
}
