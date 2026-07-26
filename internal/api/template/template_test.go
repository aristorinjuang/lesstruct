package template_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplates(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, templates)
}

func TestRenderIndex(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			Lang:      "en",
		},
		Posts: []template.PostItem{
			{
				Slug:  "hello-world",
				Title: "Hello World",
			},
		},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "<html")
	assert.Contains(t, body, "Hello World")
	assert.Contains(t, body, `href="/hello-world"`)
	assert.Regexp(t, `href="/static/style\.[a-f0-9]+\.css"`, body)
}

func TestRenderIndex_PostItemTagsAccepted(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			Lang:      "en",
		},
		Posts: []template.PostItem{
			{
				Slug:  "hello-world",
				Title: "Hello World",
				Tags:  []string{"go", "tutorial"},
			},
		},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "Hello World")
	assert.Contains(t, body, `href="/hello-world"`)
}

func TestRenderIndex_Empty(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Empty Site",
			PageTitle: "Empty Site",
			Lang:      "en",
		},
		Posts: []template.PostItem{},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "ui.no_posts")
}

func TestRenderContent(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.ContentData{
		LayoutData: template.LayoutData{
			Title:     "My Post",
			PageTitle: "My Post - Test Site",
			OGTitle:   "My Post",
			OGDesc:    "A test post",
			Lang:      "en",
		},
		Slug:      "my-post",
		Body:      "<p>Hello <strong>world</strong></p>",
		Author:    "Admin",
		Username:  "admin",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Tags:      []string{"go", "test"},
	}

	renderErr := templates.RenderContent(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "My Post")
	assert.Contains(t, body, "<p>Hello <strong>world</strong></p>")
	assert.Contains(t, body, "go")
	assert.Contains(t, body, "test")
	assert.Contains(t, body, `href="/authors/admin"`)
	assert.Contains(t, body, "ui.back_to_home")
}

func TestRenderAuthor(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.AuthorData{
		LayoutData: template.LayoutData{
			Title:     "Admin",
			PageTitle: "Admin - Test Site",
			Lang:      "en",
		},
		AuthorName: "Admin",
		Username:   "admin",
		Posts: []template.PostItem{
			{Slug: "post-1", Title: "Post One"},
			{Slug: "post-2", Title: "Post Two"},
		},
	}

	renderErr := templates.RenderAuthor(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "Admin")
	assert.Contains(t, body, "Post One")
	assert.Contains(t, body, "Post Two")
}

func TestRenderNotFound(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.NotFoundData{
		LayoutData: template.LayoutData{
			Title:     "Not Found",
			PageTitle: "Not Found - Test Site",
			Lang:      "en",
		},
	}

	renderErr := templates.RenderNotFound(w, data)
	require.NoError(t, renderErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "ui.not_found_404")
	assert.Contains(t, w.Body.String(), "ui.page_not_found")
}

func TestStaticFiles(t *testing.T) {
	handler := template.StaticFiles(nil)
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/css")
}

func TestRenderVerifyEmail(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.VerifyEmailData{
		LayoutData: template.LayoutData{
			Title:     "Verify Email",
			PageTitle: "Verify Email - Lesstruct",
			Lang:      "en",
		},
	}

	renderErr := templates.RenderVerifyEmail(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "ui.verify_email_title")
	assert.Contains(t, body, "auth-error")
	assert.Contains(t, body, "auth-success")
	assert.Contains(t, body, "/static/verify-email.js")
	assert.Contains(t, body, `href="/login"`)
	assert.Contains(t, body, `style="display:none"`)
}

func TestRenderResetPassword(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.ResetPasswordData{
		LayoutData: template.LayoutData{
			Title:     "Reset Password",
			PageTitle: "Reset Password - Lesstruct",
			Lang:      "en",
		},
	}

	renderErr := templates.RenderResetPassword(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, "ui.reset_password")
	assert.Contains(t, body, "ui.new_password")
	assert.Contains(t, body, "auth-error")
	assert.Contains(t, body, "auth-success")
	assert.Contains(t, body, "reset-form")
	assert.Contains(t, body, "new-password")
	assert.Contains(t, body, `autocomplete="new-password"`)
	assert.Contains(t, body, "/static/reset-password.js")
	assert.Contains(t, body, `href="/login"`)
	assert.Contains(t, body, `style="display:none"`)
}

func TestAssetURL_VersionedCSS(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			Lang:      "en",
		},
	}
	require.NoError(t, templates.RenderIndex(w, data))

	body := w.Body.String()
	assert.Regexp(t, `href="/static/style\.[a-f0-9]+\.css"`, body)
}

