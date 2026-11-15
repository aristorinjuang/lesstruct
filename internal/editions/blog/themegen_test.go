package blog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installTestThemeRegistry installs the embedded default theme artifacts the
// SSR stack reads from and restores the process-wide registry afterwards.
func installTestThemeRegistry(t *testing.T) {
	t.Helper()
	sitessr.SetEmbeddedFS(themedefault.Files())
	sitessr.SetThemeFuncs(themedefault.New().ThemeFuncs())
	sitessr.SetThemePages(nil)
	t.Cleanup(func() {
		sitessr.SetEmbeddedFS(nil)
		sitessr.SetThemeFuncs(nil)
		sitessr.SetThemePages(nil)
	})
}

func TestResolveActiveTheme(t *testing.T) {
	tests := []struct {
		name       string
		themeDir   string
		active     string
		setupDir   func(t *testing.T, themesRoot string)
		wantNil    bool
		wantFailed bool
		wantErr    bool
	}{
		{
			name:       "success - embedded default",
			themeDir:   "",
			active:     "",
			setupDir:   nil,
			wantNil:    true,
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "success - theme dir wins",
			themeDir:   "/tmp/manual",
			active:     "",
			setupDir:   nil,
			wantNil:    false,
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:    "success - valid installed theme",
			active:  "midnight",
			wantNil: false,
			wantErr: false,
			setupDir: func(t *testing.T, themesRoot string) {
				t.Helper()
				writeInstalledTheme(t, themesRoot, "midnight", "midnight-marker", "")
			},
		},
		{
			name:       "error - missing installed theme",
			active:     "missing",
			wantNil:    true,
			wantFailed: true,
			wantErr:    true,
		},
		{
			name:       "error - invalid active name",
			active:     "Default",
			wantNil:    true,
			wantFailed: true,
			wantErr:    true,
		},
		{
			name:       "error - broken stylesheet",
			active:     "broken-css",
			wantNil:    true,
			wantFailed: true,
			wantErr:    true,
			setupDir: func(t *testing.T, themesRoot string) {
				t.Helper()
				staticDir := filepath.Join(themesRoot, "broken-css", "static")
				require.NoError(t, os.MkdirAll(staticDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(staticDir, "base.css"), []byte(":root{"), 0o644))
			},
		},
		{
			name:       "error - broken template",
			active:     "broken-template",
			wantNil:    true,
			wantFailed: true,
			wantErr:    true,
			setupDir: func(t *testing.T, themesRoot string) {
				t.Helper()
				writeInstalledTheme(t, themesRoot, "broken-template", "broken-template-marker", `{{define "body"}}<div>unclosed`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installTestThemeRegistry(t)
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			require.NoError(t, os.MkdirAll(themesRoot, 0o755))
			if tt.setupDir != nil {
				tt.setupDir(t, themesRoot)
			}
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemeDir:   tt.themeDir,
				ThemesDir:  themesRoot,
			}
			themeCfg := "[theme]\nactive = \"" + tt.active + "\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(themeCfg), 0o644))
			theme, failed, err := resolveActiveTheme(cfg, newThemeValidator([]string{"post"}))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.wantNil {
				assert.Nil(t, theme)
			} else {
				assert.NotNil(t, theme)
			}
			assert.Equal(t, tt.wantFailed, failed)
		})
	}
}

func TestThemeActivationSaveClear(t *testing.T) {
	tests := []struct {
		name    string
		active  string
		wantErr bool
	}{
		{
			name:    "success - save and clear",
			active:  "midnight-serif",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			require.NoError(t, os.MkdirAll(filepath.Join(themesRoot, tt.active, "static"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(themesRoot, tt.active, "static", "base.css"), []byte(":root{}"), 0o644))
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemesDir:  themesRoot,
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(""), 0o644))
			store := NewThemeActivation(cfg)
			require.NoError(t, store.SaveActive(context.Background(), tt.active))
			assert.Equal(t, tt.active, store.ActiveTheme(context.Background()))
			require.NoError(t, store.ClearActive(context.Background()))
			assert.Equal(t, "", store.ActiveTheme(context.Background()))
		})
	}
}

