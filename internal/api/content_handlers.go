package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

var contentKinds = map[string]bool{"article": true, "culture": true, "notice": true}

// listContent 管理端内容列表（含草稿与下架）。
func (s *Server) listContent(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "" && !contentKinds[kind] {
		fail(c, http.StatusBadRequest, "内容类型只能是 article、culture、notice")
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && status != "draft" && status != "published" && status != "archived" {
		fail(c, http.StatusBadRequest, "状态只能是 draft、published、archived")
		return
	}
	list, err := s.Store.ListContent(c.Request.Context(), kind, status, 500)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询内容失败")
		return
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

type contentReq struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Cat     string `json:"cat"`
	Title   string `json:"title"`
	Tag     string `json:"tag"`
	Summary string `json:"summary"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	Cover   string `json:"cover"`
	Ask     string `json:"ask"`
	Minutes int    `json:"minutes"`
	Sort    int    `json:"sort"`
	Status  string `json:"status"`
}

// saveContent 新增或修改内容（安全文化、图文、宣贯解读）。
func (s *Server) saveContent(c *gin.Context) {
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	if !contentKinds[req.Kind] {
		fail(c, http.StatusBadRequest, "内容类型只能是 article、culture、notice")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		fail(c, http.StatusBadRequest, "标题必填")
		return
	}
	status := req.Status
	if status == "" {
		status = "published"
	}
	if status != "draft" && status != "published" && status != "archived" {
		fail(c, http.StatusBadRequest, "状态只能是 draft、published、archived")
		return
	}
	cl := claims(c)

	// 编辑时保留未被本表单覆盖的字段（日期、图片等），避免整条被清空。
	payload := map[string]any{}
	if strings.TrimSpace(req.ID) != "" {
		if old, err := s.Store.GetContentItem(c.Request.Context(), req.ID); err == nil {
			payload = old
		}
	}
	body := make([]string, 0, 8)
	for _, line := range strings.Split(strings.ReplaceAll(req.Body, "\r\n", "\n"), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			body = append(body, p)
		}
	}
	payload["tag"] = strings.TrimSpace(req.Tag)
	payload["summary"] = strings.TrimSpace(req.Summary)
	payload["body"] = body
	if req.Cat != "" {
		payload["cat"] = req.Cat
	}
	if req.Date != "" {
		payload["date"] = req.Date
	}
	if req.Cover != "" {
		payload["cover"] = req.Cover
	}
	if req.Ask != "" {
		payload["ask"] = req.Ask
	}
	if req.Minutes > 0 {
		payload["minutes"] = req.Minutes
	}

	it := &store.ContentItem{
		ID: strings.TrimSpace(req.ID), Kind: req.Kind, Cat: strings.TrimSpace(req.Cat),
		Title: strings.TrimSpace(req.Title), Sort: req.Sort, Status: status, Payload: payload,
	}
	id, err := s.Store.UpsertContent(c.Request.Context(), it, cl.UserID)
	if err != nil {
		s.Log.Error("保存内容失败", "err", err)
		fail(c, http.StatusInternalServerError, "保存内容失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) deleteContent(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		fail(c, http.StatusBadRequest, "id 参数无效")
		return
	}
	if err := s.Store.DeleteContent(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	ok(c, gin.H{"id": id})
}

// listCulture 员工端安全文化/图文列表，只返回已发布内容。
func (s *Server) listCulture(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	if kind == "" {
		kind = "article"
	}
	if !contentKinds[kind] {
		fail(c, http.StatusBadRequest, "内容类型不合法")
		return
	}
	items, err := s.Store.ListContentItems(c.Request.Context(), kind, strings.TrimSpace(c.Query("cat")))
	if err != nil {
		fail(c, http.StatusInternalServerError, "内容加载失败")
		return
	}
	ok(c, gin.H{"items": items, "total": len(items)})
}

// getCultureItem 员工端内容详情，只返回已发布内容。
func (s *Server) getCultureItem(c *gin.Context) {
	item, err := s.Store.GetPublishedContent(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "内容不存在或已下架")
		return
	}
	ok(c, gin.H{"item": item})
}
