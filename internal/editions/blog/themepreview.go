package blog

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themenapi "github.com/aristorinjuang/lesstruct/modules/themegen/api"
	themegendomain "github.com/aristorinjuang/lesstruct/modules/themegen/domain"
)

const (
	// maxPreviewEntries caps cached preview surfaces; the oldest built is
	// evicted first.
	maxPreviewEntries = 4
)

// previewEntry is one cached preview surface: a fully built content-page
// handler plus its static files for a theme, pinned to the theme dir
// modtime it was built from.
type previewEntry struct {
	handler http.Handler
	static  *sitessr.StaticFilesHandler
	modTime time.Time
	builtAt time.Time
}

// previewCache memoizes built preview surfaces by theme directory.
type previewCache struct {
	mu      sync.Mutex
	entries map[string]*previewEntry
}

func newPreviewCache() *previewCache {
	return &previewCache{entries: make(map[string]*previewEntry)}
}

// previewTheme resolves the effective preview theme for a request: the
// signed cookie wins over nothing else here — callers apply it between
// THEME_DIR and [theme] active by dispatching to the preview surfaces only
// when it resolves. Forged, expired, missing, or invalid themes resolve nil.
func (s *state) previewTheme(r *http.Request) *sitessr.Theme {
	if len(s.previewKey) == 0 || s.cfg == nil {
		return nil
	}
	// A manual THEME_DIR override always wins; preview never hijacks it.
	if strings.TrimSpace(s.cfg.ThemeDir) != "" {
		return nil
	}
	cookie, err := r.Cookie(themenapi.PreviewCookieName)
	if err != nil {
		return nil
	}
	name, ok := themenapi.VerifyPreviewCookie(cookie.Value, s.previewKey)
	if !ok {
		return nil
	}
	dir := filepath.Join(s.cfg.ThemesRoot(), name)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	theme, err := readInstalledThemeCSS(dir, name)
	if err != nil {
		return nil
	}
	if report := themegendomain.NewValidator().Validate(context.Background(), theme); !report.Valid {
		return nil
	}
	return &sitessr.Theme{Dir: dir}
}

// previewEntryFor returns the cached preview surface for theme, building it
// on a miss or when the theme directory changed since the build. A build
// failure reports false so callers fall back to the main surfaces.
func (s *state) previewEntryFor(theme *sitessr.Theme) (*previewEntry, bool) {
	if s.previews == nil || s.previewBuilder == nil {
		return nil, false
	}
	info, err := os.Stat(theme.Dir)
	if err != nil || !info.IsDir() {
		return nil, false
	}
	s.previews.mu.Lock()
	defer s.previews.mu.Unlock()
	if entry, ok := s.previews.entries[theme.Dir]; ok && entry.modTime.Equal(info.ModTime()) {
		return entry, true
	}
	handler, staticFiles, err := s.previewBuilder(theme)
	if err != nil {
		return nil, false
	}
	if len(s.previews.entries) >= maxPreviewEntries {
		oldest := ""
		var oldestTime time.Time
		for name, entry := range s.previews.entries {
			if oldest == "" || entry.builtAt.Before(oldestTime) {
				oldest = name
				oldestTime = entry.builtAt
			}
		}
		delete(s.previews.entries, oldest)
	}
	entry := &previewEntry{
		handler: handler,
		static:  staticFiles,
		modTime: info.ModTime(),
		builtAt: time.Now(),
	}
	s.previews.entries[theme.Dir] = entry
	return entry, true
}

// noStoreWriter forces Cache-Control: no-store at WriteHeader time, after
// the wrapped handler has set its own headers, so previewed responses are
// never cached by the previewing browser.
type noStoreWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *noStoreWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.Header().Set("Cache-Control", "no-store")
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write ensures the no-store header is set even when the handler never
// calls WriteHeader explicitly (the implicit 200 would otherwise bypass
// the override above).
func (w *noStoreWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// previewPageDispatch routes preview-cookie requests to the cached preview
// content handler and everyone else to the main handler.
type previewPageDispatch struct {
	main  http.Handler
	state *state
}

func (d *previewPageDispatch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if theme := d.state.previewTheme(r); theme != nil {
		if entry, ok := d.state.previewEntryFor(theme); ok {
			entry.handler.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
			return
		}
	}
	d.main.ServeHTTP(w, r)
}

// previewStaticDispatch routes preview-cookie requests to the cached preview
// static files and everyone else to the main static handler.
type previewStaticDispatch struct {
	main  *sitessr.StaticFilesHandler
	state *state
}

func (d *previewStaticDispatch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if theme := d.state.previewTheme(r); theme != nil {
		if entry, ok := d.state.previewEntryFor(theme); ok {
			entry.static.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
			return
		}
	}
	d.main.ServeHTTP(w, r)
}
