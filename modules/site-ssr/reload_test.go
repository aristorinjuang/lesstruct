package sitessr_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeThemeTemplate drops a template override into dir/templates.
func writeThemeTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", name), []byte(content), 0o644))
}

// renderNotFound renders the not-found page to a string through templates.
func renderNotFound(t *testing.T, templates *sitessr.Templates) string {
	t.Helper()
	body, err := templates.RenderNotFoundToString(sitessr.NotFoundData{})
	require.NoError(t, err)
	return body
}

func TestTemplatesReloadFromDisk(t *testing.T) {
	const override404 = `{{define "body"}}<div class="custom-404"><h1>Gone Reloading!</h1></div>{{end}}`

	tests := []struct {
		name       string
		setupTheme func(t *testing.T) *sitessr.Theme
		reloadWith func(t *testing.T) *sitessr.Theme
		wantIn     string
		wantErr    bool
	}{
		{
			name: "success - reload applies theme override",
			setupTheme: func(t *testing.T) *sitessr.Theme {
				return nil
			},
			reloadWith: func(t *testing.T) *sitessr.Theme {
				dir := t.TempDir()
				writeThemeTemplate(t, dir, "not_found.html", override404)
				return &sitessr.Theme{Dir: dir}
			},
			wantIn:  "Gone Reloading!",
			wantErr: false,
		},
		{
			name: "success - reload back to embedded clears override",
			setupTheme: func(t *testing.T) *sitessr.Theme {
				dir := t.TempDir()
				writeThemeTemplate(t, dir, "not_found.html", override404)
				return &sitessr.Theme{Dir: dir}
			},
			reloadWith: func(t *testing.T) *sitessr.Theme {
				return nil
			},
			wantIn:  "ui.page_not_found",
			wantErr: false,
		},
		{
			name: "error - broken template keeps old set",
			setupTheme: func(t *testing.T) *sitessr.Theme {
				return nil
			},
			reloadWith: func(t *testing.T) *sitessr.Theme {
				dir := t.TempDir()
				writeThemeTemplate(t, dir, "not_found.html", `{{define "body"}}<div>unclosed`)
				return &sitessr.Theme{Dir: dir}
			},
			wantIn:  "ui.page_not_found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templates, err := sitessr.NewTemplates(tt.setupTheme(t), nil)
			require.NoError(t, err)
			reloadErr := templates.ReloadFromDisk(tt.reloadWith(t))
			if tt.wantErr {
				require.Error(t, reloadErr)
			} else {
				require.NoError(t, reloadErr)
			}
			assert.Contains(t, renderNotFound(t, templates), tt.wantIn)
		})
	}
}

func TestTemplatesReloadConcurrent(t *testing.T) {
	dir := t.TempDir()
	writeThemeTemplate(t, dir, "not_found.html", `{{define "body"}}<div class="custom-404"><h1>Gone Reloading!</h1></div>{{end}}`)
	theme := &sitessr.Theme{Dir: dir}

	templates, err := sitessr.NewTemplates(nil, nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for range 50 {
				if i%2 == 0 {
					_, _ = templates.RenderNotFoundToString(sitessr.NotFoundData{})
				} else {
					_ = templates.ReloadFromDisk(theme)
				}
			}
		}(i)
	}
	wg.Wait()

	// The set is usable after the storm: reload to a known state and render.
	require.NoError(t, templates.ReloadFromDisk(nil))
	assert.Contains(t, renderNotFound(t, templates), "ui.page_not_found")
}

func TestStaticFilesHandlerReload(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *sitessr.Theme
		target  string
		wantIn  string
		wantErr bool
	}{
		{
			name: "success - reload serves new static file",
			setup: func(t *testing.T) *sitessr.Theme {
				return nil
			},
			target:  "/base.css",
			wantIn:  "reloaded-theme-marker",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := sitessr.StaticFiles(tt.setup(t))
			dir := t.TempDir()
			staticDir := filepath.Join(dir, "static")
			require.NoError(t, os.MkdirAll(staticDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(staticDir, "base.css"),
				[]byte("/* reloaded-theme-marker */:root{}"),
				0o644,
			))
			handler.Reload(&sitessr.Theme{Dir: dir})

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if tt.wantErr {
				assert.NotEqual(t, http.StatusOK, w.Code)
				return
			}
			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantIn)
		})
	}
}

func TestThemeRootHandlerReload(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	tests := []struct {
		name     string
		wantCode int
		wantBody string
		wantErr  bool
	}{
		{
			name:     "success - reload starts serving new root file",
			wantCode: http.StatusOK,
			wantBody: "console.log('reloaded')",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			theme := &sitessr.Theme{Dir: dir}
			handler := sitessr.RootFilesHandler(theme, nextHandler)

			// Before the root file exists the request falls through.
			req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			require.Equal(t, http.StatusTeapot, w.Code)

			require.NoError(t, os.MkdirAll(filepath.Join(dir, "root"), 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(dir, "root", "sw.js"),
				[]byte("console.log('reloaded')"),
				0o644,
			))
			handler.Reload(theme)

			w = httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if tt.wantErr {
				assert.NotEqual(t, http.StatusOK, w.Code)
				return
			}
			assert.Equal(t, tt.wantCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}
