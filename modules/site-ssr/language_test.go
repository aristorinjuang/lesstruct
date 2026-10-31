package sitessr_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServeIndex_LanguageNegotiation(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		cookie       string
		acceptLang   string
		wantChain    []string
		wantLangAttr string
		wantCookie   bool
	}{
		{
			name:         "success - query lang reorders Indonesian first and sets cookie",
			url:          "/?lang=id",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   true,
		},
		{
			name:         "success - bogus query ignored without cookie",
			url:          "/?lang=bogus",
			wantChain:    []string{"en", "id"},
			wantLangAttr: `<html lang="en">`,
			wantCookie:   false,
		},
		{
			name:         "success - returning visitor cookie reorders without redirect",
			url:          "/",
			cookie:       "lesstruct_lang=id",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   false,
		},
		{
			name:         "success - bad cookie ignored",
			url:          "/",
			cookie:       "lesstruct_lang=bogus",
			wantChain:    []string{"en", "id"},
			wantLangAttr: `<html lang="en">`,
			wantCookie:   false,
		},
		{
			name:         "success - accept-language negotiates Indonesian first",
			url:          "/",
			acceptLang:   "id-ID,id;q=0.9,en;q=0.8",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   false,
		},
		{
			name:         "success - accept-language without match keeps primary",
			url:          "/",
			acceptLang:   "fr-FR,fr;q=0.9",
			wantChain:    []string{"en", "id"},
			wantLangAttr: `<html lang="en">`,
			wantCookie:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", tt.wantChain, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			setupNavMocks(mockService)

			handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}
			if tt.acceptLang != "" {
				req.Header.Set("Accept-Language", tt.acceptLang)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantLangAttr)
			if tt.wantCookie {
				assert.Contains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=id")
			} else {
				assert.NotContains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=")
			}
			mockService.AssertCalled(t, "GetPublishedByPostType", mock.Anything, "post", tt.wantChain, 0, 0, 51, 0)
		})
	}
}

func TestServeIndex_LangCookieSecure(t *testing.T) {
	tests := []struct {
		name           string
		tls            bool
		forwardedProto string
		wantSecure     bool
	}{
		{
			name:       "plain HTTP omits Secure",
			wantSecure: false,
		},
		{
			name:       "TLS sets Secure",
			tls:        true,
			wantSecure: true,
		},
		{
			name:           "forwarded https behind a terminating proxy sets Secure",
			forwardedProto: "https",
			wantSecure:     true,
		},
		{
			name:           "forwarded scheme matching is case-insensitive",
			forwardedProto: "HTTPS",
			wantSecure:     true,
		},
		{
			name:           "forwarded scheme tolerates surrounding whitespace",
			forwardedProto: "  https  ",
			wantSecure:     true,
		},
		{
			name:           "first forwarded value wins",
			forwardedProto: "https, http",
			wantSecure:     true,
		},
		{
			name:           "http first refuses even with later https",
			forwardedProto: "http, https",
			wantSecure:     false,
		},
		{
			name:           "blank forwarded header falls back to plain HTTP",
			forwardedProto: "   ",
			wantSecure:     false,
		},
		{
			name:           "forwarded http wins over TLS",
			tls:            true,
			forwardedProto: "http",
			wantSecure:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", []string{"id", "en"}, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			setupNavMocks(mockService)

			handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

			req := httptest.NewRequest(http.MethodGet, "/?lang=id", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			setCookie := w.Header().Get("Set-Cookie")
			require.Contains(t, setCookie, "lesstruct_lang=id")
			if tt.wantSecure {
				assert.Contains(t, setCookie, "Secure")
			} else {
				assert.NotContains(t, setCookie, "Secure")
			}
		})
	}
}

func TestServeContent_QueryLangRedirect(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		url        string
		setup      func(*mockReader)
		wantCode   int
		wantLoc    string
		wantCookie bool
	}{
		{
			name: "success - translated slug redirects with cookie",
			slug: "hello",
			url:  "/hello?lang=id",
			setup: func(m *mockReader) {
				group := 1
				m.On("GetPublishedBySlugAny", mock.Anything, "hello").Return(sitessr.ContentRef{
					ID: 10, Slug: "hello", Title: "Hello", Language: "en", TranslationGroupID: &group,
					Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}]}`,
				}, nil)
				m.On("GetTranslations", mock.Anything, 1, 10).Return([]sitessr.ContentRef{
					{ID: 11, Slug: "halo", Language: "id", TranslationGroupID: &group},
				}, nil)
				setupNavMocks(m)
			},
			wantCode:   http.StatusFound,
			wantLoc:    "/halo",
			wantCookie: true,
		},
		{
			name: "success - missing translation renders as-is",
			slug: "hello",
			url:  "/hello?lang=id",
			setup: func(m *mockReader) {
				group := 1
				m.On("GetPublishedBySlugAny", mock.Anything, "hello").Return(sitessr.ContentRef{
					ID: 10, Slug: "hello", Title: "Hello", Language: "en", TranslationGroupID: &group,
					Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}]}`,
				}, nil)
				m.On("GetTranslations", mock.Anything, 1, 10).Return([]sitessr.ContentRef{}, nil)
				m.On("GetPublishedByID", mock.Anything, 1).Return(sitessr.ContentRef{}, assert.AnError)
				setupNavMocks(m)
			},
			wantCode:   http.StatusOK,
			wantLoc:    "",
			wantCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			tt.setup(mockService)
			mockService.On("GetRelated", mock.Anything, mock.Anything, mock.Anything).Return([]sitessr.ContentRef{}, nil).Maybe()

			handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tt.wantCode, w.Code)
			if tt.wantLoc != "" {
				assert.Equal(t, tt.wantLoc, w.Header().Get("Location"))
			}
			if tt.wantCookie {
				assert.Contains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=id")
			}
		})
	}
}

