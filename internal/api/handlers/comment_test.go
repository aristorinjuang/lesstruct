package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	contentmocks "github.com/aristorinjuang/lesstruct/internal/domain/content/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

const userID = 1

func TestCommentHandler_GetComments(t *testing.T) {
	tests := []struct {
		name           string
		slug           string
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "successful get approved comments",
			slug: "test-post",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "test-post", "en").Return(&contentdomain.Content{
					ID:            1,
					Slug:          "test-post",
					Title:         "Test Post",
					AllowComments: true,
				}, nil)
				s.EXPECT().GetCommentsForContent(mock.Anything, 1).Return([]*contentdomain.Comment{
					{
						ID:        1,
						Comment:   "Great article!",
						Author:    "Jane Doe",
						Username:  "janedoe",
						CreatedAt: "2026-04-19T10:30:00Z",
					},
					{
						ID:        2,
						Comment:   "Thanks for sharing",
						Author:    "John Smith",
						Username:  "johnsmith",
						CreatedAt: "2026-04-19T11:00:00Z",
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data field to be an array")
					return
				}
				if len(data) != 2 {
					t.Errorf("expected 2 comments, got %d", len(data))
				}
			},
		},
		{
			name: "empty comments for content with comments disabled",
			slug: "test-post",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "test-post", "en").Return(&contentdomain.Content{
					ID:            1,
					Slug:          "test-post",
					Title:         "Test Post",
					AllowComments: false,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data field to be an array")
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 comments when disabled, got %d", len(data))
				}
			},
		},
		{
			name: "content not found",
			slug: "non-existent",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "non-existent", "en").Return(nil, contentdomain.ErrContentNotFound)
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
			name:           "missing slug parameter",
			slug:           "",
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_slug" {
					t.Errorf("expected code 'invalid_slug', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/public/content_items/"+tt.slug+"/comments", nil)
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("slug", tt.slug)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetComments(w, req)

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

