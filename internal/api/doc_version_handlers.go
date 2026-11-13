package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coalrag/internal/store"
)

// listDocVersions 列出文档的历史版本。
func (s *Server) listDocVersions(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	list, err := s.Store.ListDocVersions(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询版本失败")
		return
	}
	ok(c, list)
}

// getDocVersion 返回某个版本的切片，用于对比查看。
func (s *Server) getDocVersion(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		fail(c, http.StatusBadRequest, "版本号无效")
		return
	}
	chunks, meta, err := s.Store.LoadDocVersion(c.Request.Context(), id, version)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "版本不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "读取版本失败")
		return
	}
	ok(c, gin.H{"version": meta, "chunks": chunks})
}

// diffDocVersions 对比两个版本：新增 / 删除 / 修改的条款号。
func (s *Server) diffDocVersions(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	from, err1 := strconv.Atoi(c.Query("from"))
	to, err2 := strconv.Atoi(c.Query("to"))
	if err1 != nil || err2 != nil || from <= 0 || to <= 0 {
		fail(c, http.StatusBadRequest, "from / to 版本号必填")
		return
	}
	diff, err := s.Svc.DiffVersions(c.Request.Context(), id, from, to)
	if err != nil {
		fail(c, http.StatusNotFound, "版本不存在")
		return
	}
	ok(c, diff)
}

// rollbackDoc 回滚到指定版本，生成一个新版本。
func (s *Server) rollbackDoc(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		fail(c, http.StatusBadRequest, "版本号无效")
		return
	}
	cl := claims(c)
	doc, err := s.Svc.RollbackDoc(c.Request.Context(), id, version, &cl.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "版本不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "回滚失败："+err.Error())
		return
	}
	ok(c, doc)
}
