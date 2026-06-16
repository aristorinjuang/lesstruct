package static

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

//go:embed all:admin
var AdminFS embed.FS

type StaticServer struct {
	adminFS     fs.FS
	devMode     bool
	adminURL    string
	contentPage http.Handler
}

func (s *StaticServer) proxyRequest(w http.ResponseWriter, r *http.Request, target string) {
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

func (s *StaticServer) ServeAdmin(w http.ResponseWriter, r *http.Request) {
	if s.devMode {
		s.proxyRequest(w, r, s.adminURL)
		return
	}

	path := r.URL.Path
	if path == "" || path == "/" {
		http.ServeFileFS(w, r, s.adminFS, "index.html")
		return
	}

	cleaned := strings.TrimPrefix(path, "/")

	if f, err := s.adminFS.Open(cleaned); err == nil {
		_ = f.Close()
		http.ServeFileFS(w, r, s.adminFS, cleaned)
		return
	}

	http.ServeFileFS(w, r, s.adminFS, "index.html")
}

func (s *StaticServer) ServeContent(w http.ResponseWriter, r *http.Request) {
	if s.contentPage != nil {
		s.contentPage.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func NewStaticServer(
	devMode bool,
	adminURL string,
	contentPage http.Handler,
) *StaticServer {
	adminSub, _ := fs.Sub(AdminFS, "admin")

	return &StaticServer{
		adminFS:     adminSub,
		devMode:     devMode,
		adminURL:    adminURL,
		contentPage: contentPage,
	}
}
