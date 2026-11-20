package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/store"
)

type issueReq struct {
	Title       string     `json:"title" binding:"required"`
	Description string     `json:"description"`
	CateID      *int64     `json:"cate_id"`
	DeptID      *int64     `json:"dept_id"`
	AssigneeID  *int64     `json:"assignee_id"`
	Severity    string     `json:"severity"`
	Deadline    *time.Time `json:"deadline"`
}

// listIssueCates 供上报表单选择分类。
func (s *Server) listIssueCates(c *gin.Context) {
	list, err := s.Store.ListIssueCates(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询分类失败")
		return
	}
	ok(c, list)
}

func (s *Server) createIssue(c *gin.Context) {
	var req issueReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		fail(c, http.StatusBadRequest, "问题标题必填")
		return
	}
	severity := req.Severity
	if severity == "" {
		severity = "normal"
	}
	switch severity {
	case "low", "normal", "high":
	default:
		fail(c, http.StatusBadRequest, "severity 只能是 low、normal、high")
		return
	}
	cl := claims(c)
	dept := req.DeptID
	if dept == nil && cl.DeptID > 0 {
		d := cl.DeptID
		dept = &d
	}
	issue, err := s.Store.CreateIssue(c.Request.Context(), &model.Issue{
		Title: req.Title, Description: req.Description, CateID: req.CateID, DeptID: dept,
		ReporterID: &cl.UserID, AssigneeID: req.AssigneeID, Severity: severity, Deadline: req.Deadline,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "上报失败："+err.Error())
		return
	}
	ok(c, issue)
}

func (s *Server) listIssues(c *gin.Context) {
	p, size, offset := pageParams(c)
	cl := claims(c)
	var dept, reporter *int64
	switch scopeOf(c) {
	case "self":
		u := cl.UserID
		reporter = &u
	case "dept":
		d := cl.DeptID
		dept = &d
	}
	if v := c.Query("dept_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dept = &id
		}
	}
	var list []model.Issue
	var total int
	var err error
	if reporter != nil && dept == nil {
		// 员工：我上报的 + 指派给我的
		list, total, err = s.Store.ListIssuesByUser(c.Request.Context(), *reporter, size, offset)
	} else {
		list, total, err = s.Store.ListIssues(c.Request.Context(), c.Query("status"), dept, reporter,
			strings.TrimSpace(c.Query("keyword")), size, offset)
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询问题失败")
		return
	}
	paged(c, list, total, p, size)
}

// canAccessIssue 按数据范围校验问题可见性。
func (s *Server) canAccessIssue(c *gin.Context, issue *model.Issue) bool {
	cl := claims(c)
	switch scopeOf(c) {
	case "all":
		return true
	case "dept":
		return issue.DeptID != nil && *issue.DeptID == cl.DeptID
	default:
		if issue.ReporterID != nil && *issue.ReporterID == cl.UserID {
			return true
		}
		return issue.AssigneeID != nil && *issue.AssigneeID == cl.UserID
	}
}

func (s *Server) getIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权查看该问题")
		return
	}
	ok(c, issue)
}

func (s *Server) listIssueFlow(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权查看该问题")
		return
	}
	flows, err := s.Store.ListIssueFlows(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询流转记录失败")
		return
	}
	ok(c, flows)
}

// authorizeIssue 校验当前用户能操作该问题：RBAC 只决定“能写问题”，
// 具体到哪一条还要过数据范围（管理员全部、区队长本部门、员工本人相关）。
func (s *Server) authorizeIssue(c *gin.Context, id int64) bool {
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return false
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权操作该问题")
		return false
	}
	return true
}

func (s *Server) transition(c *gin.Context, req store.TransitionRequest) {
	issue, err := s.Store.TransitionIssue(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			fail(c, http.StatusNotFound, "问题不存在")
		case errors.Is(err, store.ErrConflict):
			fail(c, http.StatusConflict, "当前状态不允许该操作")
		default:
			fail(c, http.StatusInternalServerError, "状态流转失败")
		}
		return
	}
	ok(c, issue)
}

// isIssueManager 区队长、安全科或管理员才能受理/驳回/验收/闭环。
func isIssueManager(role string) bool {
	return role == model.RoleAdmin || role == model.RoleDeptLeader || role == model.RoleSafety
}

