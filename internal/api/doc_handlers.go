package api

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/model"
	"coalrag/internal/service"
	"coalrag/internal/store"
)

func (s *Server) listDocs(c *gin.Context) {
	p, size, offset := pageParams(c)
	status := c.Query("status")
	if cl := claims(c); cl != nil && cl.Role != model.RoleAdmin {
		// 非管理员只看到已发布文档
		status = model.DocPublished
	}
	list, total, err := s.Store.ListDocs(c.Request.Context(), status, strings.TrimSpace(c.Query("keyword")), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询文档失败")
		return
	}
	paged(c, list, total, p, size)
}

func (s *Server) uploadDoc(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请使用 file 字段上传文件")
		return
	}
	if file.Size > 64<<20 {
		fail(c, http.StatusBadRequest, "文件不能超过 64MB")
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "无法读取上传文件")
		return
	}
	defer src.Close()

	cl := claims(c)
	force := c.PostForm("force") == "true"
	doc, reused, err := s.Svc.UploadDoc(c.Request.Context(), file.Filename, src, file.Size,
		c.PostForm("title"), c.PostForm("category"), cl.UserID, force)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"doc": doc, "duplicate": reused})
}

func (s *Server) getDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	doc, err := s.Store.GetDoc(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	if cl := claims(c); cl != nil && cl.Role != model.RoleAdmin && doc.Status != model.DocPublished {
		fail(c, http.StatusForbidden, "文档尚未发布")
		return
	}
	ok(c, doc)
}

func (s *Server) listDocChunks(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	onlyPublished := true
	if cl := claims(c); cl != nil && cl.Role == model.RoleAdmin {
		onlyPublished = false
	}
	list, err := s.Store.ListChunks(c.Request.Context(), id, onlyPublished)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询切片失败")
		return
	}
	ok(c, list)
}

func (s *Server) publishDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	cl := claims(c)
	doc, err := s.Svc.PublishDoc(c.Request.Context(), id, &cl.UserID)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			fail(c, http.StatusConflict, "只有待审核的文档可以发布")
			return
		}
		fail(c, http.StatusInternalServerError, "发布失败")
		return
	}
	ok(c, doc)
}

func (s *Server) rejectDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.Store.UpdateDocStatus(c.Request.Context(), id, model.DocRejected, req.Reason); err != nil {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	ok(c, gin.H{"status": model.DocRejected})
}

func (s *Server) reparseDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	doc, err := s.Store.GetDoc(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	if _, err := s.Store.Enqueue(c.Request.Context(), service.TaskDocParse, map[string]any{"doc_id": id}, 0); err != nil {
		fail(c, http.StatusInternalServerError, "投递解析任务失败")
		return
	}
	_ = s.Store.UpdateDocStatus(c.Request.Context(), id, model.DocParsing, "")
	ok(c, gin.H{"doc_id": doc.ID, "status": model.DocParsing})
}

func (s *Server) deleteDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	doc, err := s.Store.GetDoc(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	if err := s.Svc.Vector.DeleteByDoc(c.Request.Context(), id); err != nil {
		s.Log.Error("删除向量失败", "err", err)
	}
	if err := s.Store.DeleteDoc(c.Request.Context(), id); err != nil {
		fail(c, http.StatusInternalServerError, "删除文档失败")
		return
	}
	if doc.SourceName != "" {
		_ = os.Remove(doc.SourceName)
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) getChunk(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	chunk, err := s.Store.GetChunk(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "切片不存在")
		return
	}
	ok(c, chunk)
}

// getChunkText 返回可直接打开的原文段落，用于引用定位。
func (s *Server) getChunkText(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "id 参数无效")
		return
	}
	chunk, err := s.Store.GetChunk(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "切片不存在")
		return
	}
	body := chunk.ClauseNo
	if chunk.Chapter != "" {
		body = chunk.Chapter + "\n\n" + body
	}
	body = strings.TrimSpace(body) + "\n" + chunk.Content
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}
