// Package spa serves the embedded admin single-page application: the
// production Vue build at rest plus the development-mode reverse proxy to
// the Vite server. Unknown paths fall back to index.html so the SPA router
// owns client-side navigation.
package spa

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// FS exposes the embedded production build for tests and tooling.
func FS() fs.FS {
	adminSub, _ := fs.Sub(distFS, "dist")
	return adminSub
}

// Handler serves the admin SPA: a dev-mode reverse proxy when enabled,
// otherwise files from the embedded production build with an index.html
// fallback for unknown paths.
type Handler struct {
	devMode  bool
	adminURL string
	adminFS  fs.FS
}

func (h *Handler) proxyRequest(w http.ResponseWriter, r *http.Request, target string) {
	targetURL, err := url.Parse(target)
	if err != nil {
		http.Error(w, "Bad gateway", http.StatusBadGateway)
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.SetXForwarded()
		},
	}

	proxy.ServeHTTP(w, r)
}

// ServeAdmin serves the admin SPA: the dev-server proxy in dev mode,
// embedded files in production, index.html for anything else.
func (h *Handler) ServeAdmin(w http.ResponseWriter, r *http.Request) {
	if h.devMode {
		h.proxyRequest(w, r, h.adminURL)
		return
	}

	path := r.URL.Path
	if path == "" || path == "/" {
		http.ServeFileFS(w, r, h.adminFS, "index.html")
		return
	}

	cleaned := strings.TrimPrefix(path, "/")

	if f, err := h.adminFS.Open(cleaned); err == nil {
		_ = f.Close()
		http.ServeFileFS(w, r, h.adminFS, cleaned)
		return
	}

	http.ServeFileFS(w, r, h.adminFS, "index.html")
}

// NewHandler creates a Handler. adminURL is the Vite dev server target used
// only when devMode is set.
func NewHandler(devMode bool, adminURL string) *Handler {
	adminSub, _ := fs.Sub(distFS, "dist")
	return &Handler{
		devMode:  devMode,
		adminURL: adminURL,
		adminFS:  adminSub,
	}
}
