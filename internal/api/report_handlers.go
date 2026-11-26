package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) reportsAllowed(c *gin.Context) bool {
	role := claims(c).Role
	if role != "admin" && role != "safety" {
		fail(c, http.StatusForbidden, "没有报表权限")
		return false
	}
	return true
}

func (s *Server) learningReport(c *gin.Context) {
	if !s.reportsAllowed(c) {
		return
	}
	var dept *int64
	if v := c.Query("dept_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			dept = &id
		}
	}
	rep, err := s.Store.LearningReport(c.Request.Context(), dept)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	ok(c, rep)
}

func (s *Server) wrongReport(c *gin.Context) {
	if !s.reportsAllowed(c) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	rep, err := s.Store.WrongReport(c.Request.Context(), limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	ok(c, rep)
}

func (s *Server) issueReport(c *gin.Context) {
	if !s.reportsAllowed(c) {
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "14"))
	rep, err := s.Store.IssueReport(c.Request.Context(), days)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	if days >= 1 && days <= 90 {
		if p1, p2, err := s.Store.IssueCountsRange(c.Request.Context(), days*2-1, days); err == nil {
			rep.PrevReported = p1
			rep.PrevClosed = p2
		}
	}
	ok(c, rep)
}
