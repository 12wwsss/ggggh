package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/service"
)

var reReportName = regexp.MustCompile(`^[A-Za-z0-9._-]+\.json$`)

type evalReq struct {
	Cases    []service.EvalCase `json:"cases" binding:"required"`
	TopK     int                `json:"top_k"`
	SaveName string             `json:"save_name"`
}

func (s *Server) evalDir() string { return filepath.Join(s.Cfg.StorageDir, "eval") }

func (s *Server) saveReport(name string, payload any) (string, error) {
	dir := s.evalDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Server) evalRetrieval(c *gin.Context) {
	var req evalReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Cases) == 0 {
		fail(c, http.StatusBadRequest, "cases 不能为空")
		return
	}
	rep, err := s.Svc.EvalRetrieval(c.Request.Context(), req.Cases, req.TopK)
	if err != nil {
		fail(c, http.StatusInternalServerError, "评测失败："+err.Error())
		return
	}
	name := req.SaveName
	if name == "" {
		name = "retrieval-" + time.Now().Format("20060102-150405") + ".json"
	}
	path, err := s.saveReport(name, rep)
	if err != nil {
		s.Log.Error("保存评测报告失败", "err", err)
	}
	ok(c, gin.H{"report": rep, "saved": path})
}

func (s *Server) evalQA(c *gin.Context) {
	var req evalReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Cases) == 0 {
		fail(c, http.StatusBadRequest, "cases 不能为空")
		return
	}
	rep, err := s.Svc.EvalQA(c.Request.Context(), req.Cases)
	if err != nil {
		fail(c, http.StatusInternalServerError, "评测失败："+err.Error())
		return
	}
	name := req.SaveName
	if name == "" {
		name = "qa-" + time.Now().Format("20060102-150405") + ".json"
	}
	path, err := s.saveReport(name, rep)
	if err != nil {
		s.Log.Error("保存评测报告失败", "err", err)
	}
	ok(c, gin.H{"report": rep, "saved": path})
}

func (s *Server) listEvalReports(c *gin.Context) {
	entries, err := os.ReadDir(s.evalDir())
	if err != nil {
		if os.IsNotExist(err) {
			ok(c, []any{})
			return
		}
		fail(c, http.StatusInternalServerError, "读取评测报告失败")
		return
	}
	type item struct {
		Name     string    `json:"name"`
		Size     int64     `json:"size"`
		Modified time.Time `json:"modified"`
	}
	out := make([]item, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !reReportName.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, item{Name: e.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	ok(c, out)
}

func (s *Server) getEvalReport(c *gin.Context) {
	name := c.Param("name")
	if !reReportName.MatchString(name) {
		fail(c, http.StatusBadRequest, "报告名不合法")
		return
	}
	b, err := os.ReadFile(filepath.Join(s.evalDir(), name))
	if err != nil {
		fail(c, http.StatusNotFound, "报告不存在")
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", b)
}
