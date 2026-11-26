package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) getSettings(c *gin.Context) {
	contact, _ := s.Store.GetSetting(c.Request.Context(), "safety_contact")
	ok(c, gin.H{"safety_contact": contact})
}

func (s *Server) updateSettings(c *gin.Context) {
	var req struct {
		SafetyContact string `json:"safety_contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	if err := s.Store.SetSetting(c.Request.Context(), "safety_contact", req.SafetyContact); err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, gin.H{"updated": true})
}
