package commerce_test

import (
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	themestorefront "github.com/aristorinjuang/lesstruct/modules/theme-storefront"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installStorefrontTheme installs the real storefront artifact into the
// rendering engine, mirroring the composition root. The cleanup resets the
// process-wide theme setters so package tests never inherit this fixture.
func installStorefrontTheme(t *testing.T) {
	t.Helper()

	module := themestorefront.New()
	sitessr.SetEmbeddedFS(module.ThemeFiles())
	sitessr.SetThemeFuncs(module.ThemeFuncs())
	sitessr.SetThemePages(module.ThemePages())
	t.Cleanup(func() {
		sitessr.SetEmbeddedFS(nil)
		sitessr.SetThemeFuncs(nil)
		sitessr.SetThemePages(nil)
	})
}

func storefrontLayout() sitessr.LayoutData {
	return sitessr.LayoutData{
		Title:       "Lesstruct",
		Description: "Official Lesstruct merchandise.",
		OGTitle:     "Lesstruct",
		OGDesc:      "Official Lesstruct merchandise.",
		PageTitle:   "Lesstruct",
		CurrentPath: "/",
		Lang:        "en",
		SiteConfig:  sitessr.SiteConfig{Name: "Lesstruct"},
		NavigationItems: []sitessr.NavigationItem{
			{Title: "Shop", URL: "/", IsActive: true},
		},
	}
}

func storefrontPost() sitessr.PostItem {
	return sitessr.PostItem{
		Slug:            "heavyweight-tee",
		Title:           "Heavyweight Tee",
		MetaDescription: "A heavyweight cotton tee.",
		ImageURL:        "/media/tee.jpg",
		CreatedAt:       time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
		Tags:            []string{"apparel"},
	}
}

// TestStorefrontThemeRendersAllPageTypes renders every page type the real
// storefront theme ships. It is the regression net for template-level drift
// (wrong func arity, missing partials, broken field names) that unit tests
// over individual handlers cannot see.
func TestStorefrontThemeRendersAllPageTypes(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	layout := storefrontLayout()
	post := storefrontPost()
	posts := []sitessr.PostItem{post}

	tests := []struct {
		name   string
		render func(*sitessr.Templates, *httptest.ResponseRecorder) error
	}{
		{
			name: "index",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderIndex(w, sitessr.IndexData{
					LayoutData: layout,
					Posts:      posts,
					Tags:       []string{"apparel"},
				})
			},
		},
		{
			name: "home",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderHome(w, sitessr.IndexData{
					LayoutData: layout,
					Posts:      posts,
				})
			},
		},
		{
			name: "content post",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderContent(w, sitessr.ContentData{
					LayoutData:    layout,
					Slug:          post.Slug,
					Body:          template.HTML("<p>Hello from the storefront.</p>"),
					Tags:          post.Tags,
					Author:        "Admin",
					Username:      "admin",
					CreatedAt:     post.CreatedAt,
					AllowComments: true,
					Related:       posts,
					Comments: []sitessr.CommentItem{
						{Author: "Buyer", Text: "Great tee.", CreatedAt: post.CreatedAt},
					},
				})
			},
		},
		{
			name: "content page",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderContent(w, sitessr.ContentData{
					LayoutData: layout,
					Slug:       "about",
					PostType:   "page",
					Body:       template.HTML("<p>About Lesstruct.</p>"),
				})
			},
		},
		{
			name: "content product",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderContent(w, sitessr.ContentData{
					LayoutData: layout,
					Slug:       post.Slug,
					PostType:   "product",
					Body:       template.HTML("<p>Details.</p>"),
					Tags:       post.Tags,
					CustomFields: map[string]any{
						"images":   "/media/tee.jpg\n/media/tee-back.jpg",
						"material": "Cotton",
						"sku":      "TEE-1",
					},
					Related: posts,
				})
			},
		},
		{
			name: "author",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderAuthor(w, sitessr.AuthorData{
					LayoutData: layout,
					AuthorName: "Admin",
					Username:   "admin",
					Posts:      posts,
				})
			},
		},
		{
			name: "tag",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderTag(w, sitessr.TagData{
					LayoutData: layout,
					TagName:    "apparel",
					Posts:      posts,
				})
			},
		},
		{
			name: "not found",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderNotFound(w, sitessr.NotFoundData{LayoutData: layout})
			},
		},
		{
			name: "login",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderLogin(w, sitessr.AuthPageData{LayoutData: layout, ShowRegister: true})
			},
		},
		{
			name: "register",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderRegister(w, sitessr.AuthPageData{LayoutData: layout})
			},
		},
		{
			name: "forgot password",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderForgotPassword(w, sitessr.AuthPageData{LayoutData: layout})
			},
		},
		{
			name: "verify email",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderVerifyEmail(w, sitessr.VerifyEmailData{LayoutData: layout})
			},
		},
		{
			name: "reset password",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderResetPassword(w, sitessr.ResetPasswordData{LayoutData: layout})
			},
		},
		{
			name: "cart",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderPage(w, sitessr.StandalonePageData{LayoutData: layout, Page: "cart"})
			},
		},
		{
			name: "checkout",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderPage(w, sitessr.StandalonePageData{LayoutData: layout, Page: "checkout"})
			},
		},
		{
			name: "order tracking",
			render: func(templates *sitessr.Templates, w *httptest.ResponseRecorder) error {
				return templates.RenderPage(w, sitessr.StandalonePageData{LayoutData: layout, Page: "orders/track"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := tt.render(templates, w)
			require.NoError(t, err)
			assert.Contains(t, w.Body.String(), "<html")
		})
	}
}