// adminForceAllowed 管理员强制推进：必须 force=true 且填写原因。
func adminForceAllowed(role string, force bool, reason string) bool {
	return role == model.RoleAdmin && force && strings.TrimSpace(reason) != ""
}

func (s *Server) acceptIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if !s.authorizeIssue(c, id) {
		return
	}
	cl := claims(c)
	if !isIssueManager(cl.Role) {
		fail(c, http.StatusForbidden, "没有受理权限")
		return
	}
	var req struct {
		AssigneeID *int64     `json:"assignee_id"`
		Deadline   *time.Time `json:"deadline"`
		Comment    string     `json:"comment"`
	}
	_ = c.ShouldBindJSON(&req)
	assignee := req.AssigneeID
	if assignee == nil {
		if issue, err := s.Store.GetIssue(c.Request.Context(), id); err == nil {
			assignee = issue.ReporterID
		}
	}
	s.transition(c, store.TransitionRequest{
		IssueID: id, Action: "accept", From: []string{model.IssueReported}, To: model.IssueAccepted,
		ActorID: cl.UserID, Comment: req.Comment, AssigneeID: assignee,
	})
}

func (s *Server) rejectIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if !s.authorizeIssue(c, id) {
		return
	}
	var req struct {
		Comment string `json:"comment" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "驳回必须填写原因")
		return
	}
	cl := claims(c)
	if !isIssueManager(cl.Role) {
		fail(c, http.StatusForbidden, "没有驳回权限")
		return
	}
	s.transition(c, store.TransitionRequest{
		IssueID: id, Action: "reject", From: []string{model.IssueReported}, To: model.IssueRejected,
		ActorID: cl.UserID, Comment: req.Comment,
	})
}

func (s *Server) rectifyIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if !s.authorizeIssue(c, id) {
		return
	}
	var req struct {
		Result  string `json:"result" binding:"required"`
		Comment string `json:"comment"`
		Force   bool   `json:"force"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "整改结果必填")
		return
	}
	cl := claims(c)
	action := "rectify"
	comment := req.Comment
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if issue.Status != model.IssueAccepted {
		fail(c, http.StatusConflict, "当前状态不允许提交整改")
		return
	}
	if issue.AssigneeID == nil || *issue.AssigneeID != cl.UserID {
		if !adminForceAllowed(cl.Role, req.Force, req.Reason) {
			fail(c, http.StatusForbidden, "整改需由指派人提交（管理员可强制代提交并说明原因）")
			return
		}
		action = "admin_force_rectify"
		comment = "【管理员代整改】" + strings.TrimSpace(req.Reason) + " " + comment
	}
	s.transition(c, store.TransitionRequest{
		IssueID: id, Action: action, From: []string{model.IssueAccepted}, To: model.IssueRectified,
		ActorID: cl.UserID, Comment: comment, Result: req.Result,
	})
}