func TestRenderContent_PostTypeDispatch(t *testing.T) {
	tests := []struct {
		name         string
		postType     string
		postTypes    []string
		setupTheme   func(t *testing.T) *template.Theme
		wantInOutput string
	}{
		{
			name:         "empty post type falls back to default",
			postType:     "",
			postTypes:    []string{"post"},
			setupTheme:   func(t *testing.T) *template.Theme { return nil },
			wantInOutput: "content-article",
		},
		{
			name:         "unknown post type falls back to default",
			postType:     "menu-item",
			postTypes:    []string{"post", "page"},
			setupTheme:   func(t *testing.T) *template.Theme { return nil },
			wantInOutput: "content-article",
		},
		{
			name:     "registered post type uses default when no theme override",
			postType: "page",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme { return nil },
			wantInOutput: "content-article",
		},
		{
			name:     "theme page.html overrides page post type",
			postType: "page",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				pageHTML := `{{define "body"}}<div class="page-only">PAGE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte(pageHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "PAGE TEMPLATE",
		},
		{
			name:     "theme page.html does not affect post rendering",
			postType: "post",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				pageHTML := `{{define "body"}}<div class="page-only">PAGE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte(pageHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "content-article",
		},
		{
			name:     "theme post.html overrides all types without specific override",
			postType: "menu-item",
			postTypes: []string{"post", "page", "menu-item"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				postHTML := `{{define "body"}}<div class="custom-post">CUSTOM POST</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "post.html"), []byte(postHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "CUSTOM POST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := tt.setupTheme(t)
			templates, err := template.NewTemplates(nil, nil, tt.postTypes...)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			data := template.ContentData{
				LayoutData: template.LayoutData{
					Title:     "Test",
					PageTitle: "Test",
					Lang:      "en",
				},
				Slug:     "test",
				Body:     "<p>Body</p>",
				PostType: tt.postType,
			}

			if theme != nil {
				templates, err = template.NewTemplates(theme, nil, tt.postTypes...)
				require.NoError(t, err)
			}

			require.NoError(t, templates.RenderContent(w, data))
			assert.Contains(t, w.Body.String(), tt.wantInOutput)
		})
	}
}

func TestRenderContent_PerSlugOverride(t *testing.T) {
	tests := []struct {
		name         string
		postType     string
		slug         string
		postTypes    []string
		setupTheme   func(t *testing.T) *template.Theme
		wantInOutput string
	}{
		{
			name:      "per-slug template applies when post type and slug match",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div class="page-about">ABOUT SLUG TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page-about.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "ABOUT SLUG TEMPLATE",
		},
		{
			name:      "per-slug template does not apply when slug differs",
			postType:  "page",
			slug:      "contact",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div class="page-about">ABOUT SLUG TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page-about.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "content-article",
		},
		{
			name:      "per-slug template does not apply when post type differs",
			postType:  "post",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div class="page-about">ABOUT SLUG TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page-about.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "content-article",
		},
		{
			name:      "falls through to per-post-type template when per-slug file missing",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				pageHTML := `{{define "body"}}<div class="page-only">PAGE POST-TYPE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte(pageHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "PAGE POST-TYPE TEMPLATE",
		},
		{
			name:      "falls through to default when neither per-slug nor per-post-type present",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "content-article",
		},
		{
			name:      "per-slug template with hyphenated slug (page-about-us.html)",
			postType:  "page",
			slug:      "about-us",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div class="page-about-us">HYHENATED SLUG TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page-about-us.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "HYHENATED SLUG TEMPLATE",
		},
		{
			name:      "per-slug template with hyphenated post type (menu-item-special.html)",
			postType:  "menu-item",
			slug:      "special",
			postTypes: []string{"post", "page", "menu-item"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div class="menu-item-special">HYPHENATED TYPE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "menu-item-special.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "HYPHENATED TYPE TEMPLATE",
		},
		{
			name:      "longest post-type prefix wins when both menu and menu-item are registered",
			postType:  "menu-item",
			slug:      "special",
			postTypes: []string{"post", "page", "menu", "menu-item"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				shortMatch := `{{define "body"}}<div>SHORT MATCH (menu, item-special)</div>{{end}}`
				longMatch := `{{define "body"}}<div>LONG MATCH (menu-item, special)</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "menu-item-special.html"), []byte(longMatch), 0644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "menu-item.html"), []byte(shortMatch), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "LONG MATCH (menu-item, special)",
		},
		{
			name:      "per-post-type template file is not mistaken for per-slug override",
			postType:  "page",
			slug:      "page",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				pageHTML := `{{define "body"}}<div class="page-post-type">PAGE POST-TYPE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte(pageHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "PAGE POST-TYPE TEMPLATE",
		},
		{
			name:      "unknown post-type prefix is ignored (random-foo.html)",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				content := `{{define "body"}}<div>UNKNOWN TYPE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "random-foo.html"), []byte(content), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "content-article",
		},
		{
			name:      "nil theme skips per-slug scan entirely",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				return nil
			},
			wantInOutput: "content-article",
		},
		{
			name:      "empty theme dir skips per-slug scan entirely",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				return &template.Theme{Dir: ""}
			},
			wantInOutput: "content-article",
		},
		{
			name:      "per-slug takes precedence over per-post-type when both exist",
			postType:  "page",
			slug:      "about",
			postTypes: []string{"post", "page"},
			setupTheme: func(t *testing.T) *template.Theme {
				dir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
				pageHTML := `{{define "body"}}<div>PAGE POST-TYPE TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte(pageHTML), 0644))
				slugHTML := `{{define "body"}}<div>PAGE-SPECIFIC SLUG TEMPLATE</div>{{end}}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "page-about.html"), []byte(slugHTML), 0644))
				return &template.Theme{Dir: dir}
			},
			wantInOutput: "PAGE-SPECIFIC SLUG TEMPLATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := tt.setupTheme(t)
			templates, err := template.NewTemplates(theme, nil, tt.postTypes...)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			data := template.ContentData{
				LayoutData: template.LayoutData{
					Title:     "Test",
					PageTitle: "Test",
					Lang:      "en",
				},
				Slug:     tt.slug,
				Body:     "<p>Body</p>",
				PostType: tt.postType,
			}

			require.NoError(t, templates.RenderContent(w, data))
			assert.Contains(t, w.Body.String(), tt.wantInOutput)
		})
	}
}
