package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	dashboarddomain "github.com/aristorinjuang/lesstruct/internal/domain/dashboard"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDashboardHandler_GetStats_Success(t *testing.T) {
	mockService := handlersmocks.NewMockDashboardServiceInterface(t)
	mockService.EXPECT().
		GetStats(mock.Anything, 1).
		Return(&dashboarddomain.Stats{
			PublishedPosts:       10,
			DraftPosts:           5,
			RegisteredUsers:      3,
			PendingRegistrations: 2,
			MediaItems:           25,
			RecentContent: []*dashboarddomain.RecentItem{
				{
					ID:        15,
					Title:     "Latest Post",
					Slug:      "latest-post",
					Status:    "published",
					CreatedAt: "2026-04-10T10:30:00Z",
				},
			},
		}, nil)

	handler := handlers.NewDashboardHandler(mockService, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]any
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "Response data should be an object")

	assert.Equal(t, float64(10), data["publishedPosts"])
	assert.Equal(t, float64(5), data["draftPosts"])
	assert.Equal(t, float64(3), data["registeredUsers"])
	assert.Equal(t, float64(2), data["pendingRegistrations"])
	assert.Equal(t, float64(25), data["mediaItems"])
}

func TestDashboardHandler_GetStats_EmptyStats(t *testing.T) {
	mockService := handlersmocks.NewMockDashboardServiceInterface(t)
	mockService.EXPECT().
		GetStats(mock.Anything, 1).
		Return(&dashboarddomain.Stats{
			PublishedPosts:       0,
			DraftPosts:           0,
			RegisteredUsers:      1,
			PendingRegistrations: 0,
			MediaItems:           0,
			RecentContent:        []*dashboarddomain.RecentItem{},
		}, nil)

	handler := handlers.NewDashboardHandler(mockService, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]any
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "Response data should be an object")

	assert.Equal(t, float64(0), data["publishedPosts"])
	assert.Equal(t, float64(0), data["draftPosts"])
	assert.Equal(t, float64(1), data["registeredUsers"])
	assert.Equal(t, float64(0), data["pendingRegistrations"])
	assert.Equal(t, float64(0), data["mediaItems"])
}

func TestDashboardHandler_GetStats_Unauthorized(t *testing.T) {
	mockService := handlersmocks.NewMockDashboardServiceInterface(t)

	handler := handlers.NewDashboardHandler(mockService, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var response map[string]any
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	errorResp, ok := response["error"].(map[string]any)
	require.True(t, ok, "Response should contain error")
	assert.Equal(t, "unauthorized", errorResp["code"])
}

func TestDashboardHandler_GetStats_ServiceError(t *testing.T) {
	mockService := handlersmocks.NewMockDashboardServiceInterface(t)
	mockService.EXPECT().
		GetStats(mock.Anything, 1).
		Return(nil, errors.New("database error"))

	handler := handlers.NewDashboardHandler(mockService, util.NewLogger(os.Stdout))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var response map[string]any
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	errorResp, ok := response["error"].(map[string]any)
	require.True(t, ok, "Response should contain error")
	assert.Equal(t, "internal_error", errorResp["code"])
}

func TestDashboardHandler_GetStats_InvalidUserID(t *testing.T) {
	tests := []struct {
		name   string
		userID string
	}{
		{name: "zero user ID", userID: "0"},
		{name: "negative user ID", userID: "-1"},
		{name: "non-numeric user ID", userID: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockDashboardServiceInterface(t)

			handler := handlers.NewDashboardHandler(mockService, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.GetStats(w, req)

			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var response map[string]any
			err := json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			errorResp, ok := response["error"].(map[string]any)
			require.True(t, ok, "Response should contain error")
			assert.Equal(t, "invalid_user_id", errorResp["code"])
		})
	}
}
