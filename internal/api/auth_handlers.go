package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
	"coalrag/internal/store"
)

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "账号与口令必填")
		return
	}
	ctx := c.Request.Context()

	u, err := s.Store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusUnauthorized, "账号或口令错误")
			return
		}
		s.Log.Error("查询用户失败", "err", err)
		fail(c, http.StatusInternalServerError, "服务内部错误")
		return
	}
	if u.Status != "active" {
		fail(c, http.StatusForbidden, "账号已停用，请联系管理员")
		return
	}
	now := time.Now()
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		fail(c, http.StatusLocked, "账号已锁定，请 "+u.LockedUntil.Format("15:04")+" 后再试")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		attempts, err := s.Store.LoginFailed(ctx, u.ID)
		if err != nil {
			s.Log.Error("记录登录失败出错", "err", err)
		}
		maxFailures := s.Cfg.Security.MaxLoginFailures
		// 只有累计到阈值才锁定；一次输错不应该把账号锁住。
		if maxFailures > 0 && attempts >= maxFailures && s.Cfg.Security.LockMinutes > 0 {
			until := now.Add(time.Duration(s.Cfg.Security.LockMinutes) * time.Minute)
			if err := s.Store.LockUser(ctx, u.ID, until); err != nil {
				s.Log.Error("锁定账号失败", "err", err)
			}
			_ = s.Store.InsertAudit(ctx, &model.AuditLog{
				UserID: &u.ID, Username: u.Username, Action: "login_locked", Resource: "auth",
				IP: c.ClientIP(), UA: c.Request.UserAgent(),
			})
			fail(c, http.StatusLocked, "口令连续错误次数过多，账号已锁定，可让管理员在后台解锁")
			return
		}
		remain := 0
		if maxFailures > 0 {
			remain = maxFailures - attempts
		}
		msg := "账号或口令错误"
		if maxFailures > 0 {
			msg = fmt.Sprintf("账号或口令错误，还可尝试 %d 次", remain)
		}
		fail(c, http.StatusUnauthorized, msg)
		return
	}

	deptID := int64(0)
	if u.DeptID != nil {
		deptID = *u.DeptID
	}
	token, exp, err := s.Auth.Issue(u.ID, u.Username, u.Name, u.Role, deptID)
	if err != nil {
		s.Log.Error("签发令牌失败", "err", err)
		fail(c, http.StatusInternalServerError, "签发令牌失败")
		return
	}
	if err := s.Store.LoginSucceeded(ctx, u.ID); err != nil {
		s.Log.Error("更新登录状态失败", "err", err)
	}

	ok(c, gin.H{
		"token":      token,
		"expires_at": exp,
		"user":       u,
	})
}

func (s *Server) me(c *gin.Context) {
	cl := claims(c)
	if cl == nil {
		fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	u, err := s.Store.GetUserByID(c.Request.Context(), cl.UserID)
	if err != nil {
		fail(c, http.StatusUnauthorized, "账号不存在")
		return
	}
	ok(c, gin.H{
		"user":        u,
		"permissions": s.policy()[cl.Role],
	})
}

type passwordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func (s *Server) changePassword(c *gin.Context) {
	var req passwordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "原口令与新口令必填")
		return
	}
	if len([]rune(req.NewPassword)) < 8 {
		fail(c, http.StatusBadRequest, "新口令至少 8 位")
		return
	}
	cl := claims(c)
	u, err := s.Store.GetUserByUsername(c.Request.Context(), cl.Username)
	if err != nil {
		fail(c, http.StatusUnauthorized, "账号不存在")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		fail(c, http.StatusBadRequest, "原口令不正确")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "口令处理失败")
		return
	}
	if err := s.Store.UpdatePassword(c.Request.Context(), u.ID, hash); err != nil {
		fail(c, http.StatusInternalServerError, "更新口令失败")
		return
	}
	ok(c, gin.H{"updated": true})
}
