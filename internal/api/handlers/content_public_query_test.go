package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// stubPublicFieldRegistry is a test double for the handlers.PublicFieldLookup
// interface. It answers IsQueryable strictly from the allow map, keyed by
// "<resource>:<field>:<operation>". Post-type scoping is exercised by the
// config package's own tests; the handler tests here focus on routing and
// error mapping.
type stubPublicFieldRegistry struct {
	allow map[string]bool
}

func (s *stubPublicFieldRegistry) IsQueryable(resource, postType, field, operation string) bool {
	return s.allow[resource+":"+field+":"+operation]
}

func (s *stubPublicFieldRegistry) ExposedFields(resource, postType string) []string {
	var result []string
	for key := range s.allow {
		if !s.allow[key] {
			continue
		}
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			continue
		}
		if parts[0] == resource && parts[2] == "expose" {
			result = append(result, parts[1])
		}
	}
	return result
}

func newPublicQueryRouter(t *testing.T, mockService *handlersmocks.MockContentServiceInterface, registry handlers.PublicFieldLookup) *chi.Mux {
	t.Helper()
	handler := handlers.NewContentHandler(
		mockService,
		util.NewLogger(os.Stdout),
		"http://localhost:3000",
		nil,
		nil,
	)
	if registry != nil {
		handler = handler.WithPublicFieldRegistry(registry)
	}

	r := chi.NewRouter()
	r.Get("/api/v1/public/content_items", handler.ListPublishedContents)
	r.Get("/api/v1/public/authors", handler.ListPublishedAuthors)
	r.Get("/api/v1/public/authors/{username}", handler.GetPublishedAuthor)
	return r
}

func TestPublicQuery_AllowlistRejectsUnknownField(t *testing.T) {
	t.Run("content_items: cf_<field> not in allowlist returns 400 field_not_queryable", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{allow: map[string]bool{}} // empty: everything rejected
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items?cf_category=News", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "field_not_queryable")
		mockService.AssertNotCalled(t, "ListByFilters")
	})

	t.Run("content_items: sort_by=cf:<field> not in allowlist returns 400 field_not_queryable", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items?sort_by=cf:views&order=desc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "field_not_queryable")
		mockService.AssertNotCalled(t, "ListByFilters")
	})

	t.Run("authors: sort_by=cf:<field> rejected when allowlist only covers content", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"content:views:sort": true, // wrong resource
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=cf:points", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "field_not_queryable")
		mockService.AssertNotCalled(t, "GetPublishedAuthors")
	})
}

func TestPublicQuery_AllowlistAllowsKnownField(t *testing.T) {
	t.Run("content_items: cf filter and sort on allowlisted fields returns 200", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			0,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return f.PostType == "article" &&
					f.Status == string(contentdomain.StatusPublished) &&
					f.SortBy == "views" &&
					f.SortOrder == "desc" &&
					len(f.CustomFieldFilters) == 1 &&
					f.CustomFieldFilters[0].Field == "category" &&
					f.CustomFieldFilters[0].Operator == contentdomain.FilterOpEqual &&
					f.CustomFieldFilters[0].Value == "News"
			}),
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "News Article", Slug: "news-1", Content: "{}", PostType: "article"},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"content:category:filter": true,
				"content:views:sort":      true,
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/public/content_items?post_type=article&cf_category=News&sort_by=cf:views&order=desc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		assert.Contains(t, w.Body.String(), "News Article")
	})

	t.Run("authors: sort_by=cf:points on allowlisted field returns 200", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.MatchedBy(func(f contentdomain.PublishedAuthorFilters) bool {
				return f.SortBy == "points" && f.SortOrder == "desc" && f.Limit == 5
			}),
		).Return([]*contentdomain.PublishedAuthor{
			{Username: "topuser", DisplayName: "Top User", ContentCount: 9},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"user:points:sort": true,
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=cf:points&order=desc&limit=5", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		assert.Contains(t, w.Body.String(), "topuser")
	})

	t.Run("authors: cf filter on allowlisted field returns 200", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.MatchedBy(func(f contentdomain.PublishedAuthorFilters) bool {
				return len(f.CustomFieldFilters) == 1 &&
					f.CustomFieldFilters[0].Field == "tier" &&
					f.CustomFieldFilters[0].Value == "gold"
			}),
		).Return([]*contentdomain.PublishedAuthor{
			{Username: "golduser", DisplayName: "Gold User", ContentCount: 3},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"user:tier:filter": true,
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?cf_tier=gold", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		assert.Contains(t, w.Body.String(), "golduser")
	})
}

func TestPublicQuery_SortParsing(t *testing.T) {
	t.Run("sort_by without cf: prefix returns 400 invalid_sort", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=points", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_sort")
	})

	t.Run("sort_by=cf: with empty field returns 400 invalid_sort", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=cf:", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_sort")
	})

	t.Run("order=invalid returns 400 invalid_sort", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=cf:points&order=invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_sort")
	})
}

func TestPublicQuery_LanguageParamForwarded(t *testing.T) {
	t.Run("content_items: ?language=id routes to ListByFilters with Language field set", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			0,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return f.Language == "id" &&
					f.Status == string(contentdomain.StatusPublished)
			}),
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "Artikel", Slug: "artikel-1", Content: "{}", PostType: "article", Language: "id"},
		}, nil)

		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items?language=id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		assert.Contains(t, w.Body.String(), "Artikel")
	})

	t.Run("content_items: ?language=id with post_type routes to ListByFilters, not GetPublishedByPostType", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			0,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return f.Language == "id" &&
					f.PostType == "article" &&
					f.Status == string(contentdomain.StatusPublished)
			}),
		).Return([]*contentdomain.Content{}, nil)

		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items?post_type=article&language=id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		mockService.AssertNotCalled(t, "GetPublishedByPostType")
	})
}

