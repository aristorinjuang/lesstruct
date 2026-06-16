package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestContentHandler_CustomFields_CreateContent(t *testing.T) {
	t.Run("create content with custom fields passes them through", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().Create(
			mock.Anything,
			1,
			mock.MatchedBy(func(req contentdomain.CreateContentRequest) bool {
				return req.CustomFields != nil && req.CustomFields["price"] == "$4.50"
			}),
		).Return(&contentdomain.Content{
			ID:           1,
			UserID:       1,
			Title:        "Test",
			Slug:         "test",
			Content:      "body",
			Tags:         []string{},
			Status:       contentdomain.StatusDraft,
			CustomFields: map[string]any{"price": "$4.50"},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		body := `{
			"title": "Test",
			"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"body\"}]}]}",
			"tags": [],
			"status": "draft",
			"customFields": {"price": "$4.50"}
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.CreateContent(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		data := resp["data"].(map[string]any)
		content := data["content"].(map[string]any)
		require.NotNil(t, content["customFields"])
		cf := content["customFields"].(map[string]any)
		require.Equal(t, "$4.50", cf["price"])
	})
}

func TestContentHandler_CustomFields_GetPublishedContent(t *testing.T) {
	t.Run("published content API includes customFields", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedBySlug(
			mock.Anything,
			"chocolate-croissant",
			"en",
		).Return(&contentdomain.Content{
			ID:           1,
			UserID:       1,
			Title:        "Chocolate Croissant",
			Slug:         "chocolate-croissant",
			Content:      "body",
			Tags:         []string{},
			Status:       contentdomain.StatusPublished,
			PostType:     "menu-item",
			CustomFields: map[string]any{"price": "$4.50", "servings": float64(2)},
			Author:       "Chef",
			Username:     "chef",
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		r := chi.NewRouter()
		r.Get("/api/v1/public/content_items/{slug}", handler.GetPublishedContent)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/chocolate-croissant", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		data := resp["data"].(map[string]any)
		cf, ok := data["customFields"]
		require.True(t, ok, "customFields should be present in response")
		cfMap := cf.(map[string]any)
		require.Equal(t, "$4.50", cfMap["price"])
		require.Equal(t, float64(2), cfMap["servings"])
	})

	t.Run("published content without custom fields omits customFields key", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetPublishedBySlug(
			mock.Anything,
			"plain-post",
			"en",
		).Return(&contentdomain.Content{
			ID:       1,
			UserID:   1,
			Title:    "Plain Post",
			Slug:     "plain-post",
			Content:  "body",
			Tags:     []string{},
			Status:   contentdomain.StatusPublished,
			PostType: "post",
			Author:   "Chef",
			Username: "chef",
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		r := chi.NewRouter()
		r.Get("/api/v1/public/content_items/{slug}", handler.GetPublishedContent)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/plain-post", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		data := resp["data"].(map[string]any)
		_, exists := data["customFields"]
		require.False(t, exists, "customFields should be omitted when nil")
	})
}

func TestContentHandler_CustomFields_UpdateContent(t *testing.T) {
	t.Run("update content with custom fields passes them through", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().Update(
			mock.Anything,
			1,
			1,
			mock.Anything,
			mock.MatchedBy(func(req contentdomain.UpdateContentRequest) bool {
				return req.CustomFields != nil && req.CustomFields["color"] == "red"
			}),
		).Return(&contentdomain.Content{
			ID:           1,
			UserID:       1,
			Title:        "Updated",
			Slug:         "test",
			Content:      "body",
			Tags:         []string{},
			Status:       contentdomain.StatusDraft,
			CustomFields: map[string]any{"color": "red"},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		r := chi.NewRouter()
		r.Put("/api/v1/content_items/{id}", func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, "1")
			handler.UpdateContent(w, r.WithContext(ctx))
		})

		body := `{
			"title": "Updated",
			"content": "{\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"body\"}]}]}",
			"tags": [],
			"status": "draft",
			"customFields": {"color": "red"}
		}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/content_items/1", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}
