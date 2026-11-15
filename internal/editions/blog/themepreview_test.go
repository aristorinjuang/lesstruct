package blog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/config"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themenapi "github.com/aristorinjuang/lesstruct/modules/themegen/api"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installPreviewTheme drops a theme with the given CSS marker and
// not-found override into the themes root.
func installPreviewTheme(t *testing.T, themesRoot, name, cssMarker string) {
	t.Helper()
	staticDir := filepath.Join(themesRoot, name, "static")
	require.NoError(t, os.MkdirAll(staticDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(staticDir, "base.css"),
		[]byte("/* "+cssMarker+" */:root{}"),
		0o644,
	))
	templatesDir := filepath.Join(themesRoot, name, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "not_found.html"),
		[]byte(`{{define "body"}}<div class="custom-404"><h1>Preview marker</h1></div>{{end}}`),
		0o644,
	))
}

// previewTestState builds a state with a stub preview builder rendering the
// theme's templates, backed by the embedded default theme.
func previewTestState(t *testing.T, cfg *config.Config, builderErr error) (*state, *int) {
	t.Helper()
	sitessr.SetEmbeddedFS(themedefault.Files())
	t.Cleanup(func() { sitessr.SetEmbeddedFS(nil) })

	builds := 0
	s := &state{
		cfg:        cfg,
		previewKey: themenapi.DerivePreviewKey("test-jwt-secret"),
		previews:   newPreviewCache(),
		previewBuilder: func(theme *sitessr.Theme) (http.Handler, *sitessr.StaticFilesHandler, error) {
			builds++
			if builderErr != nil {
				return nil, nil, builderErr
			}
			templates, err := sitessr.NewTemplates(theme, nil)
			if err != nil {
				return nil, nil, err
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				body, err := templates.RenderNotFoundToString(sitessr.NotFoundData{})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(body))
			})
			return handler, sitessr.StaticFiles(theme), nil
		},
	}
	return s, &builds
}

func previewCookie(t *testing.T, s *state, name string) *http.Cookie {
	t.Helper()
	value, _ := themenapi.SignPreviewCookie(name, s.previewKey)
	return &http.Cookie{Name: themenapi.PreviewCookieName, Value: value}
}

// dispatchPage runs one request through the preview page dispatch.
func dispatchPage(s *state, main http.Handler, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	(&previewPageDispatch{main: main, state: s}).ServeHTTP(w, req)
	return w
}