func TestPublicQuery_NilRegistryFailsClosed(t *testing.T) {
	t.Run("handler without registry rejects every cf_* and sort_by=cf:*", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		// Deliberately NOT calling WithPublicFieldRegistry — simulates the
		// default for any test or operator that forgets to wire it.
		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
			nil,
			nil,
		)
		r := chi.NewRouter()
		r.Get("/api/v1/public/authors", handler.ListPublishedAuthors)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors?sort_by=cf:points", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "field_not_queryable")
		mockService.AssertNotCalled(t, "GetPublishedAuthors")
	})

	t.Run("handler without registry still serves plain (unfiltered) requests", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.MatchedBy(func(f contentdomain.PublishedAuthorFilters) bool {
				return f.SortBy == "" && f.SortOrder == "" && len(f.CustomFieldFilters) == 0
			}),
		).Return([]*contentdomain.PublishedAuthor{}, nil)
		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
			nil,
			nil,
		)
		r := chi.NewRouter()
		r.Get("/api/v1/public/authors", handler.ListPublishedAuthors)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestPublicQuery_AuthorsExposeFields(t *testing.T) {
	t.Run("ListPublishedAuthors includes publicFields when expose is allowlisted", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.Anything,
		).Return([]*contentdomain.PublishedAuthor{
			{
				Username:       "topuser",
				DisplayName:    "Top User",
				ContentCount:   10,
				CustomFields:   map[string]any{"tier_point": float64(500), "secret_field": "hidden"},
			},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"user:tier_point:expose": true,
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "topuser")
		assert.Contains(t, body, "publicFields")
		assert.Contains(t, body, "tier_point")
		assert.Contains(t, body, "500")
		assert.NotContains(t, body, "secret_field",
			"non-exposed field must not appear in the response")
		assert.NotContains(t, body, "hidden")
	})

	t.Run("ListPublishedAuthors omits publicFields when no expose configured", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.Anything,
		).Return([]*contentdomain.PublishedAuthor{
			{
				Username:     "plainuser",
				DisplayName:  "Plain User",
				ContentCount: 5,
				CustomFields: map[string]any{"tier_point": float64(500)},
			},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"user:tier_point:sort": true, // sort only, no expose
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "plainuser")
		assert.NotContains(t, body, "publicFields",
			"publicFields key must be omitted when no field has expose")
		assert.NotContains(t, body, "tier_point",
			"field values must not leak without expose")
	})

	t.Run("ListPublishedAuthors omits publicFields with nil registry", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthors(
			mock.Anything,
			mock.Anything,
		).Return([]*contentdomain.PublishedAuthor{
			{
				Username:     "safeuser",
				DisplayName:  "Safe User",
				ContentCount: 1,
				CustomFields: map[string]any{"secret": "leak"},
			},
		}, nil)

		router := newPublicQueryRouter(t, mockService, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		body := w.Body.String()
		assert.NotContains(t, body, "publicFields")
		assert.NotContains(t, body, "secret")
	})
}

func TestPublicQuery_GetPublishedAuthor(t *testing.T) {
	t.Run("success with exposed fields", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthor(
			mock.Anything,
			"johndoe",
		).Return(&contentdomain.PublishedAuthor{
			Username:     "johndoe",
			DisplayName:  "John Doe",
			ContentCount: 42,
			PostTypes:    []string{"article"},
			CustomFields: map[string]any{"tier_point": float64(900), "rank": "silver"},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{
				"user:tier_point:expose": true,
			},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors/johndoe", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "johndoe")
		assert.Contains(t, body, "John Doe")
		assert.Contains(t, body, "publicFields")
		assert.Contains(t, body, "tier_point")
		assert.Contains(t, body, "900")
		assert.NotContains(t, body, "rank",
			"non-exposed field must not appear")
		assert.NotContains(t, body, "silver")
	})

	t.Run("success without expose omits publicFields", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthor(
			mock.Anything,
			"plainjane",
		).Return(&contentdomain.PublishedAuthor{
			Username:     "plainjane",
			DisplayName:  "Plain Jane",
			ContentCount: 3,
			CustomFields: map[string]any{"tier_point": float64(100)},
		}, nil)

		registry := &stubPublicFieldRegistry{
			allow: map[string]bool{},
		}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors/plainjane", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		body := w.Body.String()
		assert.Contains(t, body, "plainjane")
		assert.NotContains(t, body, "publicFields")
		assert.NotContains(t, body, "tier_point")
	})

	t.Run("404 when author has no published content", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthor(
			mock.Anything,
			"ghost",
		).Return(nil, nil)

		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors/ghost", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "author_not_found")
	})

	t.Run("500 on service error", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedAuthor(
			mock.Anything,
			"erroruser",
		).Return(nil, assert.AnError)

		registry := &stubPublicFieldRegistry{allow: map[string]bool{}}
		router := newPublicQueryRouter(t, mockService, registry)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/authors/erroruser", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal_error")
	})
}
