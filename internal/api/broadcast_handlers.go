package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
)

func (s *Server) listBroadcastTasks(c *gin.Context) {
	p, size, offset := pageParams(c)
	status := c.Query("status")
	cl := claims(c)
	if cl.Role == model.RoleEmployee {
		status = "published"
	}
	list, total, err := s.Store.ListBroadcastTasks(c.Request.Context(), status, size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询宣贯任务失败")
		return
	}
	paged(c, list, total, p, size)
}

type broadcastReq struct {
	Title      string     `json:"title" binding:"required"`
	Content    string     `json:"content" binding:"required"`
	DocIDs     []int64    `json:"doc_ids"`
	DeptIDs    []int64    `json:"dept_ids"`
	PostIDs    []string   `json:"post_ids"`
	RequireAck *bool      `json:"require_ack"`
	Deadline   *time.Time `json:"deadline"`
	Publish    bool       `json:"publish"`
}

func (s *Server) createBroadcast(c *gin.Context) {
	var req broadcastReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "标题与内容必填")
		return
	}
	requireAck := true
	if req.RequireAck != nil {
		requireAck = *req.RequireAck
	}
	// 先以草稿入库，再按需发布；发布动作负责生成签收记录。
	status := "draft"
	cl := claims(c)
	id, err := s.Store.CreateBroadcast(c.Request.Context(), &model.BroadcastTask{
		Title: req.Title, Content: req.Content, DocIDs: req.DocIDs, DeptIDs: req.DeptIDs,
		PostIDs:     normalizePosts(req.PostIDs),
		PublisherID: &cl.UserID, RequireAck: requireAck, Deadline: req.Deadline, Status: status,
	})
	if err != nil {
		s.Log.Error("新建宣贯任务失败", "err", err)
		fail(c, http.StatusInternalServerError, "新建宣贯任务失败")
		return
	}
	if req.Publish {
		if _, err := s.Store.PublishBroadcast(c.Request.Context(), id); err != nil {
			s.Log.Error("发布宣贯任务失败", "err", err, "task_id", id)
			fail(c, http.StatusInternalServerError, "发布宣贯任务失败")
			return
		}
	}
	ok(c, gin.H{"id": id, "status": status})
}

func (s *Server) publishBroadcast(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	n, err := s.Store.PublishBroadcast(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusConflict, "只有草稿状态的任务可以发布")
		return
	}
	ok(c, gin.H{"published": true, "recipients": n})
}

func (s *Server) listBroadcastRecords(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	p, size, offset := pageParams(c)
	list, total, err := s.Store.ListBroadcastRecords(c.Request.Context(), id, c.Query("status"), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询签收明细失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) myBroadcasts(c *gin.Context) {
	cl := claims(c)
	onlyPending := c.Query("pending") == "true"
	list, err := s.Store.ListMyBroadcasts(c.Request.Context(), cl.UserID, onlyPending)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询我的宣贯任务失败")
		return
	}
	ok(c, list)
}

func (s *Server) ackBroadcast(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Ack bool `json:"ack"`
	}
	_ = c.ShouldBindJSON(&req)
	cl := claims(c)
	if err := s.Store.AckBroadcast(c.Request.Context(), id, cl.UserID, req.Ack); err != nil {
		fail(c, http.StatusNotFound, "没有待签收的任务")
		return
	}
	ok(c, gin.H{"ack": req.Ack})
}

// normalizePosts 去掉空白岗位名并去重，避免发布时重复匹配。
func normalizePosts(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// draftBroadcast 生成制度宣贯的「解读要点」初稿。
// 建设方案六（6）：新制度发布后平台自动生成解读要点，管理员确认后发布。
func (s *Server) draftBroadcast(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请提交制度标题与正文")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	d, err := s.Svc.DraftBroadcast(ctx, req.Title, req.Content)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, d)
}
