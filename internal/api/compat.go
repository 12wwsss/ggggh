package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
)

// 兼容层：小程序（原 Node 代理）使用 /api/*，响应约定 {ok:true,...}。
// 先落地健康检查与问答，其余接口按页面逐步补齐。

// compatLogin 工号登录：返回小程序约定的 {ok, token, user}。
func (s *Server) compatLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "工号与口令必填"})
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "工号或口令错误"})
		return
	}
	if u.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "账号已停用，请联系管理员"})
		return
	}
	now := time.Now()
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		c.JSON(http.StatusLocked, gin.H{"ok": false, "message": "账号已锁定，请 " + u.LockedUntil.Format("15:04") + " 后再试"})
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		attempts, _ := s.Store.LoginFailed(ctx, u.ID)
		maxFailures := s.Cfg.Security.MaxLoginFailures
		if maxFailures > 0 && attempts >= maxFailures && s.Cfg.Security.LockMinutes > 0 {
			until := now.Add(time.Duration(s.Cfg.Security.LockMinutes) * time.Minute)
			_ = s.Store.LockUser(ctx, u.ID, until)
			c.JSON(http.StatusLocked, gin.H{"ok": false, "message": "口令连续错误次数过多，账号已锁定，请联系管理员解锁"})
			return
		}
		msg := "工号或口令错误"
		if maxFailures > 0 {
			msg = fmt.Sprintf("工号或口令错误，还可尝试 %d 次", maxFailures-attempts)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": msg})
		return
	}
	deptID := int64(0)
	if u.DeptID != nil {
		deptID = *u.DeptID
	}
	token, exp, err := s.Auth.Issue(u.ID, u.Username, u.Name, u.Role, deptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "登录失败，请稍后重试"})
		return
	}
	_ = s.Store.LoginSucceeded(ctx, u.ID)
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"token":      token,
		"expires_at": exp,
		"user": gin.H{
			"id":       u.ID,
			"username": u.Username,
			"name":     u.Name,
			"role":     u.Role,
			"dept_id":  deptID,
			"post":     u.Post,
		},
	})
}

// compatMe 当前登录人信息。
func (s *Server) compatMe(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": gin.H{
		"id": u.ID, "username": u.Username, "name": u.Name, "role": u.Role, "post": u.Post,
	}})
}

// compatCurrentUser 从 Bearer 令牌解析当前用户；无令牌或无效返回 nil。
func (s *Server) compatCurrentUser(c *gin.Context) *model.User {
	h := c.GetHeader("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil
	}
	cl, err := s.Auth.Parse(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return nil
	}
	u, err := s.Store.GetUserByID(c.Request.Context(), cl.UserID)
	if err != nil {
		return nil
	}
	return u
}

func (s *Server) compatHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"key":  "offline",
		"kb":   0,
		"llm":  true,
		"mode": "offline",
	})
}

// compatOptions 岗位/区队/资料分类选项。
func (s *Server) compatOptions(c *gin.Context) {
	jobs, err := s.Store.DistinctPosts(c.Request.Context())
	if err != nil {
		jobs = nil
	}
	jobs = append([]string{"通用"}, jobs...)
	teams, err := s.Store.ListDeptNames(c.Request.Context())
	if err != nil {
		teams = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"jobs":      jobs,
		"teams":     teams,
		"mediaCats": []string{"操作视频", "设备原理", "规程制度", "事故案例"},
		"contact":   "",
		"version":   "1.0",
	})
}

// compatCulture 安全文化/企业文化的展示条目。
func (s *Server) compatCulture(c *gin.Context) {
	items, err := s.Store.ListContentItems(c.Request.Context(), "culture", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "内容加载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatContentList 图文内容列表（安全文化页也走这里）。
func (s *Server) compatContentList(c *gin.Context) {
	items, err := s.Store.ListContentItems(c.Request.Context(), "article", c.Query("cat"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "内容加载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatContentDetail 图文详情。
func (s *Server) compatContentDetail(c *gin.Context) {
	item, err := s.Store.GetContentItem(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "内容不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "item": item})
}

// compatNotices 制度宣贯列表。
func (s *Server) compatNotices(c *gin.Context) {
	items, err := s.Store.ListContentItems(c.Request.Context(), "notice", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "宣贯加载失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatMaterials 视频/资料列表；当前没有资料返回空列表，页面不报错。
func (s *Server) compatMaterials(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": []any{}})
}

// compatEmployeeSummary 首页个人统计。
func (s *Server) compatEmployeeSummary(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"hours":     0,
		"readCount": 0,
		"quiz":      gin.H{"attempts": 0},
		"issue":     gin.H{"total": 0},
	})
}

type compatChatReq struct {
	Question string `json:"question"`
	Image    string `json:"image"`
}

func (s *Server) compatChat(c *gin.Context) {
	var req compatChatReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "问题不能为空"})
		return
	}
	res, err := s.Svc.Ask(c.Request.Context(), s.compatCurrentUser(c), req.Question, c.ClientIP())
	if err != nil {
		s.Log.Error("兼容问答失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "AI 服务异常，请稍后重试"})
		return
	}
	// 小程序问答页不渲染 actions 字段，这里把处置动作并入正文，
	// 避免出现只有“依据：[第一百三十六条]”而看不到具体步骤的情况。
	answer := strings.TrimSpace(res.Answer)
	if len(res.Actions) > 0 {
		lines := make([]string, 0, len(res.Actions)+2)
		if answer != "" {
			lines = append(lines, answer)
		}
		for i, a := range res.Actions {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, a))
		}
		answer = strings.Join(lines, "\n")
	}
	cites := make([]gin.H, 0, len(res.Citations))
	for _, ct := range res.Citations {
		title := strings.TrimSpace(ct.ClauseNo)
		if ct.DocTitle != "" {
			if title == "" {
				title = ct.DocTitle
			} else {
				title = title + " · " + ct.DocTitle
			}
		}
		cites = append(cites, gin.H{
			"id":        ct.ChunkID,
			"title":     title,
			"clause_no": ct.ClauseNo,
			"text":      ct.Snippet,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"answer":   answer,
		"cites":    cites,
		"grounded": res.Answered,
		"model":    res.Model,
	})
}
