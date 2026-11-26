package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
)

func (s *Server) listPosts(c *gin.Context) {
	list, err := s.Store.ListPosts(c.Request.Context(), c.Query("all") != "1")
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询岗位失败")
		return
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

func (s *Server) createPost(c *gin.Context) {
	var p model.Post
	if err := c.ShouldBindJSON(&p); err != nil || strings.TrimSpace(p.Name) == "" {
		fail(c, http.StatusBadRequest, "岗位名称必填")
		return
	}
	if p.Status == "" {
		p.Status = "active"
	}
	if p.Status != "active" && p.Status != "disabled" {
		fail(c, http.StatusBadRequest, "状态不合法")
		return
	}
	id, err := s.Store.CreatePost(c.Request.Context(), &p)
	if err != nil {
		fail(c, http.StatusConflict, "岗位已存在或保存失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) updatePost(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var p model.Post
	if err := c.ShouldBindJSON(&p); err != nil || strings.TrimSpace(p.Name) == "" {
		fail(c, http.StatusBadRequest, "岗位名称必填")
		return
	}
	if p.Status == "" {
		p.Status = "active"
	}
	p.ID = id
	if err := s.Store.UpdatePost(c.Request.Context(), &p); err != nil {
		fail(c, http.StatusConflict, "保存失败：岗位名重复或记录不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}

func (s *Server) deletePost(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.DeletePost(c.Request.Context(), id); err != nil {
		fail(c, http.StatusConflict, "岗位仍被账号使用，不能删除")
		return
	}
	ok(c, gin.H{"id": id})
}
