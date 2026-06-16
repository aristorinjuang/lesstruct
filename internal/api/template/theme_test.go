package template_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompositeFS(t *testing.T) {
	tests := []struct {
		name      string
		primary   fs.FS
		secondary fs.FS
		fileName  string
		wantData  string
		wantErr   bool
	}{
		{
			name: "file exists in primary",
			primary: fstest.MapFS{
				"style.css": &fstest.MapFile{Data: []byte("primary-css")},
			},
			secondary: fstest.MapFS{
				"style.css": &fstest.MapFile{Data: []byte("secondary-css")},
			},
			fileName: "style.css",
			wantData: "primary-css",
			wantErr:  false,
		},
		{
			name:    "file missing in primary falls back to secondary",
			primary: fstest.MapFS{},
			secondary: fstest.MapFS{
				"style.css": &fstest.MapFile{Data: []byte("secondary-css")},
			},
			fileName: "style.css",
			wantData: "secondary-css",
			wantErr:  false,
		},
		{
			name:      "file missing in both returns error",
			primary:   fstest.MapFS{},
			secondary: fstest.MapFS{},
			fileName:  "missing.css",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfs := template.NewCompositeFSForTest(tt.primary, tt.secondary)

			f, err := cfs.Open(tt.fileName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { _ = f.Close() }()

			data, err := fs.ReadFile(cfs, tt.fileName)
			require.NoError(t, err)
			assert.Equal(t, tt.wantData, string(data))
		})
	}
}

func TestResolveFS(t *testing.T) {
	embedded := fstest.MapFS{
		"static/style.css": &fstest.MapFile{Data: []byte("embedded-css")},
		"static/app.js":    &fstest.MapFile{Data: []byte("embedded-js")},
	}

	tests := []struct {
		name     string
		theme    *template.Theme
		embedded fs.FS
		subPath  string
		fileName string
		wantData string
		wantErr  bool
	}{
		{
			name:     "nil theme returns embedded",
			theme:    nil,
			embedded: embedded,
			subPath:  "static",
			fileName: "style.css",
			wantData: "embedded-css",
			wantErr:  false,
		},
		{
			name:     "empty theme dir returns embedded",
			theme:    &template.Theme{Dir: ""},
			embedded: embedded,
			subPath:  "static",
			fileName: "style.css",
			wantData: "embedded-css",
			wantErr:  false,
		},
		{
			name:     "nil theme no subpath returns full embedded",
			theme:    nil,
			embedded: embedded,
			subPath:  "",
			fileName: "static/style.css",
			wantData: "embedded-css",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := template.ResolveFSForTest(tt.theme, tt.embedded, tt.subPath)

			data, err := fs.ReadFile(resolved, tt.fileName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, string(data))
		})
	}
}

