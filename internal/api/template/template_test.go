package template_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	assert.Contains(t, body, `<link rel="stylesheet" href="/static/style.css">`)
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
		CreatedAt: "2025-01-01",
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
