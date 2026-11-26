package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// zipIssueAttachments 把一个工单的附件打成一个 zip 下载。
func (s *Server) zipIssueAttachments(c *gin.Context) {
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
	if err != nil || len(list) == 0 {
		fail(c, http.StatusBadRequest, "该工单没有附件")
		return
	}
	name := issue.Code
	if name == "" {
		name = fmt.Sprintf("issue-%d", id)
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	zw := zip.NewWriter(c.Writer)
	defer zw.Close()
	for _, a := range list {
		p := a.FileName
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.Cfg.StorageDir, "uploads", filepath.Base(p))
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		w, err := zw.Create(filepath.Base(p))
		if err != nil {
			f.Close()
			continue
		}
		_, _ = io.Copy(w, f)
		f.Close()
	}
}
