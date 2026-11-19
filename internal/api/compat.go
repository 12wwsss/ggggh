package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
	"coalrag/internal/store"
)

// 兼容层：小程序（原 Node 代理）使用 /api/*，响应约定 {ok:true,...}。
// 先落地健康检查与问答，其余接口按页面逐步补齐。

// compatProfile 个人资料（目前只读姓名，头像后续接上传）。
func (s *Server) compatProfile(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "name": "", "avatar": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": u.Name, "role": u.Role, "post": u.Post, "avatar": ""})
}

// compatSaveProfile 保存姓名/头像；当前不改库，返回成功避免页面报错。
func (s *Server) compatSaveProfile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// compatQuizStats 答题统计；题库接完后替换为真实数据。
func (s *Server) compatQuizStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "mine": nil, "byTeam": []any{}, "passRate": 0, "total": 0})
}

// compatQGroups 题库分组（按岗位）。
func (s *Server) compatQGroups(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": []any{}, "items": []any{}})
}

// compatQuestions 题目列表；题库接完后替换。
func (s *Server) compatQuestions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": []any{}})
}

// compatExams 考试列表；考试页接完后替换。
func (s *Server) compatExams(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": []any{}})
}

// compatFAQ 高频问题。
func (s *Server) compatFAQ(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": []any{}})
}

// ---------- 隐患上报（兼容小程序 /api/issue*） ----------

// compatUploadName 从 "/uploads/xxx" 或完整 URL 中取出文件名。
func compatUploadName(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "/uploads/"); i >= 0 {
		raw = raw[i+len("/uploads/"):]
	}
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" || strings.Contains(raw, "..") {
		return ""
	}
	return filepath.Base(raw)
}

func compatIssueStatus(st string) string {
	switch st {
	case model.IssueReported:
		return "待分派"
	case model.IssueClosed:
		return "已闭环"
	case model.IssueRejected:
		return "已驳回"
	default:
		return "处置中"
	}
}

// compatIssueStatusText 五态明细，给小程序展示；status 仍保留三态给标签页筛选。
func compatIssueStatusText(st string) string {
	switch st {
	case model.IssueReported:
		return "已上报"
	case model.IssueAccepted:
		return "已受理"
	case model.IssueRectified:
		return "已整改"
	case model.IssueVerified:
		return "已验收"
	case model.IssueClosed:
		return "已闭环"
	case model.IssueRejected:
		return "已驳回"
	default:
		return st
	}
}

func compatFlowAction(a string) string {
	switch a {
	case "report":
		return "上报"
	case "accept":
		return "受理"
	case "reject":
		return "驳回"
	case "rectify":
		return "提交整改"
	case "verify":
		return "验收通过"
	case "verify_reject":
		return "验收不合格"
	case "close":
		return "闭环"
	case "admin_force_rectify":
		return "管理员代整改"
	case "admin_force_verify":
		return "管理员强制验收"
	case "admin_force_close":
		return "管理员强制闭环"
	default:
		return a
	}
}

func (s *Server) compatUpload(c *gin.Context) {
	var req struct {
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "缺少文件数据"})
		return
	}
	b64 := req.Data
	ext := ".bin"
	if strings.HasPrefix(b64, "data:") {
		parts := strings.SplitN(b64, ",", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "文件格式不正确"})
			return
		}
		meta := parts[0]
		b64 = parts[1]
		switch {
		case strings.Contains(meta, "image/png"):
			ext = ".png"
		case strings.Contains(meta, "image/webp"):
			ext = ".webp"
		case strings.Contains(meta, "image/jpeg"), strings.Contains(meta, "image/jpg"):
			ext = ".jpg"
		case strings.Contains(meta, "audio/mpeg"), strings.Contains(meta, "audio/mp3"):
			ext = ".mp3"
		case strings.Contains(meta, "audio/mp4"), strings.Contains(meta, "audio/m4a"):
			ext = ".m4a"
		}
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "文件解码失败"})
		return
	}
	if len(raw) > 12<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "文件超过 12MB"})
		return
	}
	dir := filepath.Join(s.Cfg.StorageDir, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "保存失败"})
		return
	}
	name := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), os.Getpid(), ext)
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "url": "/uploads/" + name})
}

func (s *Server) serveUpload(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	path := filepath.Join(s.Cfg.StorageDir, "uploads", name)
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(path)
}

