package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
	"coalrag/internal/store"
)

// canManagePlan 管理员/安全科、本队队长、带培师傅可以确认阶段。
func canManagePlan(cl *auth.Claims, plan *store.TrainingPlan) bool {
	if cl == nil || plan == nil {
		return false
	}
	isMentor := plan.MentorUserID != nil && *plan.MentorUserID == cl.UserID
	switch cl.Role {
	case "admin", "safety", "trainer":
		return true
	case "dept_leader":
		if isMentor {
			return true
		}
		return cl.DeptID > 0 && plan.UserDeptID != nil && *plan.UserDeptID == cl.DeptID
	}
	return isMentor
}

func (s *Server) listTrainingPlans(c *gin.Context) {
	cl := claims(c)
	p, size, offset := pageParams(c)
	if cl.Role == "admin" || cl.Role == "safety" || cl.Role == "trainer" {
		list, total, err := s.Store.ListTrainingPlans(c.Request.Context(), size, offset)
		if err != nil {
			fail(c, http.StatusInternalServerError, "查询失败")
			return
		}
		paged(c, list, total, p, size)
		return
	}
	var dept *int64
	if cl.Role == "dept_leader" && cl.DeptID > 0 {
		d := cl.DeptID
		dept = &d
	}
	mentorID := cl.UserID
	list, total, err := s.Store.ListTrainingPlansScoped(c.Request.Context(), dept, &mentorID, size, offset)
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
	if req.Done {
		body := plan.Title + "：阶段已完成确认"
		_ = s.Store.CreateMessage(c.Request.Context(), plan.UserID, "training", "带培进度更新", body, "training", &plan.ID)
	}
	ok(c, gin.H{"id": id, "done": req.Done})
}

// createTrainingPlans 新建带培计划（管理员/安全科，区队长限本队员工）。
func (s *Server) createTrainingPlan(c *gin.Context) {
	cl := claims(c)
	var req struct {
		UserID       int64  `json:"user_id"`
		MentorUserID int64  `json:"mentor_user_id"`
		Title        string `json:"title"`
		TotalWeeks   int    `json:"total_weeks"`
		Target       string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 || req.MentorUserID <= 0 {
		fail(c, http.StatusBadRequest, "请选择员工和师傅")
		return
	}
	u, err := s.Store.GetUserByID(c.Request.Context(), req.UserID)
	if err != nil || u.Status != "active" {
		fail(c, http.StatusBadRequest, "员工不存在或已停用")
		return
	}
	if cl.Role == "dept_leader" {
		if u.DeptID == nil || cl.DeptID <= 0 || *u.DeptID != cl.DeptID {
			fail(c, http.StatusForbidden, "只能给本队员工建计划")
			return
		}
	} else if cl.Role != "admin" && cl.Role != "safety" && cl.Role != "trainer" {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	m, err := s.Store.GetUserByID(c.Request.Context(), req.MentorUserID)
	if err != nil || m.Status != "active" {
		fail(c, http.StatusBadRequest, "师傅不存在或已停用")
		return
	}
	title := req.Title
	if title == "" {
		title = "新员工入矿带培计划"
	}
	target := req.Target
	if target == "" {
		target = "完成入矿教育、岗位跟班与独立上岗考核"
	}
	if req.TotalWeeks <= 0 {
		req.TotalWeeks = 4
	}
	id, err := s.Store.CreateTrainingPlan(c.Request.Context(), req.UserID, req.MentorUserID, title, req.TotalWeeks, target)
	if err != nil {
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	plan, _ := s.Store.GetTrainingPlanByID(c.Request.Context(), id)
	ok(c, plan)
}

// createTrainingPlansBatch 批量给新员工生成计划（同一师傅/周期）。
func (s *Server) createTrainingPlansBatch(c *gin.Context) {
	cl := claims(c)
	var req struct {
		UserIDs      []int64 `json:"user_ids"`
		MentorUserID int64   `json:"mentor_user_id"`
		Title        string  `json:"title"`
		TotalWeeks   int     `json:"total_weeks"`
		Target       string  `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.UserIDs) == 0 || req.MentorUserID <= 0 {
		fail(c, http.StatusBadRequest, "请选择员工和师傅")
		return
	}
	m, err := s.Store.GetUserByID(c.Request.Context(), req.MentorUserID)
	if err != nil || m.Status != "active" {
		fail(c, http.StatusBadRequest, "师傅不存在或已停用")
		return
	}
	title := req.Title
	if title == "" {
		title = "新员工入矿带培计划"
	}
	target := req.Target
	if target == "" {
		target = "完成入矿教育、岗位跟班与独立上岗考核"
	}
	if req.TotalWeeks <= 0 {
		req.TotalWeeks = 4
	}
	created := []int64{}
	skipped := []int64{}
	for _, uid := range req.UserIDs {
		u, err := s.Store.GetUserByID(c.Request.Context(), uid)
		if err != nil || u.Status != "active" {
			skipped = append(skipped, uid)
			continue
		}
		if cl.Role == "dept_leader" {
			if u.DeptID == nil || cl.DeptID <= 0 || *u.DeptID != cl.DeptID {
				skipped = append(skipped, uid)
				continue
			}
		} else if cl.Role != "admin" && cl.Role != "safety" && cl.Role != "trainer" {
			fail(c, http.StatusForbidden, "没有权限")
			return
		}
		id, err := s.Store.CreateTrainingPlan(c.Request.Context(), uid, req.MentorUserID, title, req.TotalWeeks, target)
		if err != nil {
			skipped = append(skipped, uid)
			continue
		}
		created = append(created, id)
	}
	ok(c, gin.H{"created": created, "skipped": skipped, "count": len(created)})
}

// deleteTrainingPlan 删除带培计划（仅管理员；阶段与小结级联删除）。
func (s *Server) deleteTrainingPlan(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if claims(c).Role != model.RoleAdmin {
		fail(c, http.StatusForbidden, "只有管理员可以删除带培计划")
		return
	}
	if err := s.Store.DeleteTrainingPlan(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "计划不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"deleted": true})
}