func TestCommentHandler_CreateComment(t *testing.T) {
	tests := []struct {
		name           string
		slug           string
		requestBody    string
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "successful comment submission",
			slug: "test-post",
			requestBody: `{
				"comment": "Great article!"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "test-post", "en").Return(&contentdomain.Content{
					ID:            1,
					Slug:          "test-post",
					Title:         "Test Post",
					AllowComments: true,
				}, nil)
				s.EXPECT().SubmitComment(mock.Anything, 1, userID, mock.Anything).Return(&contentdomain.Comment{
					ID:        1,
					Comment:   "Great article!",
					Status:    contentdomain.CommentStatusPending,
					CreatedAt: "2026-04-19T10:30:00Z",
				}, nil)
			},
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				if data["id"] != float64(1) {
					t.Errorf("expected id 1, got %v", data["id"])
				}
				if data["comment"] != "Great article!" {
					t.Errorf("expected comment 'Great article!', got %v", data["comment"])
				}
			},
		},
		{
			name:           "unauthorized - no user context",
			slug:           "test-post",
			requestBody:    `{"comment": "Great!"}`,
			setupService:   func(s *contentmocks.MockServiceInterface) {},
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
		{
			name: "content not found",
			slug: "non-existent",
			requestBody: `{
				"comment": "Great article!"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "non-existent", "en").Return(nil, contentdomain.ErrContentNotFound)
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
			name:           "comments disabled on content",
			slug:           "test-post",
			requestBody:    `{"comment": "Great!"}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "test-post", "en").Return(&contentdomain.Content{
					ID:            1,
					AllowComments: false,
				}, nil)
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
			name:           "invalid request body",
			slug:           "test-post",
			requestBody:    `invalid json`,
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name:        "empty comment text",
			slug:        "test-post",
			requestBody: `{"comment": "   "}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetPublishedBySlug(mock.Anything, "test-post", "en").Return(&contentdomain.Content{
					ID:            1,
					AllowComments: true,
				}, nil)
				s.EXPECT().SubmitComment(mock.Anything, 1, userID, mock.Anything).Return(nil, contentdomain.ErrInvalidCommentText)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_comment" {
					t.Errorf("expected code 'invalid_comment', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items/"+tt.slug+"/comments", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("slug", tt.slug)

			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			if tt.name != "unauthorized - no user context" {
				ctx = context.WithValue(ctx, middleware.UserIDKey, "1")
			}
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.CreateComment(w, req)

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

func TestCommentHandler_GetCommentsForModeration(t *testing.T) {
	tests := []struct {
		name           string
		contentID      string
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful get all comments for moderation",
			contentID: "1",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetCommentsForModeration(mock.Anything, 1).Return([]*contentdomain.Comment{
					{
						ID:        1,
						Comment:   "Spam comment",
						Author:    "Spammer",
						Username:  "spammer",
						Status:    contentdomain.CommentStatusSpam,
						CreatedAt: "2026-04-19T10:30:00Z",
					},
					{
						ID:        2,
						Comment:   "Pending comment",
						Author:    "User",
						Username:  "user",
						Status:    contentdomain.CommentStatusPending,
						CreatedAt: "2026-04-19T11:00:00Z",
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data field to be an array")
					return
				}
				if len(data) != 2 {
					t.Errorf("expected 2 comments, got %d", len(data))
				}
			},
		},
		{
			name:           "invalid content id",
			contentID:      "invalid",
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_id" {
					t.Errorf("expected code 'invalid_id', got %v", err["code"])
				}
			},
		},
		{
			name:      "empty comments list",
			contentID: "999",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().GetCommentsForModeration(mock.Anything, 999).Return([]*contentdomain.Comment{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].([]any)
				if !ok {
					t.Errorf("expected data field to be an array")
					return
				}
				if len(data) != 0 {
					t.Errorf("expected 0 comments, got %d", len(data))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/content_items/"+tt.contentID+"/comments", nil)
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.contentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetCommentsForModeration(w, req)

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

func TestCommentHandler_UpdateCommentStatus(t *testing.T) {
	tests := []struct {
		name           string
		commentID      string
		requestBody    string
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful approve comment",
			commentID: "1",
			requestBody: `{
				"status": "approved"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, contentdomain.CommentStatusApproved).Return(nil)
				s.EXPECT().GetComment(mock.Anything, 1).Return(&contentdomain.Comment{
					ID:        1,
					Comment:   "Test comment",
					Status:    contentdomain.CommentStatusApproved,
					CreatedAt: "2026-04-20T10:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				if data["id"] != float64(1) {
					t.Errorf("expected id 1, got %v", data["id"])
				}
				if data["status"] != "approved" {
					t.Errorf("expected status approved, got %v", data["status"])
				}
			},
		},
		{
			name:      "successful reject comment",
			commentID: "1",
			requestBody: `{
				"status": "rejected"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, contentdomain.CommentStatusRejected).Return(nil)
				s.EXPECT().GetComment(mock.Anything, 1).Return(&contentdomain.Comment{
					ID:        1,
					Comment:   "Test comment",
					Status:    contentdomain.CommentStatusRejected,
					CreatedAt: "2026-04-20T10:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp map[string]any) {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Errorf("expected data field")
					return
				}
				if data["status"] != "rejected" {
					t.Errorf("expected status rejected, got %v", data["status"])
				}
			},
		},
		{
			name:           "invalid comment id",
			commentID:      "invalid",
			requestBody:    `{"status": "approved"}`,
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_id" {
					t.Errorf("expected code 'invalid_id', got %v", err["code"])
				}
			},
		},
		{
			name:           "invalid request body",
			commentID:      "1",
			requestBody:    `invalid json`,
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_request" {
					t.Errorf("expected code 'invalid_request', got %v", err["code"])
				}
			},
		},
		{
			name:      "comment not found",
			commentID: "999",
			requestBody: `{
				"status": "approved"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 999, contentdomain.CommentStatusApproved).Return(contentdomain.ErrCommentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "comment_not_found" {
					t.Errorf("expected code 'comment_not_found', got %v", err["code"])
				}
			},
		},
		{
			name:      "invalid comment status",
			commentID: "1",
			requestBody: `{
				"status": "invalid_status"
			}`,
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, contentdomain.CommentStatus("invalid_status")).Return(contentdomain.ErrInvalidCommentStatus)
			},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "invalid_status" {
					t.Errorf("expected code 'invalid_status', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/comments/"+tt.commentID+"/status", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.commentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()

			handler.UpdateCommentStatus(w, req)

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

func TestCommentHandler_DeleteComment(t *testing.T) {
	tests := []struct {
		name           string
		commentID      string
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
	}{
		{
			name:      "successful delete comment",
			commentID: "1",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().DeleteComment(mock.Anything, 1).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "invalid comment id",
			commentID:      "invalid",
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "comment not found",
			commentID: "999",
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().DeleteComment(mock.Anything, 999).Return(contentdomain.ErrCommentNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/comments/"+tt.commentID, nil)
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.commentID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.DeleteComment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestCommentHandler_DeleteOwnComment(t *testing.T) {
	tests := []struct {
		name           string
		commentID      string
		setupContext   func(*http.Request) *http.Request
		setupService   func(*contentmocks.MockServiceInterface)
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "successful self-delete",
			commentID: "1",
			setupContext: func(req *http.Request) *http.Request {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", "1")
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				ctx = context.WithValue(ctx, middleware.UserIDKey, "1")
				return req.WithContext(ctx)
			},
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 1, 1).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Body.Len() != 0 {
					t.Errorf("expected empty body for 204, got %s", w.Body.String())
				}
			},
		},
		{
			name:      "delete comment owned by another user",
			commentID: "2",
			setupContext: func(req *http.Request) *http.Request {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", "2")
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				ctx = context.WithValue(ctx, middleware.UserIDKey, "1")
				return req.WithContext(ctx)
			},
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 2, 1).Return(contentdomain.ErrCommentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
					return
				}
				respErr, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if respErr["code"] != "comment_not_found" {
					t.Errorf("expected code 'comment_not_found', got %v", respErr["code"])
				}
			},
		},
		{
			name:      "delete non-existent comment",
			commentID: "999",
			setupContext: func(req *http.Request) *http.Request {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", "999")
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				ctx = context.WithValue(ctx, middleware.UserIDKey, "1")
				return req.WithContext(ctx)
			},
			setupService: func(s *contentmocks.MockServiceInterface) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 999, 1).Return(contentdomain.ErrCommentNotFound)
			},
			expectedStatus: http.StatusNotFound,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
					return
				}
				respErr, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if respErr["code"] != "comment_not_found" {
					t.Errorf("expected code 'comment_not_found', got %v", respErr["code"])
				}
			},
		},
		{
			name:      "unauthenticated request",
			commentID: "1",
			setupContext: func(req *http.Request) *http.Request {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", "1")
				return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			},
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
					return
				}
				respErr, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if respErr["code"] != "unauthorized" {
					t.Errorf("expected code 'unauthorized', got %v", respErr["code"])
				}
			},
		},
		{
			name:      "invalid comment id",
			commentID: "invalid",
			setupContext: func(req *http.Request) *http.Request {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("id", "invalid")
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				ctx = context.WithValue(ctx, middleware.UserIDKey, "1")
				return req.WithContext(ctx)
			},
			setupService:   func(s *contentmocks.MockServiceInterface) {},
			expectedStatus: http.StatusBadRequest,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]any
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
					return
				}
				respErr, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if respErr["code"] != "invalid_id" {
					t.Errorf("expected code 'invalid_id', got %v", respErr["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := contentmocks.NewMockServiceInterface(t)
			tt.setupService(mockService)

			handler := handlers.NewCommentHandler(mockService)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/my-comments/"+tt.commentID, nil)
			req = tt.setupContext(req)

			w := httptest.NewRecorder()

			handler.DeleteOwnComment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.validateResp != nil {
				tt.validateResp(t, w)
			}
		})
	}
}
