package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) dashboard(c *gin.Context) {
	cl := claims(c)
	var dept *int64
	if scopeOf(c) != "all" && cl.DeptID > 0 {
		d := cl.DeptID
		dept = &d
	}
	data, err := s.Store.DashboardData(c.Request.Context(), dept)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	ok(c, data)
}

func (s *Server) listAudit(c *gin.Context) {
	p, size, offset := pageParams(c)
	list, total, err := s.Store.ListAudit(c.Request.Context(), c.Query("resource"), c.Query("action"), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询审计日志失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) listTasks(c *gin.Context) {
	p, size, offset := pageParams(c)
	list, total, err := s.Store.ListTasks(c.Request.Context(), c.Query("status"), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询任务失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) retryTask(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.RetryTask(c.Request.Context(), id); err != nil {
		fail(c, http.StatusConflict, "任务不存在或仍在执行")
		return
	}
	ok(c, gin.H{"retried": true})
}
