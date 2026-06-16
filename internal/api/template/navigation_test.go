package template_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderIndex_NavigationItems(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			NavigationItems: []template.NavigationItem{
				{Title: "Home", URL: "/", IsActive: true},
				{Title: "About", URL: "/about", IsActive: false},
			},
		},
		Posts: []template.PostItem{},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, `href="/"`)
	assert.Contains(t, body, `href="/about"`)
	assert.Contains(t, body, "About")
}

func TestRenderIndex_NavigationItems_ActiveClass(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:     "Test Site",
			PageTitle: "Test Site",
			NavigationItems: []template.NavigationItem{
				{Title: "Home", URL: "/", IsActive: true},
				{Title: "About", URL: "/about", IsActive: false},
			},
		},
		Posts: []template.PostItem{},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, `class="active"`)
}

func TestRenderIndex_NavigationItems_EmptyNav(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.IndexData{
		LayoutData: template.LayoutData{
			Title:           "Test Site",
			PageTitle:       "Test Site",
			NavigationItems: []template.NavigationItem{},
		},
		Posts: []template.PostItem{},
	}

	renderErr := templates.RenderIndex(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, `<nav class="site-nav">`)
	assert.Contains(t, body, `</nav>`)
}

func TestRenderContent_ActiveNav(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.ContentData{
		LayoutData: template.LayoutData{
			Title:     "About",
			PageTitle: "About - Test Site",
			NavigationItems: []template.NavigationItem{
				{Title: "Home", URL: "/", IsActive: false},
				{Title: "About", URL: "/about", IsActive: true},
			},
		},
		Body:  "<p>About us</p>",
		Tags:  []string{},
	}

	renderErr := templates.RenderContent(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, `href="/"`)
	assert.Contains(t, body, `href="/about"`)
	assert.Contains(t, body, `class="active"`)
	assert.Contains(t, body, "About us")
}

func TestRenderNotFound_NavigationItems(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.NotFoundData{
		LayoutData: template.LayoutData{
			Title:     "Not Found",
			PageTitle: "Not Found - Test Site",
			NavigationItems: []template.NavigationItem{
				{Title: "Home", URL: "/", IsActive: false},
			},
		},
	}

	renderErr := templates.RenderNotFound(w, data)
	require.NoError(t, renderErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `href="/"`)
}

func TestRenderAuthor_NavigationItems(t *testing.T) {
	templates, err := template.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	data := template.AuthorData{
		LayoutData: template.LayoutData{
			Title:     "Admin",
			PageTitle: "Admin - Test Site",
			NavigationItems: []template.NavigationItem{
				{Title: "Home", URL: "/", IsActive: false},
			},
		},
		AuthorName: "Admin",
		Posts:      []template.PostItem{},
	}

	renderErr := templates.RenderAuthor(w, data)
	require.NoError(t, renderErr)

	body := w.Body.String()
	assert.Contains(t, body, `href="/"`)
	assert.Contains(t, body, "Admin")
}
