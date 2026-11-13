package api

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// cidrMiddleware 按来源网段放行；未配置时不做限制。
// 只挂在 /api/v1 上，/healthz、/metrics 保持可探测。
func (s *Server) cidrMiddleware(c *gin.Context) {
	if len(s.allowedNets) == 0 {
		c.Next()
		return
	}
	ip := net.ParseIP(c.ClientIP())
	if ip == nil {
		fail(c, http.StatusForbidden, "来源网络不在允许范围内")
		c.Abort()
		return
	}
	for _, block := range s.allowedNets {
		if block.Contains(ip) {
			c.Next()
			return
		}
	}
	s.Log.Warn("拦截白名单外的请求", "ip", c.ClientIP(), "path", c.Request.URL.Path)
	fail(c, http.StatusForbidden, "来源网络不在允许范围内，请连接公司网络后重试")
	c.Abort()
}
