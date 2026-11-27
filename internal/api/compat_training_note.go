package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/store"
)

// canWriteTrainingNote 判断当前用户能否给这份计划写小结：
// 管理员/安全科/培训员，或这份计划的带培师傅。
func canWriteTrainingNote(u *model.User, plan *store.TrainingPlan) bool {
	if u == nil || plan == nil {
		return false
	}
	switch u.Role {
	case model.RoleAdmin, model.RoleSafety, model.RoleTrainer:
		return true
	}
	return plan.MentorUserID != nil && *plan.MentorUserID == u.ID
}

// compatSaveTrainingNote 小程序端写师傅小结（师傅、培训员、管理员）。
func (s *Server) compatSaveTrainingNote(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		PlanID  int64  `json:"plan_id"`
		Kind    string `json:"kind"`
		Period  string `json:"period"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	plan, err := s.Store.GetTrainingPlanByID(c.Request.Context(), req.PlanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "带培计划不存在"})
		return
	}
	if !canWriteTrainingNote(u, plan) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "只有带培师傅、培训员或管理员可以写小结"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if len([]rune(content)) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "小结至少写 10 个字"})
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "monthly"
	}
	if kind != "monthly" && kind != "final" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "kind 只能是 monthly 或 final"})
		return
	}
	period := strings.TrimSpace(req.Period)
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	if _, err := time.Parse("2006-01", period); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "period 格式应为 2026-11"})
		return
	}
	author := u.ID
	id, err := s.Store.AddTrainingNote(c.Request.Context(), &store.TrainingNote{
		PlanID: plan.ID, AuthorID: &author, Kind: kind, Period: period, Content: content,
	})
	if err != nil {
		s.Log.Error("保存带培小结失败", "err", err, "plan_id", plan.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// compatSetTrainingStage 小程序端确认/撤销阶段：师傅、培训员、管理员可用。
func (s *Server) compatSetTrainingStage(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		StageID int64 `json:"stage_id"`
		Done    bool  `json:"done"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.StageID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	ctx := c.Request.Context()
	planID, err := s.Store.GetPlanIDByStage(ctx, req.StageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "阶段不存在"})
		return
	}
	plan, err := s.Store.GetTrainingPlanByID(ctx, planID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "计划不存在"})
		return
	}
	if !canWriteTrainingNote(u, plan) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "只有带培师傅、培训员或管理员可以确认阶段"})
		return
	}
	if err := s.Store.SetStageDoneByID(ctx, req.StageID, req.Done); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "阶段不存在"})
		return
	}
	if req.Done {
		body := plan.Title + "：阶段已完成确认"
		_ = s.Store.CreateMessage(ctx, plan.UserID, "training", "带培进度更新", body, "training", &plan.ID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "done": req.Done})
}
