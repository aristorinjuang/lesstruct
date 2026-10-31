package sitessr_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServeIndex_ConsentDeclined(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		cookie       string
		wantChain    []string
		wantLangAttr string
		wantCookie   bool
	}{
		{
			name:         "success - declined renders Indonesian without persist",
			url:          "/?lang=id",
			cookie:       "lesstruct_consent=declined",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   false,
		},
		{
			name:         "success - accepted persists language cookie",
			url:          "/?lang=id",
			cookie:       "lesstruct_consent=accepted",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   true,
		},
		{
			name:         "success - no consent persists language cookie",
			url:          "/?lang=id",
			cookie:       "",
			wantChain:    []string{"id", "en"},
			wantLangAttr: `<html lang="id">`,
			wantCookie:   true,
		},
		{
			name:         "success - banner present on first visit",
			url:          "/",
			cookie:       "",
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
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantLangAttr)
			assert.Contains(t, w.Body.String(), `id="cookie-banner"`)
			if tt.wantCookie {
				assert.Contains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=id")
			} else {
				assert.NotContains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=")
			}
			mockService.AssertCalled(t, "GetPublishedByPostType", mock.Anything, "post", tt.wantChain, 0, 0, 51, 0)
		})
	}
}

func TestServeContent_ConsentDeclined(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		cookie     string
		wantCookie bool
	}{
		{
			name:       "success - declined renders translation without persist",
			url:        "/hello?lang=id",
			cookie:     "lesstruct_consent=declined",
			wantCookie: false,
		},
		{
			name:       "success - accepted persists language cookie on redirect",
			url:        "/hello?lang=id",
			cookie:     "lesstruct_consent=accepted",
			wantCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := 1
			mockService := newMockReader()
			mockService.On("GetPublishedBySlugAny", mock.Anything, "hello").Return(sitessr.ContentRef{
				ID: 10, Slug: "hello", Title: "Hello", Language: "en", TranslationGroupID: &group,
				Content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hi"}]}]}`,
			}, nil)
			mockService.On("GetTranslations", mock.Anything, 1, 10).Return([]sitessr.ContentRef{
				{ID: 11, Slug: "halo", Language: "id", TranslationGroupID: &group},
			}, nil)
			setupNavMocks(mockService)

			handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			req.Header.Set("Cookie", tt.cookie)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, "/halo", w.Header().Get("Location"))
			if tt.wantCookie {
				assert.Contains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=id")
			} else {
				assert.NotContains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=")
			}
		})
	}
}

func TestServeAuthor_ConsentDeclined(t *testing.T) {
	mockService := newMockReader()
	mockService.On("AuthorExists", mock.Anything, "admin").Return(true, nil)
	mockService.On("GetPublishedByAuthorUsername", mock.Anything, "admin", []string{"id", "en"}, 51, 0).Return([]sitessr.ContentRef{}, nil)
	setupNavMocks(mockService)

	handler := setupHandlerWithLanguages(t, mockService, []string{"en", "id"})

	req := httptest.NewRequest(http.MethodGet, "/authors/admin?lang=id", nil)
	req.Header.Set("Cookie", "lesstruct_consent=declined")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `<html lang="id">`)
	assert.NotContains(t, w.Header().Get("Set-Cookie"), "lesstruct_lang=")
	mockService.AssertCalled(t, "GetPublishedByAuthorUsername", mock.Anything, "admin", []string{"id", "en"}, 51, 0)
}
