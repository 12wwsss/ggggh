// Package web 托管前端构建产物（Vite 构建的 dist 目录），
// 通过 go:embed 打进同一个二进制，部署时不需要额外的 nginx 静态目录。
package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler 提供两个入口：员工端 index.html 与后台 admin.html。
type Handler struct {
	fsys fs.FS
	ok   bool
}

// New 构造静态资源处理器。dist 里没有 index.html 时返回一个
// 明确提示“前端未构建”的处理器，而不是静默 404。
func New() *Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return &Handler{}
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return &Handler{}
	}
	return &Handler{fsys: sub, ok: true}
}

// Ready 表示前端产物是否可用。
func (h *Handler) Ready() bool { return h.ok }

// ServeHTTP 按路径选择入口，未知路径回退到对应 SPA 的 index.html。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("前端尚未构建：请在 web 目录执行 pnpm build，再重新编译服务端。\n"))
		return
	}

	urlPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if urlPath == "" || urlPath == "." {
		urlPath = "index.html"
	}

	// 后台使用 /admin/ 前缀的 history 路由，其余路径交给员工端。
	isAdmin := urlPath == "admin" || strings.HasPrefix(urlPath, "admin/") || urlPath == "admin.html"
	if isAdmin && !strings.Contains(path.Base(urlPath), ".") {
		urlPath = "admin.html"
	}

	// 静态资源缺失时不要回退成 HTML，避免浏览器把 JS 当页面解析。
	if strings.HasPrefix(urlPath, "assets/") {
		if _, err := fs.Stat(h.fsys, urlPath); err != nil {
			http.NotFound(w, r)
			return
		}
		h.serve(w, r, urlPath, true)
		return
	}

	// 小程序/微信的域名校验文件是 .txt，缺失时必须返回 404：
	// 回退成 index.html 会让校验方拿到 HTML 而失败，且原因很难查。
	if strings.HasSuffix(urlPath, ".txt") {
		if _, err := fs.Stat(h.fsys, urlPath); err != nil {
			http.NotFound(w, r)
			return
		}
	}

	if st, err := fs.Stat(h.fsys, urlPath); err == nil && !st.IsDir() {
		h.serve(w, r, urlPath, false)
		return
	}

	if isAdmin {
		h.serve(w, r, "admin.html", false)
		return
	}
	h.serve(w, r, "index.html", false)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, name string, immutable bool) {
	f, err := h.fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	if immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	seeker, ok := f.(interface {
		Read([]byte) (int, error)
		Seek(int64, int) (int64, error)
	})
	if ok {
		http.ServeContent(w, r, name, st.ModTime(), seeker)
		return
	}
	http.ServeContent(w, r, name, st.ModTime(), nil)
}
