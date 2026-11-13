package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// metricsMiddleware 按方法/状态码统计请求量。
func (s *Server) metricsMiddleware(c *gin.Context) {
	c.Next()
	status := c.Writer.Status()
	class := strconv.Itoa(status/100) + "xx"
	s.Metrics.Inc("http_requests_total", map[string]string{
		"method": c.Request.Method,
		"status": class,
		"path":   metricPath(c),
	})
}

// metricPath 把 /docs/12 这类路径归并成 /docs/:id，避免标签爆炸。
func metricPath(c *gin.Context) string {
	route := c.FullPath()
	if route == "" {
		return "unmatched"
	}
	return route
}

// metricsHandler 输出 Prometheus 文本格式，无需鉴权（只含计数，无业务数据）。
func (s *Server) metricsHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if st, err := s.Store.RuntimeStats(ctx); err == nil {
		s.Metrics.Set("docs_published", nil, float64(st.Docs))
		s.Metrics.Set("chunks_published", nil, float64(st.Chunks))
		s.Metrics.Set("users_active", nil, float64(st.Users))
		s.Metrics.Set("tasks_pending", nil, float64(st.TasksPending))
		s.Metrics.Set("tasks_failed", nil, float64(st.TasksFailed))
		s.Metrics.Set("tasks_failed_recent", nil, float64(st.TasksFailedRecent))
		s.Metrics.Set("issues_open", nil, float64(st.IssuesOpen))
		s.Metrics.Set("qa_answered_total", nil, float64(st.QAAnswered))
		s.Metrics.Set("qa_fallback_total", nil, float64(st.QAFallback))
		s.Metrics.Set("questions_today", nil, float64(st.QuestionsToday))
	}
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, s.Metrics.Render())
}