// writeInstalledTheme drops a theme with the given CSS marker into the
// themes root. An empty template leaves the theme CSS-only; a non-empty one
// is written to templates/index.html.
func writeInstalledTheme(t *testing.T, themesRoot, name, cssMarker, template string) {
	t.Helper()
	staticDir := filepath.Join(themesRoot, name, "static")
	require.NoError(t, os.MkdirAll(staticDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(staticDir, "base.css"),
		[]byte("/* "+cssMarker+" */:root{}"),
		0o644,
	))
	if template != "" {
		templatesDir := filepath.Join(themesRoot, name, "templates")
		require.NoError(t, os.MkdirAll(templatesDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "index.html"), []byte(template), 0o644))
	}
}

// writeActiveThemeConfig points [theme] active at name.
func writeActiveThemeConfig(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.toml"),
		[]byte("[theme]\nactive = \""+name+"\"\n"),
		0o644,
	))
}

// serveStatic renders one static path through the handler.
func serveStatic(t *testing.T, handler http.Handler, target string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func TestStateReload(t *testing.T) {
	const goodMarker = "reload-good-marker"
	const badMarker = "reload-bad-marker"

	tests := []struct {
		name           string
		firstActive    string
		secondActive   string
		brokenTemplate bool
		wantErr        bool
		wantMarker     string
		wantRefreshed  string
	}{
		{
			name:          "success - reload applies new theme",
			firstActive:   "",
			secondActive:  "midnight",
			wantErr:       false,
			wantMarker:    goodMarker,
			wantRefreshed: goodMarker,
		},
		{
			name:          "success - reload back to embedded clears override",
			firstActive:   "midnight",
			secondActive:  "",
			wantErr:       false,
			wantMarker:    "",
			wantRefreshed: "",
		},
		{
			name:           "error - broken templates abort reload and keep old set",
			firstActive:    "midnight",
			secondActive:   "broken",
			brokenTemplate: true,
			wantErr:        true,
			wantMarker:     goodMarker,
			wantRefreshed:  goodMarker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installTestThemeRegistry(t)
			dir := t.TempDir()
			themesRoot := filepath.Join(dir, "themes")
			writeInstalledTheme(t, themesRoot, "midnight", goodMarker, "")
			if tt.brokenTemplate {
				writeInstalledTheme(t, themesRoot, "broken", badMarker, `{{define "body"}}<div>unclosed`)
			}
			cfg := &config.Config{
				ConfigDir:  dir,
				ConfigFile: "config.toml",
				ThemesDir:  themesRoot,
			}
			writeActiveThemeConfig(t, dir, tt.firstActive)

			templates, err := sitessr.NewTemplates(nil, nil)
			require.NoError(t, err)
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
			var refreshedBase string
			refreshed := false
			s := &state{
				cfg:         cfg,
				validator:   newThemeValidator([]string{"post"}),
				templates:   templates,
				staticFiles: sitessr.StaticFiles(nil),
				rootHandler: sitessr.RootFilesHandler(nil, next),
				textgenRefresher: func(baseCSS, _ string) {
					refreshed = true
					refreshedBase = baseCSS
				},
			}

			if tt.firstActive != "" {
				require.NoError(t, s.Reload(context.Background()))
			}
			writeActiveThemeConfig(t, dir, tt.secondActive)
			err = s.Reload(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, body := serveStatic(t, s.staticFiles, "/base.css")
			if tt.wantMarker == "" {
				assert.NotContains(t, body, goodMarker)
				assert.NotContains(t, body, badMarker)
			} else {
				assert.Contains(t, body, tt.wantMarker)
			}
			if tt.wantRefreshed == "" {
				if tt.firstActive == "" && tt.secondActive == "" {
					assert.False(t, refreshed)
				} else {
					assert.True(t, refreshed)
					assert.NotContains(t, refreshedBase, goodMarker)
					assert.NotContains(t, refreshedBase, badMarker)
				}
			} else {
				assert.True(t, refreshed)
				assert.Contains(t, refreshedBase, tt.wantRefreshed)
			}

			_, err = templates.RenderNotFoundToString(sitessr.NotFoundData{})
			require.NoError(t, err)
		})
	}
}

func TestStateReloadBeforeMount(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "error - reload before mount",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{}
			err := s.Reload(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