func TestServeListings_LanguageNegotiation(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		setup      func(*mockReader, *MockPostTypeResolver)
		wantMethod string
		wantArgs   []any
		wantPath   string
	}{
		{
			name: "success - author listing negotiates Indonesian first",
			url:  "/authors/admin?lang=id",
			setup: func(m *mockReader, _ *MockPostTypeResolver) {
				m.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
				m.On("GetPublishedByAuthorUsername", mock.Anything, "admin", []string{"id", "en"}, 51, 0).Return([]sitessr.ContentRef{}, nil)
				setupNavMocks(m)
			},
			wantMethod: "GetPublishedByAuthorUsername",
			wantArgs:   []any{mock.Anything, "admin", []string{"id", "en"}, 51, 0},
			wantPath:   "/authors/admin",
		},
		{
			name: "success - tag listing negotiates Indonesian first",
			url:  "/tags/tee?lang=id",
			setup: func(m *mockReader, _ *MockPostTypeResolver) {
				m.On("GetPublishedByTag", mock.Anything, "tee", []string{"id", "en"}, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
				setupNavMocks(m)
			},
			wantMethod: "GetPublishedByTag",
			wantArgs:   []any{mock.Anything, "tee", []string{"id", "en"}, 0, 0, 51, 0},
			wantPath:   "/tags/tee",
		},
		{
			name: "success - post type listing negotiates Indonesian first",
			url:  "/menu-item?lang=id",
			setup: func(m *mockReader, r *MockPostTypeResolver) {
				m.On("GetPublishedByPostType", mock.Anything, "menu-item", []string{"id", "en"}, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
				setupNavMocks(m)
				r.On("GetBySlug", "menu-item").Return(posttype.PostType{Name: "Menu Item", Slug: "menu-item"}, nil)
			},
			wantMethod: "GetPublishedByPostType",
			wantArgs:   []any{mock.Anything, "menu-item", []string{"id", "en"}, 0, 0, 51, 0},
			wantPath:   "/menu-item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockResolver := new(MockPostTypeResolver)
			tt.setup(mockService, mockResolver)

			handler := setupHandlerWithLanguagesAndResolver(t, mockService, []string{"en", "id"}, mockResolver)

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), `<html lang="id">`)
			assert.Contains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=id")
			assert.Contains(t, w.Body.String(), tt.wantPath+"?lang=id")
			mockService.AssertCalled(t, tt.wantMethod, tt.wantArgs...)
			mockService.AssertExpectations(t)
		})
	}
}

func TestServeIndex_LanguageDropdownSelected(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		wantChain     []string
		wantSelected  string
		wantNotSelect string
	}{
		{
			name:          "success - active Indonesian code selected",
			url:           "/?lang=id",
			wantChain:     []string{"id", "en"},
			wantSelected:  `<option value="/?lang=id" selected>ID</option>`,
			wantNotSelect: `<option value="/?lang=en" selected>`,
		},
		{
			name:          "success - bogus code falls back to primary",
			url:           "/?lang=bogus",
			wantChain:     []string{"en", "id"},
			wantSelected:  `<option value="/?lang=en" selected>EN</option>`,
			wantNotSelect: `<option value="/?lang=id" selected>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockReader()
			mockService.On("GetPublishedByPostType", mock.Anything, "post", tt.wantChain, 0, 0, 51, 0).Return([]sitessr.ContentRef{}, nil)
			setupNavMocks(mockService)

			handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, `id="language-select"`)
			assert.Contains(t, body, tt.wantSelected)
			assert.NotContains(t, body, tt.wantNotSelect)
			mockService.AssertCalled(t, "GetPublishedByPostType", mock.Anything, "post", tt.wantChain, 0, 0, 51, 0)
		})
	}
}
