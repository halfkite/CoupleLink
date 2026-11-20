package api

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"couplelink/server/internal/webdist"
)

const placeholderPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoupleLink</title>
<style>body{font:16px/1.6 system-ui,-apple-system,"Segoe UI","Microsoft YaHei",sans-serif;margin:0;padding:32px;background:#f7f5fb;color:#241b2f}
main{max-width:520px;margin:0 auto}h1{font-size:20px}a{color:#6b4fa0}.card{background:#fff;border-radius:14px;padding:18px;box-shadow:0 1px 3px rgba(40,20,70,.08)}</style>
</head><body><main>
<h1>CoupleLink</h1>
<div class="card"><p>这次构建没有打包网页客户端，根路径只提供管理入口。</p>
<p><a href="/admin">打开管理页</a> · <a href="/healthz">健康检查</a></p></div>
</main></body></html>`

// webApp serves the embedded couple web app; unknown paths fall back to
// index.html so the single-page client also works on a phone browser.
func (s *Server) webApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dist := webdist.Dist()
	if dist == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(placeholderPage))
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(dist, name)
	if err != nil {
		name = "index.html"
		data, err = fs.ReadFile(dist, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}