// Package web embeds the built React single-page application and serves it with a
// client-side-routing fallback to index.html.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// DistFS returns the embedded frontend build directory as a filesystem.
func DistFS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic("web: embedded dist missing: " + err.Error())
	}
	return sub
}

// Handler serves the embedded SPA. Requests for existing files are served directly;
// any other (non-asset) path falls back to index.html so client-side routes work.
func Handler() http.Handler {
	dist := DistFS()
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		// index.html must not be cached so a rebuilt app (with new hashed asset names) is
		// picked up on the next load; the hashed assets themselves stay cacheable.
		if p == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if _, err := fs.Stat(dist, p); err != nil {
			// Unknown path: let the SPA's router handle it (unless it looks like a
			// missing static asset, which should 404 honestly).
			if looksLikeAsset(p) {
				http.NotFound(w, r)
				return
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			serveIndex(w, r2, dist)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	f, err := dist.Open("index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotImplemented)
		return
	}
	defer f.Close()
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func looksLikeAsset(p string) bool {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return false
	}
	switch strings.ToLower(p[i:]) {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".map", ".json", ".wasm":
		return true
	}
	return false
}
