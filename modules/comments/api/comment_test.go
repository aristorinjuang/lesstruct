package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/comments/api"
	apimocks "github.com/aristorinjuang/lesstruct/modules/comments/api/mocks"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

// stubRoles is a test-only role registry granting the comment capability to
// a fixed set of roles. It satisfies api.RoleCapabilities without the core
// role service, which the module cannot import.
type stubRoles struct {
	comments map[string]bool
}

func (s *stubRoles) CanComment(name string) bool {
	return s.comments[name]
}

func staticResolver(ref api.ContentRef, err error) api.ContentResolver {
	return func(context.Context, string, string) (api.ContentRef, error) {
		return ref, err
	}
}

func mustNotResolve(t *testing.T) api.ContentResolver {
	t.Helper()
	return func(context.Context, string, string) (api.ContentRef, error) {
		t.Fatal("content resolver must not be called")
		return api.ContentRef{}, nil
	}
}

func TestCommentHandler_GetComments(t *testing.T) {
	tests := []struct {
		name           string
		slug           string
		allowComments  bool
		contentMissing bool
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:          "successful get approved comments",
			slug:          "test-post",
			allowComments: true,
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsForContent(mock.Anything, 1).Return([]*commentdomain.Comment{
					{
						ID:        1,
						Comment:   "Great article!",
						Author:    "Jane Doe",
						Username:  "janedoe",
						CreatedAt: time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
					},
					{
						ID:        2,
						Comment:   "Thanks for sharing",
						Author:    "John Smith",
						Username:  "johnsmith",
						CreatedAt: time.Date(2026, 4, 19, 11, 0, 0, 0, time.UTC),
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
			name:           "empty comments for content with comments disabled",
			slug:           "test-post",
			allowComments:  false,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			name:           "content not found",
			slug:           "non-existent",
			contentMissing: true,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			var resolver api.ContentResolver
			switch {
			case tt.slug == "":
				resolver = mustNotResolve(t)
			case tt.contentMissing:
				resolver = staticResolver(api.ContentRef{}, api.ErrContentNotFound)
			default:
				resolver = staticResolver(api.ContentRef{ID: 1, AllowComments: tt.allowComments}, nil)
			}

			handler := api.NewCommentHandler(mockComments, resolver)

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
		allowComments  bool
		contentMissing bool
		noResolve      bool
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:          "successful comment submission",
			slug:          "test-post",
			allowComments: true,
			requestBody: `{
				"comment": "Great article!"
			}`,
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().SubmitComment(mock.Anything, 1, mock.Anything, mock.Anything).Return(&commentdomain.Comment{
					ID:        1,
					Comment:   "Great article!",
					Status:    commentdomain.CommentStatusPending,
					CreatedAt: time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
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
			noResolve:      true,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			name:           "content not found",
			slug:           "non-existent",
			contentMissing: true,
			requestBody: `{
				"comment": "Great article!"
			}`,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			allowComments:  false,
			requestBody:    `{"comment": "Great!"}`,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			noResolve:      true,
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			name:          "empty comment text",
			slug:          "test-post",
			allowComments: true,
			requestBody:   `{"comment": "   "}`,
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().SubmitComment(mock.Anything, 1, mock.Anything, mock.Anything).Return(nil, commentdomain.ErrInvalidCommentText)
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
		{
			name:          "content deleted after lookup maps to content_not_found",
			slug:          "test-post",
			allowComments: true,
			requestBody:   `{"comment": "Great article!"}`,
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().SubmitComment(mock.Anything, 1, mock.Anything, mock.Anything).Return(
					nil,
					fmt.Errorf("failed to get content: %w", commentdomain.ErrContentNotFound),
				)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			var resolver api.ContentResolver
			switch {
			case tt.noResolve:
				resolver = mustNotResolve(t)
			case tt.contentMissing:
				resolver = staticResolver(api.ContentRef{}, api.ErrContentNotFound)
			default:
				resolver = staticResolver(api.ContentRef{ID: 1, AllowComments: tt.allowComments}, nil)
			}

			handler := api.NewCommentHandler(mockComments, resolver)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items/"+tt.slug+"/comments", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("slug", tt.slug)

			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			if tt.name != "unauthorized - no user context" {
				ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser"})
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

// TestCommentHandler_CreateComment_RoleGate verifies posting a comment is gated
// by the caller's role capability when a role service is configured. A role
// without the comments capability gets 403; a role with it proceeds to the
// comment service. Without a role service, every authenticated user may comment
// (legacy behavior, covered by TestCommentHandler_CreateComment).
func TestCommentHandler_CreateComment_RoleGate(t *testing.T) {
	roleService := &stubRoles{comments: map[string]bool{}}
	roleServiceWithComments := &stubRoles{comments: map[string]bool{"Commentator": true}}

	tests := []struct {
		name           string
		role           string
		withRoleSvc    bool
		roleSvc        *stubRoles
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:           "role cannot comment",
			role:           "Journalist",
			withRoleSvc:    true,
			roleSvc:        roleService,
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
			name:           "role can comment",
			role:           "Commentator",
			withRoleSvc:    true,
			roleSvc:        roleServiceWithComments,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "no role service - legacy allows",
			role:           "Journalist",
			withRoleSvc:    false,
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockComments := apimocks.NewMockCommentService(t)
			if tt.expectedStatus == http.StatusCreated {
				mockComments.EXPECT().SubmitComment(mock.Anything, 1, mock.Anything, mock.Anything).Return(&commentdomain.Comment{
					ID:        1,
					Comment:   "Great!",
					Status:    commentdomain.CommentStatusPending,
					CreatedAt: time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
				}, nil)
			}

			var opts []api.CommentHandlerOption
			if tt.withRoleSvc {
				opts = append(opts, api.WithCommentRoleService(tt.roleSvc))
			}
			handler := api.NewCommentHandler(
				mockComments,
				staticResolver(api.ContentRef{ID: 1, AllowComments: true}, nil),
				opts...,
			)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/content_items/test-post/comments", bytes.NewReader([]byte(`{"comment": "Great!"}`)))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("slug", "test-post")

			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser", Roles: []string{tt.role}})
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.CreateComment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestCommentHandler_GetCommentsForModeration(t *testing.T) {
	tests := []struct {
		name           string
		contentID      string
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful get all comments for moderation",
			contentID: "1",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsForModeration(mock.Anything, 1).Return([]*commentdomain.Comment{
					{
						ID:        1,
						Comment:   "Spam comment",
						Author:    "Spammer",
						Username:  "spammer",
						Status:    commentdomain.CommentStatusSpam,
						CreatedAt: time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
					},
					{
						ID:        2,
						Comment:   "Pending comment",
						Author:    "User",
						Username:  "user",
						Status:    commentdomain.CommentStatusPending,
						CreatedAt: time.Date(2026, 4, 19, 11, 0, 0, 0, time.UTC),
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
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsForModeration(mock.Anything, 999).Return([]*commentdomain.Comment{}, nil)
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
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

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

func TestCommentHandler_GetPendingComments(t *testing.T) {
	tests := []struct {
		name           string
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "successful get pending comments with content context",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsByStatus(mock.Anything, commentdomain.CommentStatusPending).Return([]*commentdomain.Comment{
					{
						ID:           1,
						ContentID:    10,
						ContentTitle: "Getting Started with Go",
						ContentSlug:  "getting-started-with-go",
						Comment:      "Great article!",
						Author:       "Jane Doe",
						Username:     "janedoe",
						Status:       commentdomain.CommentStatusPending,
						CreatedAt:    time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
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
				if len(data) != 1 {
					t.Errorf("expected 1 pending comment, got %d", len(data))
					return
				}
				first := data[0].(map[string]any)
				if first["contentTitle"] != "Getting Started with Go" {
					t.Errorf("expected contentTitle 'Getting Started with Go', got %v", first["contentTitle"])
				}
				if first["contentSlug"] != "getting-started-with-go" {
					t.Errorf("expected contentSlug 'getting-started-with-go', got %v", first["contentSlug"])
				}
			},
		},
		{
			name: "empty pending comments list",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsByStatus(mock.Anything, commentdomain.CommentStatusPending).Return([]*commentdomain.Comment{}, nil)
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
		{
			name: "service error returns internal error",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsByStatus(mock.Anything, commentdomain.CommentStatusPending).Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateResp: func(t *testing.T, resp map[string]any) {
				err, ok := resp["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error field")
					return
				}
				if err["code"] != "internal_error" {
					t.Errorf("expected code 'internal_error', got %v", err["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/comments/pending", nil)
			w := httptest.NewRecorder()

			handler.GetPendingComments(w, req)

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
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name:      "successful approve comment",
			commentID: "1",
			requestBody: `{
				"status": "approved"
			}`,
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, commentdomain.CommentStatusApproved).Return(nil)
				s.EXPECT().GetComment(mock.Anything, 1).Return(&commentdomain.Comment{
					ID:        1,
					Comment:   "Test comment",
					Status:    commentdomain.CommentStatusApproved,
					CreatedAt: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
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
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, commentdomain.CommentStatusRejected).Return(nil)
				s.EXPECT().GetComment(mock.Anything, 1).Return(&commentdomain.Comment{
					ID:        1,
					Comment:   "Test comment",
					Status:    commentdomain.CommentStatusRejected,
					CreatedAt: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
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
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 999, commentdomain.CommentStatusApproved).Return(commentdomain.ErrCommentNotFound)
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
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().UpdateCommentStatus(mock.Anything, 1, commentdomain.CommentStatus("invalid_status")).Return(commentdomain.ErrInvalidCommentStatus)
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
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

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
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
	}{
		{
			name:      "successful delete comment",
			commentID: "1",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().DeleteComment(mock.Anything, 1).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "invalid comment id",
			commentID:      "invalid",
			setupComments:  func(s *apimocks.MockCommentService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "comment not found",
			commentID: "999",
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().DeleteComment(mock.Anything, 999).Return(commentdomain.ErrCommentNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

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
		setupComments  func(*apimocks.MockCommentService)
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
				ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser"})
				return req.WithContext(ctx)
			},
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 1, mock.Anything).Return(nil)
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
				ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser"})
				return req.WithContext(ctx)
			},
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 2, mock.Anything).Return(commentdomain.ErrCommentNotFound)
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
				ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser"})
				return req.WithContext(ctx)
			},
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().DeleteOwnComment(mock.Anything, 999, mock.Anything).Return(commentdomain.ErrCommentNotFound)
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
			setupComments:  func(s *apimocks.MockCommentService) {},
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
				ctx = kernel.ContextWithPrincipal(ctx, kernel.Principal{ID: "1", Username: "testuser"})
				return req.WithContext(ctx)
			},
			setupComments:  func(s *apimocks.MockCommentService) {},
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
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

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

func TestCommentHandler_GetMyComments(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		setupContext   func(*http.Request) *http.Request
		setupComments  func(*apimocks.MockCommentService)
		expectedStatus int
		wantTotal      int
		validateResp   func(*testing.T, map[string]any)
	}{
		{
			name: "regular user gets own comment count",
			role: "Author",
			setupContext: func(req *http.Request) *http.Request {
				ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{
					ID:       "7",
					Username: "testuser",
					Roles:    []string{"Author"},
				})
				return req.WithContext(ctx)
			},
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsByUserID(mock.Anything, 7).Return([]*commentdomain.Comment{
					{
						ID:        1,
						Comment:   "My comment",
						Status:    commentdomain.CommentStatusPending,
						CreatedAt: time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
					},
				}, nil)
				s.EXPECT().CountComments(mock.Anything, 7).Return(3, nil)
			},
			expectedStatus: http.StatusOK,
			wantTotal:      3,
		},
		{
			name: "admin gets global comment count",
			role: "Admin",
			setupContext: func(req *http.Request) *http.Request {
				ctx := kernel.ContextWithPrincipal(req.Context(), kernel.Principal{
					ID:       "7",
					Username: "testuser",
					Roles:    []string{"Admin"},
				})
				return req.WithContext(ctx)
			},
			setupComments: func(s *apimocks.MockCommentService) {
				s.EXPECT().GetCommentsByUserID(mock.Anything, 7).Return([]*commentdomain.Comment{}, nil)
				s.EXPECT().CountComments(mock.Anything, 0).Return(42, nil)
			},
			expectedStatus: http.StatusOK,
			wantTotal:      42,
		},
		{
			name: "unauthenticated request returns 401",
			setupContext: func(req *http.Request) *http.Request {
				return req
			},
			setupComments:  func(s *apimocks.MockCommentService) {},
			expectedStatus: http.StatusUnauthorized,
			validateResp: func(t *testing.T, resp map[string]any) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockComments := apimocks.NewMockCommentService(t)
			tt.setupComments(mockComments)

			handler := api.NewCommentHandler(mockComments, mustNotResolve(t))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/my-comments", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}

			w := httptest.NewRecorder()

			handler.GetMyComments(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("failed to decode response: %v", err)
				return
			}

			if tt.wantTotal > 0 {
				meta, ok := resp["meta"].(map[string]any)
				if !ok {
					t.Errorf("expected meta field")
					return
				}
				pagination, ok := meta["pagination"].(map[string]any)
				if !ok {
					t.Errorf("expected pagination field")
					return
				}
				total, ok := pagination["total"].(float64)
				if !ok {
					t.Errorf("expected total field")
					return
				}
				if int(total) != tt.wantTotal {
					t.Errorf("expected total %d, got %v", tt.wantTotal, total)
				}
			}

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}
