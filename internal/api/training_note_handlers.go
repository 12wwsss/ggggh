package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

// listTrainingNotes 查看某份带培计划的小结：可管计划的人 + 计划本人。
func (s *Server) listTrainingNotes(c *gin.Context) {
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
	if !canManagePlan(cl, plan) && !(cl != nil && cl.UserID == plan.UserID) {
		fail(c, http.StatusForbidden, "没有权限查看这份计划的小结")
		return
	}
	notes, err := s.Store.ListTrainingNotes(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询小结失败")
		return
	}
	ok(c, gin.H{"list": notes, "total": len(notes)})
}

// createTrainingNote 师傅写月度小结/期满总结；培训员、管理员、本队队长也可以补写。
func (s *Server) createTrainingNote(c *gin.Context) {
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
	if !canManagePlan(cl, plan) {
		fail(c, http.StatusForbidden, "只有带培师傅、培训员或管理员可以写小结")
		return
	}
	var req struct {
		Kind    string `json:"kind"`
		Period  string `json:"period"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	content := strings.TrimSpace(req.Content)
	if len([]rune(content)) < 10 {
		fail(c, http.StatusBadRequest, "小结至少写 10 个字")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "monthly"
	}
	if kind != "monthly" && kind != "final" {
		fail(c, http.StatusBadRequest, "kind 只能是 monthly 或 final")
		return
	}
	period := strings.TrimSpace(req.Period)
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	if len(period) != 7 || period[4] != '-' {
		fail(c, http.StatusBadRequest, "period 格式应为 2026-11")
		return
	}
	if _, err := time.Parse("2006-01", period); err != nil {
		fail(c, http.StatusBadRequest, "period 格式应为 2026-11")
		return
	}
	author := cl.UserID
	noteID, err := s.Store.AddTrainingNote(c.Request.Context(), &store.TrainingNote{
		PlanID: plan.ID, AuthorID: &author, Kind: kind, Period: period, Content: content,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存小结失败")
		return
	}
	ok(c, gin.H{"id": noteID, "kind": kind, "period": period})
}

// archiveUserID 校验并解析导出档案的目标用户。
func (s *Server) archiveUserID(c *gin.Context) (int64, bool) {
	id, _ := strconv.ParseInt(c.DefaultQuery("user_id", "0"), 10, 64)
	if id <= 0 {
		fail(c, http.StatusBadRequest, "user_id 不能为空")
		return 0, false
	}
	return id, true
}
