package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// compatMessages 我的站内消息。
func (s *Server) compatMessages(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	_, size, offset := pageParams(c)
	kind := c.Query("kind")
	unreadOnly := c.Query("unread") == "1"
	list, total, err := s.Store.ListMessages(c.Request.Context(), u.ID, kind, unreadOnly, size, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "消息加载失败"})
		return
	}
	unread, _ := s.Store.UnreadMessageCount(c.Request.Context(), u.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": list, "total": total, "unread": unread})
}

// compatMessageRead 单条/全部已读。
func (s *Server) compatMessageRead(c *gin.Context) {
	u := s.compatCurrentUser(c)
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "请先登录"})
		return
	}
	var req struct {
		ID  int64 `json:"id"`
		All bool  `json:"all"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
		return
	}
	if req.All {
		if err := s.Store.MarkAllMessagesRead(c.Request.Context(), u.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "标记失败"})
			return
		}
	} else {
		if req.ID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "参数不正确"})
			return
		}
		_ = s.Store.MarkMessageRead(c.Request.Context(), u.ID, req.ID)
	}
	unread, _ := s.Store.UnreadMessageCount(c.Request.Context(), u.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "unread": unread})
}
