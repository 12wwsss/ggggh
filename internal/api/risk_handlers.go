package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

// compatRisks 小程序岗位风险点。
func (s *Server) compatRisks(c *gin.Context) {
	job := strings.TrimSpace(c.Query("job"))
	list, err := s.Store.ListRiskPoints(c.Request.Context(), job, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "风险点加载失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, r := range list {
		items = append(items, gin.H{
			"id": r.ID, "job": r.Post, "cat": r.Category, "title": r.Title,
			"desc": r.Description, "level": r.Level,
			"measures": r.Measures, "clauses": r.Clauses, "case": r.CaseText,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

func (s *Server) riskAllowed(c *gin.Context) bool {
	role := claims(c).Role
	if role != "admin" && role != "safety" {
		fail(c, http.StatusForbidden, "没有风险点维护权限")
		return false
	}
	return true
}

func (s *Server) listRisks(c *gin.Context) {
	if !s.riskAllowed(c) {
		return
	}
	job := strings.TrimSpace(c.Query("job"))
	list, err := s.Store.ListRiskPoints(c.Request.Context(), job, 300)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

func (s *Server) createRisk(c *gin.Context) {
	if !s.riskAllowed(c) {
		return
	}
	var req struct {
		Post        string   `json:"post"`
		Category    string   `json:"category"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Measures    []string `json:"measures"`
		Level       string   `json:"level"`
		Clauses     []string `json:"clause_refs"`
		CaseText    string   `json:"case_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		fail(c, http.StatusBadRequest, "请填写风险点名称")
		return
	}
	if req.Post == "" {
		req.Post = "通用"
	}
	if req.Level == "" {
		req.Level = "中"
	}
	item := &store.RiskPoint{Post: req.Post, Category: req.Category, Title: req.Title,
		Description: req.Description, Measures: req.Measures, Level: req.Level, Clauses: req.Clauses,
		CaseText: strings.TrimSpace(req.CaseText)}
	id, err := s.Store.CreateRiskPoint(c.Request.Context(), item)
	if err != nil {
		s.Log.Error("保存风险点失败", "err", err, "post", item.Post, "title", item.Title)
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) deleteRisk(c *gin.Context) {
	if !s.riskAllowed(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	if err := s.Store.DeleteRiskPoint(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, "风险点不存在")
		return
	}
	ok(c, gin.H{"id": id})
}
