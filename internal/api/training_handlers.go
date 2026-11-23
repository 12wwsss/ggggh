package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// 带培管理：只允许管理员维护。
func (s *Server) trainingAdmin(c *gin.Context) bool {
	if claims(c).Role != "admin" {
		fail(c, http.StatusForbidden, "只有管理员可以维护带培计划")
		return false
	}
	return true
}

func (s *Server) listTrainingPlans(c *gin.Context) {
	if !s.trainingAdmin(c) {
		return
	}
	p, size, offset := pageParams(c)
	list, total, err := s.Store.ListTrainingPlans(c.Request.Context(), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) getTrainingPlan(c *gin.Context) {
	if !s.trainingAdmin(c) {
		return
	}
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
	ok(c, plan)
}

func (s *Server) setTrainingStage(c *gin.Context) {
	if !s.trainingAdmin(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
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
