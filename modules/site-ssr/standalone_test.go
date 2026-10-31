package sitessr_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	// fixtureLayout is the minimal shared layout every fixture page renders
	// through, mirroring the embedded artifact's block structure.
	fixtureLayout = `{{define "layout"}}<!DOCTYPE html>
<html lang="{{.Lang}}"><head>{{if .NoIndex}}<meta name="robots" content="noindex">{{end}}<link rel="stylesheet" href="{{assetURL "style.css"}}"></head><body>{{template "body" .}}</body></html>{{end}}`

	// fixtureBody is the placeholder fixed-page body used for every page the
	// test does not exercise.
	fixtureBody = `{{define "body"}}<main data-page="fixed">{{.PageTitle}}</main>{{end}}`

	// fixtureCartBody exercises both the standalone page data and a
	// theme-contributed func.
	fixtureCartBody = `{{define "body"}}<main data-page="{{.Page}}">{{shout "cart shell"}}</main>{{end}}`

	// fixtureTrackBody proves nested page keys render.
	fixtureTrackBody = `{{define "body"}}<main data-page="{{.Page}}">Track shell</main>{{end}}`
)

// installThemeFixture installs an embedded theme fixture plus the standalone
// pages the engine is expected to serve, restoring the package globals when
// the test finishes.
func installThemeFixture(t *testing.T) {
	t.Helper()

	files := fstest.MapFS{
		"pages/layout.gohtml":          {Data: []byte(fixtureLayout)},
		"pages/index.gohtml":           {Data: []byte(fixtureBody)},
		"pages/homepage.gohtml":        {Data: []byte(fixtureBody)},
		"pages/author.gohtml":          {Data: []byte(fixtureBody)},
		"pages/tag.gohtml":             {Data: []byte(fixtureBody)},
		"pages/not_found.gohtml":       {Data: []byte(fixtureBody)},
		"pages/login.gohtml":           {Data: []byte(fixtureBody)},
		"pages/register.gohtml":        {Data: []byte(fixtureBody)},
		"pages/forgot_password.gohtml": {Data: []byte(fixtureBody)},
		"pages/verify_email.gohtml":    {Data: []byte(fixtureBody)},
		"pages/reset_password.gohtml":  {Data: []byte(fixtureBody)},
		"pages/post.gohtml":            {Data: []byte(fixtureBody)},
		"pages/cart.gohtml":            {Data: []byte(fixtureCartBody)},
		"pages/orders-track.gohtml":    {Data: []byte(fixtureTrackBody)},
	}
	sitessr.SetEmbeddedFS(files)
	sitessr.SetThemeFuncs(map[string]any{"shout": strings.ToUpper})
	// "checkout" is declared without a template: the engine must skip it and
	// let the path fall through to content/404.
	sitessr.SetThemePages([]string{"cart", "checkout", "orders/track"})

	t.Cleanup(func() {
		sitessr.SetEmbeddedFS(themedefault.Files())
		sitessr.SetThemeFuncs(nil)
		sitessr.SetThemePages(nil)
	})
}

func TestNewTemplates_StandalonePages(t *testing.T) {
	installThemeFixture(t)

	templates, err := sitessr.NewTemplates(nil, nil)
	require.NoError(t, err)

	assert.True(t, templates.HasPage("cart"))
	assert.True(t, templates.HasPage("orders/track"))
	assert.False(t, templates.HasPage("checkout"), "a declared page without a template must be skipped")
	assert.False(t, templates.HasPage("unknown"))

	w := httptest.NewRecorder()
	renderErr := templates.RenderPage(w, sitessr.StandalonePageData{
		LayoutData: sitessr.LayoutData{Title: "Cart", PageTitle: "Cart - Lesstruct", Lang: "en"},
		Page:       "cart",
	})
	require.NoError(t, renderErr)
	assert.Contains(t, w.Body.String(), `data-page="cart"`)
	assert.Contains(t, w.Body.String(), "CART SHELL")

	w = httptest.NewRecorder()
	renderErr = templates.RenderPage(w, sitessr.StandalonePageData{
		LayoutData: sitessr.LayoutData{Title: "Track", PageTitle: "Track - Lesstruct", Lang: "en"},
		Page:       "orders/track",
	})
	require.NoError(t, renderErr)
	assert.Contains(t, w.Body.String(), `data-page="orders/track"`)
	assert.Contains(t, w.Body.String(), "Track shell")
}

func TestRenderPage_Undeclared(t *testing.T) {
	installThemeFixture(t)

	templates, err := sitessr.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	renderErr := templates.RenderPage(w, sitessr.StandalonePageData{Page: "checkout"})
	require.Error(t, renderErr)
	assert.Contains(t, renderErr.Error(), "checkout")
}

func TestServeStandalonePage(t *testing.T) {
	installThemeFixture(t)

	mockService := newMockReader()
	setupNavMocks(mockService)
	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `data-page="cart"`)
	assert.Contains(t, w.Body.String(), "CART SHELL")
	assert.Contains(t, w.Body.String(), `<meta name="robots" content="noindex">`)
	mockService.AssertExpectations(t)
}

func TestServeStandalonePage_FallsThroughWithoutTemplate(t *testing.T) {
	installThemeFixture(t)

	mockService := newMockReader()
	mockService.On("GetPublishedBySlugAny", mock.Anything, "checkout").Return(sitessr.ContentRef{}, assert.AnError)
	setupNavMocks(mockService)
	handler := setupHandler(t, mockService)

	req := httptest.NewRequest(http.MethodGet, "/checkout", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Not Found")
	mockService.AssertExpectations(t)
}

func TestNewTemplates_ThemeFuncsDoNotShadowEngine(t *testing.T) {
	installThemeFixture(t)
	sitessr.SetThemeFuncs(map[string]any{
		"assetURL": func(string) string { return "hijacked" },
		"shout":    strings.ToUpper,
	})

	templates, err := sitessr.NewTemplates(nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	renderErr := templates.RenderIndex(w, sitessr.IndexData{
		LayoutData: sitessr.LayoutData{Lang: "en"},
	})
	require.NoError(t, renderErr)
	assert.Contains(t, w.Body.String(), "/static/style.dev.css")
	assert.NotContains(t, w.Body.String(), "hijacked")
}