// TestStorefrontThemeFavicons checks the layout's favicon links: the bundled
// canonical set by default, and the [site_config] override when one is set.
func TestStorefrontThemeFavicons(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name            string
		favicon         string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "bundled canonical set",
			wantContains: []string{
				`<link rel="icon" type="image/png" href="/static/favicon-96x96.png" sizes="96x96">`,
				`<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">`,
				`<link rel="shortcut icon" href="/static/favicon.ico">`,
				`<link rel="apple-touch-icon" sizes="180x180" href="/static/apple-touch-icon.png">`,
			},
			wantNotContains: []string{`<link rel="icon" href=`},
		},
		{
			name:    "configured override",
			favicon: "/uploads/icon.png",
			wantContains: []string{
				`<link rel="icon" href="/uploads/icon.png">`,
				`<link rel="apple-touch-icon" href="/uploads/icon.png">`,
			},
			wantNotContains: []string{"/static/favicon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			layout.SiteConfig.Favicon = tt.favicon

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderIndex(w, sitessr.IndexData{
				LayoutData: layout,
				Posts:      []sitessr.PostItem{storefrontPost()},
			}))

			body := w.Body.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContains {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

// TestStorefrontThemeRendersComments covers the comment section the storefront
// post template shares with the default theme: listing existing comments,
// exposing the form, and loading comments.js only when comments are allowed.
func TestStorefrontThemeRendersComments(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	post := storefrontPost()
	tests := []struct {
		name         string
		data         sitessr.ContentData
		wantComments bool
	}{
		{
			name: "comments enabled",
			data: sitessr.ContentData{
				LayoutData:    storefrontLayout(),
				Slug:          post.Slug,
				AllowComments: true,
				Comments: []sitessr.CommentItem{
					{Author: "Buyer", Text: "Great tee.", CreatedAt: post.CreatedAt},
				},
			},
			wantComments: true,
		},
		{
			name: "comments disabled",
			data: sitessr.ContentData{
				LayoutData: storefrontLayout(),
				Slug:       post.Slug,
			},
			wantComments: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderContent(w, tt.data))

			body := w.Body.String()
			if tt.wantComments {
				assert.Contains(t, body, `class="comments content-wrapper"`)
				assert.Contains(t, body, "Great tee.")
				assert.Contains(t, body, `id="comment-form"`)
				assert.Contains(t, body, `src="/static/comments.js"`)
				return
			}
			assert.NotContains(t, body, `class="comments content-wrapper"`)
			assert.NotContains(t, body, `src="/static/comments.js"`)
		})
	}
}

