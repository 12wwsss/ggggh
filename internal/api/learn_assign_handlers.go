package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// listLearnAssignments 某学习条目的分配列表。
func (s *Server) listLearnAssignments(c *gin.Context) {
	itemID, _ := strconv.ParseInt(c.Query("item_id"), 10, 64)
	list, err := s.Store.ListLearnAssignments(c.Request.Context(), itemID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"list": list, "total": len(list)})
}

// createLearnAssignment 按岗位/区队分配必修条目。
func (s *Server) createLearnAssignment(c *gin.Context) {
	var req struct {
		ItemID int64  `json:"item_id"`
		Post   string `json:"post"`
		DeptID *int64 `json:"dept_id"`
		DueAt  string `json:"due_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	var due *time.Time
	if req.DueAt != "" {
		if t, err := time.Parse("2006-01-02", req.DueAt); err == nil {
			due = &t
		}
	}
	id, err := s.Store.CreateLearnAssignment(c.Request.Context(), req.ItemID, req.Post, req.DeptID, due)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, gin.H{"id": id})
}

// deleteLearnAssignment 删除分配。
func (s *Server) deleteLearnAssignment(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.DeleteLearnAssignment(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, "分配不存在")
		return
	}
	ok(c, gin.H{"id": id})
}