func (s *Server) compatCreateIssue(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		Type     string   `json:"type"`
		Title    string   `json:"title"`
		Desc     string   `json:"desc"`
		Urgent   bool     `json:"urgent"`
		Loc      string   `json:"loc"`
		Photos   []string `json:"photos"`
		Audio    string   `json:"audio"`
		AudioSec int      `json:"audioSec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "上报内容不正确"})
		return
	}
	title := strings.TrimSpace(req.Title)
	desc := strings.TrimSpace(req.Desc)
	if title == "" {
		title = desc
	}
	if r := []rune(title); len(r) > 40 {
		title = string(r[:40])
	}
	if title == "" && req.Audio == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "写一句或录一段语音"})
		return
	}
	severity := "normal"
	if req.Urgent {
		severity = "high"
	}
	meta := []string{}
	if req.Type != "" {
		meta = append(meta, "【"+req.Type+"】")
	}
	if req.Loc != "" {
		meta = append(meta, "位置："+req.Loc)
	}
	if len(meta) > 0 {
		desc = strings.Join(meta, " ") + "\n" + desc
	}
	issue := &model.Issue{Title: title, Description: desc, Severity: severity, ReporterID: &u.ID}
	if u.DeptID != nil {
		issue.DeptID = u.DeptID
	}
	created, err := s.Store.CreateIssue(c.Request.Context(), issue)
	if err != nil {
		s.Log.Error("兼容上报失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "上报失败"})
		return
	}
	files := append([]string{}, req.Photos...)
	if req.Audio != "" {
		files = append(files, req.Audio)
	}
	for _, raw := range files {
		name := compatUploadName(raw)
		if name == "" {
			continue
		}
		_, _ = s.Store.AddIssueAttachment(c.Request.Context(), &model.IssueAttachment{
			IssueID: created.ID, FileName: name, UploaderID: &u.ID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "item": gin.H{"id": created.ID, "code": created.Code}})
}

func (s *Server) compatListIssues(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var dept, reporter *int64
	switch {
	case u.Role == model.RoleAdmin || u.Role == model.RoleSafety:
		// 管理员与安全科看全矿
	case u.Role == model.RoleDeptLeader:
		if u.DeptID != nil {
			dept = u.DeptID
		}
	default:
		reporter = &u.ID
	}
	var list []model.Issue
	var err error
	if reporter != nil {
		// 员工：我上报的 + 指派给我的
		list, _, err = s.Store.ListIssuesByUser(c.Request.Context(), u.ID, 100, 0)
	} else {
		list, _, err = s.Store.ListIssues(c.Request.Context(), "", dept, nil, "", 100, 0)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	want := strings.TrimSpace(c.Query("status"))
	items := make([]gin.H, 0, len(list))
	for _, it := range list {
		label := compatIssueStatus(it.Status)
		if want != "" && label != want {
			continue
		}
		photos := []string{}
		if atts, err := s.Store.ListIssueAttachments(c.Request.Context(), it.ID); err == nil {
			for _, a := range atts {
				name := a.FileName
				// 兼容层上传的附件是 uploads 下的裸文件名；历史附件存的是绝对路径，跳过
				if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
					continue
				}
				if _, err := os.Stat(filepath.Join(s.Cfg.StorageDir, "uploads", name)); err != nil {
					continue
				}
				photos = append(photos, "/uploads/"+name)
			}
		}
		note := it.Result
		if note == "" {
			note = it.VerifyComment
		}
		items = append(items, gin.H{
			"id":          it.ID,
			"code":        it.Code,
			"title":       it.Title,
			"desc":        it.Description,
			"type":        it.CateName,
			"level":       it.Severity,
			"status":      label,
			"status_text": compatIssueStatusText(it.Status),
			"raw_status":  it.Status,
			"createdAt":   it.CreatedAt.Format(time.RFC3339),
			"handleNote":  note,
			"photos":      photos,
			"reporter":    it.Reporter,
			"reporter_id": it.ReporterID,
			"assignee_id": it.AssigneeID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items, "total": len(items)})
}

func (s *Server) compatIssueStats(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var dept, reporter *int64
	switch {
	case u.Role == model.RoleAdmin || u.Role == model.RoleSafety:
	case u.Role == model.RoleDeptLeader:
		if u.DeptID != nil {
			dept = u.DeptID
		}
	default:
		reporter = &u.ID
	}
	stats, err := s.Store.IssueStats(c.Request.Context(), dept, reporter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "统计失败"})
		return
	}
	pending := stats[model.IssueReported]
	closed := stats[model.IssueClosed]
	overdue := stats["overdue"]
	total := 0
	for st, n := range stats {
		if st == "overdue" {
			continue
		}
		total += n
	}
	doing := total - pending - closed
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "pending": pending, "doing": doing, "closed": closed,
		"total": total, "overdue": overdue,
	})
}

func (s *Server) compatHandleIssue(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	if u.Role == model.RoleEmployee {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "没有处置权限"})
		return
	}
	var req struct {
		ID         int64  `json:"id"`
		Note       string `json:"note"`
		Reason     string `json:"reason"`
		AssigneeID *int64 `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "问题不存在"})
		return
	}
	// 区队长只能处置本队问题；管理员与安全科不限
	if u.Role == model.RoleDeptLeader && (issue.DeptID == nil || u.DeptID == nil || *issue.DeptID != *u.DeptID) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "无权处置其他区队的问题"})
		return
	}
	note := strings.TrimSpace(req.Note)
	treq := store.TransitionRequest{IssueID: req.ID, ActorID: u.ID, Comment: note}
	switch issue.Status {
	case model.IssueReported:
		treq.Action, treq.From, treq.To = "accept", []string{model.IssueReported}, model.IssueAccepted
		if req.AssigneeID != nil {
			treq.AssigneeID = req.AssigneeID
		} else {
			treq.AssigneeID = issue.ReporterID
		}
	case model.IssueAccepted:
		if issue.AssigneeID != nil && *issue.AssigneeID == u.ID {
			treq.Action, treq.From, treq.To = "rectify", []string{model.IssueAccepted}, model.IssueRectified
			treq.Result = note
			if treq.Result == "" {
				treq.Result = "已整改"
			}
			break
		}
		if !adminForceAllowed(u.Role, true, note) {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "整改需由指派人提交"})
			return
		}
		treq.Action, treq.From, treq.To = "admin_force_rectify", []string{model.IssueAccepted}, model.IssueRectified
		treq.Result = note
		if treq.Result == "" {
			treq.Result = "管理员代整改"
		}
		treq.Comment = "【管理员代整改】" + note
	case model.IssueRectified:
		last, _ := s.Store.LastFlowActor(c.Request.Context(), req.ID, "rectify", "admin_force_rectify")
		if last != nil && *last == u.ID {
			if !adminForceAllowed(u.Role, true, note) {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "验收人不能是整改人"})
				return
			}
			treq.Action = "admin_force_verify"
			treq.Comment = "【管理员强制验收】" + note
		} else {
			treq.Action = "verify"
			treq.Comment = note
		}
		treq.From, treq.To = []string{model.IssueRectified}, model.IssueVerified
		treq.VerifyComment = note
		if treq.VerifyComment == "" {
			treq.VerifyComment = "验收通过"
		}
	case model.IssueVerified:
		last, _ := s.Store.LastFlowActor(c.Request.Context(), req.ID, "verify", "admin_force_verify")
		if last == nil || *last != u.ID {
			if !adminForceAllowed(u.Role, true, note) {
				c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "只有验收人可以闭环"})
				return
			}
			treq.Action = "admin_force_close"
			treq.Comment = "【管理员强制闭环】" + note
		} else {
			treq.Action = "close"
			treq.Comment = note
		}
		treq.From, treq.To = []string{model.IssueVerified}, model.IssueClosed
		treq.Close = true
	default:
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "当前状态不能处置"})
		return
	}
	updated, err := s.Store.TransitionIssue(c.Request.Context(), treq)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "状态流转失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": compatIssueStatus(updated.Status)})
}