func TestNewTemplates_WithTheme(t *testing.T) {
	tests := []struct {
		name         string
		setupTheme   func(t *testing.T) *template.Theme
		renderFunc   func(t *testing.T, templates *template.Templates) string
		wantInOutput string
		wantMissing  string
	}{
		{
			name: "nil theme uses embedded defaults",
			setupTheme: func(t *testing.T) *template.Theme {
				return nil
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.IndexData{
					LayoutData: template.LayoutData{
						Title:     "Test Site",
						PageTitle: "Test Site",
					},
					Posts: []template.PostItem{},
				}
				require.NoError(t, templates.RenderIndex(w, data))
				return w.Body.String()
			},
			wantInOutput: "Lesstruct",
		},
		{
			name: "empty theme dir uses embedded defaults",
			setupTheme: func(t *testing.T) *template.Theme {
				return &template.Theme{Dir: ""}
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.IndexData{
					LayoutData: template.LayoutData{
						Title:     "Test Site",
						PageTitle: "Test Site",
					},
					Posts: []template.PostItem{},
				}
				require.NoError(t, templates.RenderIndex(w, data))
				return w.Body.String()
			},
			wantInOutput: "Lesstruct",
		},
		{
			name: "theme with custom layout overrides logo",
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				templatesDir := filepath.Join(dir, "templates")
				require.NoError(t, os.MkdirAll(templatesDir, 0755))

				customLayout := `{{define "layout"}}<!DOCTYPE html>
<html lang="en">
<head><title>{{.PageTitle}}</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header class="site-header"><div class="container">
<a href="/" class="site-logo">CustomTheme</a>
</div></header>
<main class="container">{{template "body" .}}</main>
</body>
</html>{{end}}`
				require.NoError(t, os.WriteFile(
					filepath.Join(templatesDir, "layout.html"),
					[]byte(customLayout),
					0644,
				))

				return &template.Theme{Dir: dir}
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.IndexData{
					LayoutData: template.LayoutData{
						Title:     "Test Site",
						PageTitle: "Test Site",
					},
					Posts: []template.PostItem{},
				}
				require.NoError(t, templates.RenderIndex(w, data))
				return w.Body.String()
			},
			wantInOutput: "CustomTheme",
			wantMissing:  "Lesstruct",
		},
		{
			name: "theme with only layout overrides keeps default page templates",
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				templatesDir := filepath.Join(dir, "templates")
				require.NoError(t, os.MkdirAll(templatesDir, 0755))

				customLayout := `{{define "layout"}}<!DOCTYPE html>
<html lang="en">
<head><title>{{.PageTitle}}</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header class="site-header"><div class="container">
<a href="/" class="site-logo">PartialTheme</a>
</div></header>
<main class="container">{{template "body" .}}</main>
</body>
</html>{{end}}`
				require.NoError(t, os.WriteFile(
					filepath.Join(templatesDir, "layout.html"),
					[]byte(customLayout),
					0644,
				))

				return &template.Theme{Dir: dir}
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.IndexData{
					LayoutData: template.LayoutData{
						Title:     "Test Site",
						PageTitle: "Test Site",
					},
					Posts: []template.PostItem{},
				}
				require.NoError(t, templates.RenderIndex(w, data))
				return w.Body.String()
			},
			wantInOutput: "ui.no_posts",
		},
		{
			name: "theme with custom index overrides page content",
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				templatesDir := filepath.Join(dir, "templates")
				require.NoError(t, os.MkdirAll(templatesDir, 0755))

				customIndex := `{{define "body"}}
<div class="custom-index">
<h2>Custom Index Page</h2>
{{range .Posts}}<p>{{.Title}}</p>{{end}}
</div>
{{end}}`
				require.NoError(t, os.WriteFile(
					filepath.Join(templatesDir, "index.html"),
					[]byte(customIndex),
					0644,
				))

				return &template.Theme{Dir: dir}
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.IndexData{
					LayoutData: template.LayoutData{
						Title:     "Test Site",
						PageTitle: "Test Site",
					},
					Posts: []template.PostItem{},
				}
				require.NoError(t, templates.RenderIndex(w, data))
				return w.Body.String()
			},
			wantInOutput: "Custom Index Page",
		},
		{
			name: "theme with custom 404 overrides not found page",
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				templatesDir := filepath.Join(dir, "templates")
				require.NoError(t, os.MkdirAll(templatesDir, 0755))

				custom404 := `{{define "body"}}
<div class="custom-404">
<h1>Lost in Space!</h1>
<p>The page you are looking for has drifted away.</p>
</div>
{{end}}`
				require.NoError(t, os.WriteFile(
					filepath.Join(templatesDir, "not_found.html"),
					[]byte(custom404),
					0644,
				))

				return &template.Theme{Dir: dir}
			},
			renderFunc: func(t *testing.T, templates *template.Templates) string {
				w := httptest.NewRecorder()
				data := template.NotFoundData{
					LayoutData: template.LayoutData{
						Title:     "Not Found",
						PageTitle: "Not Found",
					},
				}
				require.NoError(t, templates.RenderNotFound(w, data))
				return w.Body.String()
			},
			wantInOutput: "Lost in Space!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := tt.setupTheme(t)
			templates, err := template.NewTemplates(theme, nil)
			require.NoError(t, err)
			require.NotNil(t, templates)

			body := tt.renderFunc(t, templates)
			assert.Contains(t, body, tt.wantInOutput)
			if tt.wantMissing != "" {
				assert.NotContains(t, body, tt.wantMissing)
			}
		})
	}
}

func TestNewTemplates_WithTheme_RenderNotFound(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))

	custom404 := `{{define "body"}}
<div class="custom-404">
<h1>Gone Exploring!</h1>
<p>This page is off the map.</p>
</div>
{{end}}`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "not_found.html"),
		[]byte(custom404),
		0644,
	))

	theme := &template.Theme{Dir: dir}
	templates, err := template.NewTemplates(theme, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.NotFoundData{
		LayoutData: template.LayoutData{
			Title:     "Not Found",
			PageTitle: "Not Found",
		},
	}

	renderErr := templates.RenderNotFound(w, data)
	require.NoError(t, renderErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Gone Exploring!")
	assert.NotContains(t, w.Body.String(), "Page not found.")
}
