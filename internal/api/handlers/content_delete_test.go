package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

func TestContentHandler_DeleteContent(t *testing.T) {
	tests := []struct {
		name           string
		contentID      string
		setupService   func(s *handlersmocks.MockContentServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful deletion",
			contentID: "1",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().DeleteContent(
					mock.Anything,
					1,
					1,
					mock.Anything,
				).Return(nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				if data["message"] != "Content deleted successfully" {
					t.Errorf("expected success message, got %v", data["message"])
				}
			},
		},
		{
			name:      "content not found",
			contentID: "999",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().DeleteContent(
					mock.Anything,
					999,
					1,
					mock.Anything,
				).Return(content.ErrContentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "content_not_found" {
					t.Errorf("expected code 'content_not_found', got %v", err["code"])
				}
			},
		},
		{
			name:      "unauthorized - wrong owner returns 403",
			contentID: "1",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
				s.EXPECT().DeleteContent(
					mock.Anything,
					1,
					1,
					mock.Anything,
				).Return(content.ErrUnauthorized)
			},
			expectedStatus: http.StatusForbidden,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "forbidden" {
					t.Errorf("expected code 'forbidden', got %v", err["code"])
				}
			},
		},
		{
			name:      "invalid content id",
			contentID: "abc",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_content_id" {
					t.Errorf("expected code 'invalid_content_id', got %v", err["code"])
				}
			},
		},
		{
			name:      "no authentication",
			contentID: "1",
			setupService: func(s *handlersmocks.MockContentServiceInterface) {
			},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "unauthorized" {
					t.Errorf("expected code 'unauthorized', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockContentServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewContentHandler(mockService, util.NewLogger(os.Stdout), "http://localhost:3000")

			r := chi.NewRouter()
			r.Delete("/api/v1/content_items/{id}", handler.DeleteContent)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/content_items/"+tt.contentID, nil)

			if tt.name != "no authentication" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, "1")
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}