// compatIssueFlow 隐患流转记录（小程序详情展示）。
func (s *Server) compatIssueFlow(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "问题不存在"})
		return
	}
	switch {
	case u.Role == model.RoleAdmin || u.Role == model.RoleSafety:
	case u.Role == model.RoleDeptLeader:
		if issue.DeptID == nil || u.DeptID == nil || *issue.DeptID != *u.DeptID {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "无权查看"})
			return
		}
	default:
		if (issue.ReporterID == nil || *issue.ReporterID != u.ID) &&
			(issue.AssigneeID == nil || *issue.AssigneeID != u.ID) {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "无权查看"})
			return
		}
	}
	flows, err := s.Store.ListIssueFlows(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	items := make([]gin.H, 0, len(flows))
	for _, f := range flows {
		items = append(items, gin.H{
			"id":      f.ID,
			"action":  f.Action,
			"text":    compatFlowAction(f.Action),
			"actor":   f.Actor,
			"comment": f.Comment,
			"at":      f.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatUsers 处置人候选：安全科/管理员看全部，区队长看本队，员工只看自己。
func (s *Server) compatUsers(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var deptID *int64
	switch u.Role {
	case model.RoleAdmin, model.RoleSafety:
	case model.RoleDeptLeader:
		deptID = u.DeptID
	default:
		deptID = u.DeptID
	}
	list, _, err := s.Store.ListUsers(c.Request.Context(), deptID, "", 500, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "查询失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, x := range list {
		if x.Status != "active" {
			continue
		}
		items = append(items, gin.H{
			"id": x.ID, "username": x.Username, "name": x.Name, "post": x.Post, "role": x.Role,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// compatRectify 被指派人提交整改（小程序“指派给我的”使用）。
func (s *Server) compatRectify(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ID     int64  `json:"id"`
		Result string `json:"result"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 || strings.TrimSpace(req.Result) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请填写整改结果"})
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), req.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "message": "问题不存在"})
		return
	}
	if issue.AssigneeID == nil || *issue.AssigneeID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "message": "只有责任人可以提交整改"})
		return
	}
	updated, err := s.Store.TransitionIssue(c.Request.Context(), store.TransitionRequest{
		IssueID: req.ID, Action: "rectify", From: []string{model.IssueAccepted}, To: model.IssueRectified,
		ActorID: u.ID, Result: strings.TrimSpace(req.Result), Comment: strings.TrimSpace(req.Result),
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": "当前状态不能提交整改"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": compatIssueStatus(updated.Status)})
}

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