// TestStorefrontThemeStandalonePageNotices checks that the storefront layout
// honours .NoIndex and that each commerce shell explains the JavaScript
// requirement in the site language.
func TestStorefrontThemeStandalonePageNotices(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	tests := []struct {
		name     string
		page     string
		lang     string
		expected string
	}{
		{
			name:     "cart in English",
			page:     "cart",
			lang:     "en",
			expected: "JavaScript is required to load your cart.",
		},
		{
			name:     "cart in Indonesian",
			page:     "cart",
			lang:     "id",
			expected: "JavaScript diperlukan untuk memuat keranjang.",
		},
		{
			name:     "checkout",
			page:     "checkout",
			lang:     "en",
			expected: "JavaScript is required to check out.",
		},
		{
			name:     "order tracking",
			page:     "orders/track",
			lang:     "en",
			expected: "JavaScript is required to track your order.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := storefrontLayout()
			layout.Lang = tt.lang
			layout.NoIndex = true

			w := httptest.NewRecorder()
			require.NoError(t, templates.RenderPage(w, sitessr.StandalonePageData{
				LayoutData: layout,
				Page:       tt.page,
			}))

			body := w.Body.String()
			assert.Contains(t, body, `<meta name="robots" content="noindex">`)
			assert.Contains(t, body, "<noscript>")
			assert.Contains(t, body, tt.expected)
		})
	}
}

// TestStorefrontThemeChildLayoutKeepsEmbeddedPartials covers the THEME_DIR
// override path: a child theme that supplies only layout.html must keep the
// partials the embedded storefront layout defines, such as product-card.
func TestStorefrontThemeChildLayoutKeepsEmbeddedPartials(t *testing.T) {
	installStorefrontTheme(t)

	themeDir := t.TempDir()
	templatesDir := filepath.Join(themeDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	childLayout := `{{define "layout"}}<!DOCTYPE html>
<html lang="{{.Lang}}">
<head><title>{{.PageTitle}}</title></head>
<body data-child-layout="true">{{template "body" .}}</body>
</html>{{end}}`
	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "layout.html"), []byte(childLayout), 0o644))

	templates, err := sitessr.NewTemplates(&sitessr.Theme{Dir: themeDir}, nil, "post", "page", "product")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	err = templates.RenderHome(w, sitessr.IndexData{
		LayoutData: storefrontLayout(),
		Posts:      []sitessr.PostItem{storefrontPost()},
	})
	require.NoError(t, err)

	body := w.Body.String()
	assert.Contains(t, body, `data-child-layout="true"`)
	assert.Contains(t, body, "Heavyweight Tee")
	assert.NotContains(t, body, "site-footer")
}

// TestStorefrontHeaderOrder pins the header switcher placement: Cart before
// the currency select, currency before the language dropdown, language before
// the log-out link. The language dropdown shows the active code uppercased.
func TestStorefrontHeaderOrder(t *testing.T) {
	installStorefrontTheme(t)

	templates, err := sitessr.NewTemplates(nil, nil, "post", "page", "product")
	require.NoError(t, err)

	layout := storefrontLayout()
	layout.Lang = "id"
	layout.Languages = []sitessr.LanguageLink{
		{Code: "en", Name: "English", URL: "/?lang=en"},
		{Code: "id", Name: "Indonesian", URL: "/?lang=id"},
	}

	w := httptest.NewRecorder()
	require.NoError(t, templates.RenderIndex(w, sitessr.IndexData{
		LayoutData: layout,
		Posts:      []sitessr.PostItem{storefrontPost()},
	}))

	body := w.Body.String()
	// The theme styles are inlined in <head>; scope the order check past
	// <body> so CSS selectors can't satisfy it.
	if _, markup, ok := strings.Cut(body, "<body>"); ok {
		body = markup
	}
	cartIdx := strings.Index(body, "cart-link")
	currencyIdx := strings.Index(body, "currency-select")
	languageIdx := strings.Index(body, "language-select")
	logoutIdx := strings.Index(body, "nav-logout")

	require.NotEqual(t, -1, cartIdx)
	require.NotEqual(t, -1, currencyIdx)
	require.NotEqual(t, -1, languageIdx)
	require.NotEqual(t, -1, logoutIdx)
	assert.Less(t, cartIdx, currencyIdx)
	assert.Less(t, currencyIdx, languageIdx)
	assert.Less(t, languageIdx, logoutIdx)
	assert.Contains(t, body, `<option value="/?lang=id" selected>ID</option>`)
}
