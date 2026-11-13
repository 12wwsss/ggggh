package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"coalrag/internal/logx"
	"coalrag/internal/model"
)

const (
	ctxClaims = "claims"
	ctxScope  = "scope"
	ctxTrace  = "trace_id"
)

// traceMiddleware 注入 trace id 并写访问日志。
func (s *Server) traceMiddleware(c *gin.Context) {
	tid := c.GetHeader("X-Request-Id")
	if tid == "" {
		tid = uuid.NewString()
	}
	c.Set(ctxTrace, tid)
	c.Header("X-Request-Id", tid)
	ctx := logx.WithTraceID(c.Request.Context(), tid)
	c.Request = c.Request.WithContext(ctx)

	start := time.Now()
	c.Next()
	s.Log.Info("http",
		"trace_id", tid,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", c.Writer.Status(),
		"ms", time.Since(start).Milliseconds(),
		"ip", c.ClientIP(),
	)
}

// recoverMiddleware 捕获 panic。
func (s *Server) recoverMiddleware(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.Log.Error("panic", "err", r, "path", c.Request.URL.Path, "trace_id", c.GetString(ctxTrace))
			fail(c, http.StatusInternalServerError, "服务内部错误")
		}
	}()
	c.Next()
}

// corsMiddleware 允许开发期跨域；生产由 nginx 同源托管。
func (s *Server) corsMiddleware(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}

type bucket struct {
	window time.Time
	count  int
}

type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
}

func newLimiter(perMinute int) *limiter {
	if perMinute <= 0 {
		perMinute = 600
	}
	return &limiter{buckets: map[string]*bucket{}, limit: perMinute}
}

func (l *limiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.window) > time.Minute {
		l.buckets[key] = &bucket{window: now, count: 1}
		return true
	}
	b.count++
	return b.count <= l.limit
}

func (s *Server) rateLimitMiddleware(c *gin.Context) {
	if !s.limiter.allow(c.ClientIP()) {
		fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		return
	}
	c.Next()
}

// authMiddleware 校验 JWT。
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			fail(c, http.StatusUnauthorized, "缺少访问令牌")
			return
		}
		cl, err := s.Auth.Parse(strings.TrimSpace(raw[7:]))
		if err != nil {
			fail(c, http.StatusUnauthorized, "访问令牌无效或已过期")
			return
		}
		c.Set(ctxClaims, cl)
		c.Next()
	}
}

// require 做 RBAC 判定，并把数据范围写入上下文。
func (s *Server) require(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := claims(c)
		if cl == nil {
			fail(c, http.StatusUnauthorized, "未登录")
			return
		}
		sc, ok := s.policy().Scope(cl.Role, resource, action)
		if !ok {
			fail(c, http.StatusForbidden, "无权访问该资源")
			return
		}
		c.Set(ctxScope, sc)
		c.Next()
	}
}

// auditMiddleware 记录变更类请求。
func (s *Server) auditMiddleware(c *gin.Context) {
	c.Next()
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return
	}
	cl := claims(c)
	var uid *int64
	username := ""
	if cl != nil {
		id := cl.UserID
		uid = &id
		username = cl.Username
	}
	segs := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
	resource := ""
	if len(segs) >= 3 {
		resource = segs[2]
	}
	detail := map[string]any{
		"method": c.Request.Method,
		"path":   c.Request.URL.Path,
		"status": c.Writer.Status(),
		"query":  c.Request.URL.RawQuery,
	}
	l := &model.AuditLog{
		UserID:     uid,
		Username:   username,
		Action:     strings.ToLower(c.Request.Method),
		Resource:   resource,
		ResourceID: c.Param("id"),
		Detail:     detail,
		IP:         c.ClientIP(),
		UA:         c.Request.UserAgent(),
	}
	if err := s.Store.InsertAudit(c.Request.Context(), l); err != nil {
		s.Log.Error("写审计日志失败", "err", err)
	}
}