func (s *Server) verifyIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if !s.authorizeIssue(c, id) {
		return
	}
	var req struct {
		Pass    *bool  `json:"pass"`
		Comment string `json:"comment" binding:"required"`
		Force   bool   `json:"force"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "验收意见必填")
		return
	}
	cl := claims(c)
	if !isIssueManager(cl.Role) {
		fail(c, http.StatusForbidden, "没有验收权限")
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if issue.Status != model.IssueRectified {
		fail(c, http.StatusConflict, "当前状态不允许验收")
		return
	}
	pass := true
	if req.Pass != nil {
		pass = *req.Pass
	}
	action := "verify"
	comment := req.Comment
	if !pass {
		action = "verify_reject"
	}
	if last, err := s.Store.LastFlowActor(c.Request.Context(), id, "rectify", "admin_force_rectify"); err == nil && last != nil && *last == cl.UserID {
		if !adminForceAllowed(cl.Role, req.Force, req.Reason) {
			fail(c, http.StatusForbidden, "验收人不能是整改人（管理员可强制验收并说明原因）")
			return
		}
		action = "admin_force_" + action
		comment = "【管理员强制验收】" + strings.TrimSpace(req.Reason) + " " + comment
	}
	if !pass {
		s.transition(c, store.TransitionRequest{
			IssueID: id, Action: action, From: []string{model.IssueRectified}, To: model.IssueAccepted,
			ActorID: cl.UserID, Comment: comment, VerifyComment: req.Comment,
		})
		return
	}
	s.transition(c, store.TransitionRequest{
		IssueID: id, Action: action, From: []string{model.IssueRectified}, To: model.IssueVerified,
		ActorID: cl.UserID, Comment: comment, VerifyComment: req.Comment,
	})
}

func (s *Server) closeIssue(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if !s.authorizeIssue(c, id) {
		return
	}
	var req struct {
		Comment string `json:"comment"`
		Force   bool   `json:"force"`
		Reason  string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	cl := claims(c)
	if !isIssueManager(cl.Role) {
		fail(c, http.StatusForbidden, "没有闭环权限")
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if issue.Status != model.IssueVerified {
		fail(c, http.StatusConflict, "当前状态不允许闭环")
		return
	}
	action := "close"
	comment := req.Comment
	if last, err := s.Store.LastFlowActor(c.Request.Context(), id, "verify", "admin_force_verify"); err == nil && (last == nil || *last != cl.UserID) {
		if !adminForceAllowed(cl.Role, req.Force, req.Reason) {
			fail(c, http.StatusForbidden, "只有验收人可以闭环（管理员可强制闭环并说明原因）")
			return
		}
		action = "admin_force_close"
		comment = "【管理员强制闭环】" + strings.TrimSpace(req.Reason) + " " + comment
	}
	s.transition(c, store.TransitionRequest{
		IssueID: id, Action: action, From: []string{model.IssueVerified}, To: model.IssueClosed,
		ActorID: cl.UserID, Comment: comment, Close: true,
	})
}

func (s *Server) issueStats(c *gin.Context) {
	cl := claims(c)
	var dept, reporter *int64
	switch scopeOf(c) {
	case "all":
	case "dept":
		if cl.DeptID > 0 {
			d := cl.DeptID
			dept = &d
		}
	default:
		u := cl.UserID
		reporter = &u
	}
	stats, err := s.Store.IssueStats(c.Request.Context(), dept, reporter)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	ok(c, stats)
}

func (s *Server) uploadIssueAttachment(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权操作该问题")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请使用 file 字段上传附件")
		return
	}
	if file.Size > 32<<20 {
		fail(c, http.StatusBadRequest, "附件不能超过 32MB")
		return
	}
	dir := filepath.Join(s.Cfg.StorageDir, "issues", strconv.FormatInt(id, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "创建附件目录失败")
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "无法读取附件")
		return
	}
	defer src.Close()
	dstPath := filepath.Join(dir, filepath.Base(file.Filename))
	dst, err := os.Create(dstPath)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, h), src); err != nil {
		dst.Close()
		fail(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	dst.Close()

	cl := claims(c)
	aid, err := s.Store.AddIssueAttachment(c.Request.Context(), &model.IssueAttachment{
		IssueID: id, FileName: dstPath, FileSize: file.Size,
		SHA256: hex.EncodeToString(h.Sum(nil)), UploaderID: &cl.UserID,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "记录附件失败")
		return
	}
	ok(c, gin.H{"id": aid, "file_name": file.Filename})
}

// downloadIssueAttachment 按数据范围校验后返回附件原文件。
func (s *Server) downloadIssueAttachment(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	aid, err := strconv.ParseInt(c.Param("aid"), 10, 64)
	if err != nil || aid <= 0 {
		fail(c, http.StatusBadRequest, "附件 id 无效")
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权查看该问题")
		return
	}
	att, err := s.Store.GetIssueAttachment(c.Request.Context(), aid)
	if err != nil || att.IssueID != id {
		fail(c, http.StatusNotFound, "附件不存在")
		return
	}
	if _, err := os.Stat(att.FileName); err != nil {
		fail(c, http.StatusNotFound, "附件文件已丢失")
		return
	}
	c.FileAttachment(att.FileName, filepath.Base(att.FileName))
}

func (s *Server) listIssueAttachments(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	issue, err := s.Store.GetIssue(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "问题不存在")
		return
	}
	if !s.canAccessIssue(c, issue) {
		fail(c, http.StatusForbidden, "无权查看该问题")
		return
	}
	list, err := s.Store.ListIssueAttachments(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询附件失败")
		return
	}
	ok(c, list)
}
