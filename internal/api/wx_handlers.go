package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"coalrag/internal/auth"
	"coalrag/internal/model"
	"coalrag/internal/store"
)

const bindTicketPurpose = "wx_bind"

type wxLoginReq struct {
	Code string `json:"code" binding:"required"`
}

type wxBindReq struct {
	BindTicket string `json:"bind_ticket" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type ticketReq struct {
	Code string `json:"code" binding:"required"`
}

// wxLogin 用小程序 code 换 openid：已绑定直接发会话，未绑定发一张绑定票据。
func (s *Server) wxLogin(c *gin.Context) {
	var req wxLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "code 必填")
		return
	}
	ctx := c.Request.Context()
	if s.Wx == nil {
		fail(c, http.StatusServiceUnavailable, "微信登录未启用")
		return
	}
	openid, unionid, err := s.Wx.Code2Session(ctx, req.Code)
	if err != nil {
		s.Log.Error("微信 code 换取 openid 失败", "err", err)
		fail(c, http.StatusBadRequest, "微信登录失败："+err.Error())
		return
	}

	userID, err := s.Store.GetWxBind(ctx, openid)
	if err == nil {
		u, err := s.Store.GetUserByID(ctx, userID)
		if err != nil || u.Status != "active" {
			fail(c, http.StatusForbidden, "账号已停用，请联系管理员")
			return
		}
		_ = s.Store.TouchWxLogin(ctx, openid)
		s.issueWxSession(c, u)
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("查询微信绑定失败", "err", err)
		fail(c, http.StatusInternalServerError, "服务内部错误")
		return
	}

	ticket, err := s.signBindTicket(openid, unionid)
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成绑定票据失败")
		return
	}
	ok(c, gin.H{"need_bind": true, "bind_ticket": ticket})
}

// wxBind 首次绑定：校验工号与口令，写入 openid 后发会话。
func (s *Server) wxBind(c *gin.Context) {
	var req wxBindReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "绑定票据、工号与口令必填")
		return
	}
	openid, unionid, err := s.parseBindTicket(req.BindTicket)
	if err != nil {
		fail(c, http.StatusUnauthorized, "绑定票据已过期，请重新打开小程序")
		return
	}

	ctx := c.Request.Context()
	u, err := s.Store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		fail(c, http.StatusUnauthorized, "工号或口令错误")
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
		attempts, _ := s.Store.LoginFailed(ctx, u.ID)
		maxFailures := s.Cfg.Security.MaxLoginFailures
		if maxFailures > 0 && attempts >= maxFailures && s.Cfg.Security.LockMinutes > 0 {
			until := now.Add(time.Duration(s.Cfg.Security.LockMinutes) * time.Minute)
			_ = s.Store.LockUser(ctx, u.ID, until)
			fail(c, http.StatusLocked, "口令连续错误次数过多，账号已锁定")
			return
		}
		fail(c, http.StatusUnauthorized, "工号或口令错误")
		return
	}

	if err := s.Store.BindWx(ctx, openid, unionid, u.ID); err != nil {
		s.Log.Error("写入微信绑定失败", "err", err)
		fail(c, http.StatusInternalServerError, "绑定失败")
		return
	}
	if err := s.Store.LoginSucceeded(ctx, u.ID); err != nil {
		s.Log.Error("更新登录状态失败", "err", err)
	}
	_ = s.Store.InsertAudit(ctx, &model.AuditLog{
		UserID: &u.ID, Username: u.Username, Action: "wx_bind", Resource: "auth",
		IP: c.ClientIP(), UA: c.Request.UserAgent(),
	})
	s.issueWxSession(c, u)
}

// ticketLogin 用一次性票据换会话，供 web-view 打开 H5 时使用。
func (s *Server) ticketLogin(c *gin.Context) {
	var req ticketReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "票据必填")
		return
	}
	ctx := c.Request.Context()
	userID, err := s.Store.ConsumeLoginTicket(ctx, req.Code)
	if err != nil {
		fail(c, http.StatusUnauthorized, "票据无效或已过期，请重新进入")
		return
	}
	u, err := s.Store.GetUserByID(ctx, userID)
	if err != nil || u.Status != "active" {
		fail(c, http.StatusForbidden, "账号已停用")
		return
	}
	cl := s.issueSession(c, u)
	if cl == nil {
		return
	}
	s.Log.Info("票据换会话成功", "user", cl.username)
	ok(c, gin.H{"token": cl.token, "expires_at": cl.exp, "user": u})
}

func (s *Server) issueWxSession(c *gin.Context, u *model.User) {
	cl := s.issueSession(c, u)
	if cl == nil {
		return
	}
	code, err := s.Store.CreateLoginTicket(c.Request.Context(), u.ID, 2*time.Minute)
	if err != nil {
		// 票据失败不影响小程序直接调接口，只是不能免登录打开 web-view
		s.Log.Error("生成登录票据失败", "err", err)
		code = ""
	}
	ok(c, gin.H{"token": cl.token, "expires_at": cl.exp, "user": u, "ticket": code})
}

type session struct {
	token    string
	exp      time.Time
	username string
}

// issueSession 统一签发 JWT 并写响应；返回会话供调用方追加字段。
func (s *Server) issueSession(c *gin.Context, u *model.User) *session {
	deptID := int64(0)
	if u.DeptID != nil {
		deptID = *u.DeptID
	}
	token, exp, err := s.Auth.Issue(u.ID, u.Username, u.Name, u.Role, deptID)
	if err != nil {
		s.Log.Error("签发令牌失败", "err", err)
		fail(c, http.StatusInternalServerError, "签发令牌失败")
		return nil
	}
	_ = s.Store.LoginSucceeded(c.Request.Context(), u.ID)
	return &session{token: token, exp: exp, username: u.Username}
}

func (s *Server) signBindTicket(openid, unionid string) (string, error) {
	claims := jwt.MapClaims{
		"purpose": bindTicketPurpose,
		"openid":  openid,
		"unionid": unionid,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *Server) parseBindTicket(tokenStr string) (openid, unionid string, err error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不符")
		}
		return []byte(s.Cfg.JWTSecret), nil
	})
	if err != nil || !tok.Valid {
		return "", "", errors.New("票据无效")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok || claims["purpose"] != bindTicketPurpose {
		return "", "", errors.New("票据用途不符")
	}
	openid, _ = claims["openid"].(string)
	unionid, _ = claims["unionid"].(string)
	if openid == "" {
		return "", "", errors.New("票据缺少 openid")
	}
	return openid, unionid, nil
}
