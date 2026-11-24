package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

// compatMaterials 小程序资料/视频列表。
func (s *Server) compatMaterials(c *gin.Context) {
	cat := strings.TrimSpace(c.Query("cat"))
	list, _, err := s.Store.ListMedia(c.Request.Context(), cat, 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "资料加载失败"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, m := range list {
		items = append(items, gin.H{
			"id": m.ID, "title": m.Title, "cat": m.Category, "type": m.Type,
			"url": m.URL, "size": m.SizeBytes, "at": m.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}

// serveMedia 提供上传的视频文件。
func (s *Server) serveMedia(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	if name == "" || name == "." || name == ".." {
		c.Status(http.StatusNotFound)
		return
	}
	path := filepath.Join(s.Cfg.StorageDir, "media", name)
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(path)
}

func (s *Server) mediaAllowed(c *gin.Context) bool {
	role := claims(c).Role
	if role != "admin" && role != "safety" {
		fail(c, http.StatusForbidden, "没有资料管理权限")
		return false
	}
	return true
}

func (s *Server) listMedia(c *gin.Context) {
	if !s.mediaAllowed(c) {
		return
	}
	cat := strings.TrimSpace(c.Query("cat"))
	list, total, err := s.Store.ListMedia(c.Request.Context(), cat, 200, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"list": list, "total": total})
}

func (s *Server) uploadMedia(c *gin.Context) {
	if !s.mediaAllowed(c) {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择视频文件")
		return
	}
	if file.Size > 300<<20 {
		fail(c, http.StatusBadRequest, "文件不能超过 300MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allow := map[string]bool{".mp4": true, ".webm": true, ".mov": true, ".m4v": true, ".ogg": true}
	if !allow[ext] {
		fail(c, http.StatusBadRequest, "只支持 mp4/webm/mov/ogg 视频")
		return
	}
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file.Filename), ext)
	}
	category := strings.TrimSpace(c.PostForm("category"))
	if category == "" {
		category = "操作视频"
	}
	dir := filepath.Join(s.Cfg.StorageDir, "media")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "存储目录不可用")
		return
	}
	name := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), claims(c).UserID, ext)
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		fail(c, http.StatusInternalServerError, "保存文件失败")
		return
	}
	item := &store.MediaItem{Title: title, Category: category, Type: "video",
		URL: "/media/" + name, SizeBytes: file.Size}
	id, err := s.Store.CreateMedia(c.Request.Context(), item, claims(c).UserID)
	if err != nil {
		_ = os.Remove(dst)
		fail(c, http.StatusInternalServerError, "记录失败")
		return
	}
	ok(c, gin.H{"id": id, "url": "/media/" + name})
}

// createMediaLink 外链视频/资料（矿上文件服务器、OSS 等）。
func (s *Server) createMediaLink(c *gin.Context) {
	if !s.mediaAllowed(c) {
		return
	}
	var req struct {
		Title    string `json:"title"`
		Category string `json:"category"`
		URL      string `json:"url"`
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		fail(c, http.StatusBadRequest, "请填写标题和地址")
		return
	}
	if req.Title == "" {
		req.Title = "未命名资料"
	}
	if req.Category == "" {
		req.Category = "操作视频"
	}
	if req.Type == "" {
		req.Type = "video"
	}
	item := &store.MediaItem{Title: req.Title, Category: req.Category, Type: req.Type, URL: req.URL}
	id, err := s.Store.CreateMedia(c.Request.Context(), item, claims(c).UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) deleteMedia(c *gin.Context) {
	if !s.mediaAllowed(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	url, err := s.Store.DeleteMedia(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "资料不存在")
		return
	}
	if name := strings.TrimPrefix(url, "/media/"); name != url {
		_ = os.Remove(filepath.Join(s.Cfg.StorageDir, "media", filepath.Base(name)))
	}
	ok(c, gin.H{"id": id})
}