func TestPreviewPageDispatch(t *testing.T) {
	main := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("main marker"))
	})

	tests := []struct {
		name        string
		cookie      func(t *testing.T, s *state) *http.Cookie
		themeDir    string
		builderErr  bool
		wantBody    string
		wantNoStore bool
	}{
		{
			name: "success - valid cookie renders preview without store",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				return previewCookie(t, s, "preview-theme")
			},
			wantBody:    "Preview marker",
			wantNoStore: true,
		},
		{
			name: "success - missing cookie renders main",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				return nil
			},
			wantBody:    "main marker",
			wantNoStore: false,
		},
		{
			name: "success - tampered cookie renders main",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				c := previewCookie(t, s, "preview-theme")
				c.Value += "tampered"
				return c
			},
			wantBody:    "main marker",
			wantNoStore: false,
		},
		{
			name: "success - wrong key renders main",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				value, _ := themenapi.SignPreviewCookie("preview-theme", themenapi.DerivePreviewKey("other-secret"))
				return &http.Cookie{Name: themenapi.PreviewCookieName, Value: value}
			},
			wantBody:    "main marker",
			wantNoStore: false,
		},
		{
			name: "success - builder failure falls back to main",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				return previewCookie(t, s, "preview-theme")
			},
			builderErr:  true,
			wantBody:    "main marker",
			wantNoStore: false,
		},
		{
			name: "success - theme dir wins over preview",
			cookie: func(t *testing.T, s *state) *http.Cookie {
				t.Helper()
				return previewCookie(t, s, "preview-theme")
			},
			themeDir:    t.TempDir(),
			wantBody:    "main marker",
			wantNoStore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			installPreviewTheme(t, themesRoot, "preview-theme", "preview-css-marker")
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemeDir:   tt.themeDir,
				ThemesDir:  themesRoot,
			}
			var builderErr error
			if tt.builderErr {
				builderErr = assert.AnError
			}
			s, _ := previewTestState(t, cfg, builderErr)

			w := dispatchPage(s, main, "/missing", tt.cookie(t, s))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
			if tt.wantNoStore {
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			} else {
				assert.NotEqual(t, "no-store", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestPreviewStaticDispatch(t *testing.T) {
	tests := []struct {
		name        string
		withCookie  bool
		wantBody    string
		wantNoStore bool
	}{
		{
			name:        "success - valid cookie serves preview static without store",
			withCookie:  true,
			wantBody:    "preview-css-marker",
			wantNoStore: true,
		},
		{
			name:        "success - missing cookie serves main static",
			withCookie:  false,
			wantBody:    "main static marker",
			wantNoStore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			installPreviewTheme(t, themesRoot, "preview-theme", "preview-css-marker")
			mainDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(mainDir, "static"), 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(mainDir, "static", "base.css"),
				[]byte("/* main static marker */:root{}"),
				0o644,
			))
			main := sitessr.StaticFiles(&sitessr.Theme{Dir: mainDir})
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemesDir:  themesRoot,
			}
			s, _ := previewTestState(t, cfg, nil)

			req := httptest.NewRequest(http.MethodGet, "/base.css", nil)
			if tt.withCookie {
				req.AddCookie(previewCookie(t, s, "preview-theme"))
			}
			w := httptest.NewRecorder()
			(&previewStaticDispatch{main: main, state: s}).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
			if tt.wantNoStore {
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			} else {
				assert.NotEqual(t, "no-store", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestPreviewEntryCaching(t *testing.T) {
	tests := []struct {
		name       string
		sequence   []string
		touch      map[string]bool
		wantBuilds int
	}{
		{
			name:       "success - reuses cached entry",
			sequence:   []string{"theme-a", "theme-a", "theme-b", "theme-a"},
			wantBuilds: 2,
		},
		{
			name:       "success - rebuilds after theme change",
			sequence:   []string{"theme-a", "theme-a"},
			touch:      map[string]bool{"theme-a": true},
			wantBuilds: 2,
		},
		{
			name:       "success - evicts oldest beyond cap",
			sequence:   []string{"theme-a", "theme-b", "theme-c", "theme-d", "theme-e", "theme-a"},
			wantBuilds: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			for _, name := range []string{"theme-a", "theme-b", "theme-c", "theme-d", "theme-e"} {
				installPreviewTheme(t, themesRoot, name, name+"-css-marker")
			}
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemesDir:  themesRoot,
			}
			s, builds := previewTestState(t, cfg, nil)

			touched := map[string]bool{}
			for _, name := range tt.sequence {
				theme := &sitessr.Theme{Dir: filepath.Join(themesRoot, name)}
				_, ok := s.previewEntryFor(theme)
				require.True(t, ok)
				if tt.touch[name] && !touched[name] {
					touched[name] = true
					themeDir := filepath.Join(themesRoot, name)
					future := time.Now().Add(2 * time.Hour)
					require.NoError(t, os.Chtimes(themeDir, future, future))
				}
			}
			assert.Equal(t, tt.wantBuilds, *builds)
		})
	}
}

func TestPreviewDispatchConcurrent(t *testing.T) {
	dir := t.TempDir()
	themesRoot := filepath.Join(dir, "themes")
	installPreviewTheme(t, themesRoot, "preview-theme", "preview-css-marker")
	cfg := &config.Config{
		ConfigDir:  dir,
		ConfigFile: "config.toml",
		ThemesDir:  themesRoot,
	}
	s, _ := previewTestState(t, cfg, nil)
	main := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("main marker"))
	})

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for range 25 {
				if i%2 == 0 {
					w := dispatchPage(s, main, "/missing", previewCookie(t, s, "preview-theme"))
					_ = w.Code
				} else {
					w := dispatchPage(s, main, "/missing", nil)
					_ = w.Code
				}
			}
		}(i)
	}
	wg.Wait()
}
