package handlers_test

import (
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestContentHandler_CustomFieldQuery_ExactMatch(t *testing.T) {
	t.Run("cf_category=Pastry returns matching content", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			1,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return len(f.CustomFieldFilters) == 1 &&
					f.CustomFieldFilters[0].Field == "category" &&
					f.CustomFieldFilters[0].Operator == contentdomain.FilterOpEqual &&
					f.CustomFieldFilters[0].Value == "Pastry"
			}),
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "Croissant", CustomFields: map[string]any{"category": "Pastry"}},
			{ID: 2, Title: "Eclair", CustomFields: map[string]any{"category": "Pastry"}},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?cf_category=Pastry", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		data := resp["data"].([]any)
		require.Len(t, data, 2)
	})
}

func TestContentHandler_CustomFieldQuery_NumberRange(t *testing.T) {
	t.Run("cf_price_min=5&cf_price_max=20 returns range-filtered content", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			1,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				if len(f.CustomFieldFilters) != 2 {
					return false
				}
				hasMin := false
				hasMax := false
				for _, cf := range f.CustomFieldFilters {
					if cf.Field == "price" && cf.Operator == contentdomain.FilterOpMin && cf.Value == "5" {
						hasMin = true
					}
					if cf.Field == "price" && cf.Operator == contentdomain.FilterOpMax && cf.Value == "20" {
						hasMax = true
					}
				}
				return hasMin && hasMax
			}),
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "Croissant", CustomFields: map[string]any{"price": 7.5}},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?cf_price_min=5&cf_price_max=20", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestContentHandler_CustomFieldQuery_CombinedFilter(t *testing.T) {
	t.Run("post_type=menu-item&cf_category=Pastry combined filter", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			1,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return f.PostType == "menu-item" &&
					len(f.CustomFieldFilters) == 1 &&
					f.CustomFieldFilters[0].Field == "category" &&
					f.CustomFieldFilters[0].Operator == contentdomain.FilterOpEqual &&
					f.CustomFieldFilters[0].Value == "Pastry"
			}),
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "Croissant", PostType: "menu-item", CustomFields: map[string]any{"category": "Pastry"}},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?post_type=menu-item&cf_category=Pastry", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestContentHandler_CustomFieldQuery_InvalidSlug(t *testing.T) {
	t.Run("invalid cf_ parameter slug ignored gracefully", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetByUser(
			mock.Anything,
			1,
			100,
			0,
		).Return([]*contentdomain.Content{}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?cf_Cat!gori@=Pastry", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid slug alongside valid filters only uses valid ones", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			1,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return len(f.CustomFieldFilters) == 1 &&
					f.CustomFieldFilters[0].Field == "category" &&
					f.CustomFieldFilters[0].Operator == contentdomain.FilterOpEqual &&
					f.CustomFieldFilters[0].Value == "Pastry"
			}),
		).Return([]*contentdomain.Content{}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?cf_Cat!gori@=Bread&cf_category=Pastry", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestContentHandler_CustomFieldQuery_NoFilters(t *testing.T) {
	t.Run("missing filter parameters returns all content via GetByUser", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().GetByUser(
			mock.Anything,
			1,
			100,
			0,
		).Return([]*contentdomain.Content{
			{ID: 1, Title: "Post 1"},
			{ID: 2, Title: "Post 2"},
		}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		data := resp["data"].([]any)
		require.Len(t, data, 2)
	})
}

func TestContentHandler_CustomFieldQuery_PostTypeOnly(t *testing.T) {
	t.Run("post_type only uses ListByFilters", func(t *testing.T) {
		mockService := handlersmocks.NewMockContentServiceInterface(t)
		mockService.EXPECT().ListByFilters(
			mock.Anything,
			1,
			mock.MatchedBy(func(f contentdomain.ContentFilters) bool {
				return f.PostType == "page" && len(f.CustomFieldFilters) == 0
			}),
		).Return([]*contentdomain.Content{}, nil)

		handler := handlers.NewContentHandler(
			mockService,
			util.NewLogger(os.Stdout),
			"http://localhost:3000",
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/content_items?post_type=page", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ListContents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}
