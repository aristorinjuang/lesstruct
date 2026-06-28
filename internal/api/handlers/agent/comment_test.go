package agent_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent/mocks"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newCommentHandler builds a CommentHandler with a discard logger so handler
// unit tests do not require stdout wiring (mirrors newContentHandler).
func newCommentHandler(svc agent.CommentService) *agent.CommentHandler {
	return agent.NewCommentHandler(svc, util.NewLogger(io.Discard))
}

// envelopeDataCommentID decodes the envelope and returns the id nested under
// data.comment.id (0 when absent). Mirrors envelopeDataContentID for the
// comment response wrapper.
func envelopeDataCommentID(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp struct {
		Data struct {
			Comment struct {
				ID int `json:"id"`
			} `json:"comment"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "failed to decode response envelope")
	return resp.Data.Comment.ID
}

// envelopeDataCommentStatus decodes data.comment.status ("" when absent), used
// to assert the moderation status the UpdateStatus handler echoes back.
func envelopeDataCommentStatus(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Data struct {
			Comment struct {
				Status string `json:"status"`
			} `json:"comment"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "failed to decode response envelope")
	return resp.Data.Comment.Status
}

// buildComment builds a *contentdomain.Comment fixture with the given fields.
func buildComment(id, contentID, userID int, status contentdomain.CommentStatus, text string) *contentdomain.Comment {
	return &contentdomain.Comment{
		ID:        id,
		ContentID: contentID,
		UserID:    userID,
		Comment:   text,
		Status:    status,
		Author:    "Author " + strconv.Itoa(userID),
		Username:  "user" + strconv.Itoa(userID),
		CreatedAt: "2026-06-23T12:00:00Z",
	}
}

func TestCommentHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		contentID  string
		body       string
		userID     int
		role       string
		withUser   bool
		setup      func(svc *agentmocks.MockCommentService)
		wantStatus int
		wantCode   string
		wantCommID int // expected data.comment.id (0 = skip)
	}{
		{
			name:       "success - published content with comments enabled creates a pending comment",
			contentID:  "5",
			body:       marshalJSON(map[string]any{"comment": "Nice post!"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusOK,
			wantCommID: 101,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: true,
				}, nil)
				svc.EXPECT().SubmitComment(mock.Anything, 5, testUserID, mock.Anything).
					Return(buildComment(101, 5, testUserID, contentdomain.CommentStatusPending, "Nice post!"), nil)
			},
		},
		{
			name:       "success - owner can comment on their own draft that allows comments",
			contentID:  "6",
			body:       marshalJSON(map[string]any{"comment": "Self note"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusOK,
			wantCommID: 102,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 6).Return(&contentdomain.Content{
					ID: 6, UserID: testUserID, Status: contentdomain.StatusDraft, AllowComments: true,
				}, nil)
				svc.EXPECT().SubmitComment(mock.Anything, 6, testUserID, mock.Anything).
					Return(buildComment(102, 6, testUserID, contentdomain.CommentStatusPending, "Self note"), nil)
			},
		},
		{
			name:       "error - comments disabled returns FORBIDDEN before SubmitComment",
			contentID:  "5",
			body:       marshalJSON(map[string]any{"comment": "hi"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: false,
				}, nil)
				// SubmitComment must NOT be called.
			},
		},
		{
			name:       "error - another user's draft returns NOT_FOUND (existence not disclosed)",
			contentID:  "7",
			body:       marshalJSON(map[string]any{"comment": "hi"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 7).Return(&contentdomain.Content{
					ID: 7, UserID: 999, Status: contentdomain.StatusDraft, AllowComments: true,
				}, nil)
			},
		},
		{
			name:       "error - content not found returns NOT_FOUND",
			contentID:  "5",
			body:       marshalJSON(map[string]any{"comment": "hi"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, contentdomain.ErrContentNotFound)
			},
		},
		{
			name:       "error - invalid comment text returns VALIDATION_ERROR",
			contentID:  "5",
			body:       marshalJSON(map[string]any{"comment": ""}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: true,
				}, nil)
				svc.EXPECT().SubmitComment(mock.Anything, 5, testUserID, mock.Anything).
					Return(nil, contentdomain.ErrInvalidCommentText)
			},
		},
		{
			name:       "error - malformed JSON body returns VALIDATION_ERROR",
			contentID:  "5",
			body:       "{bad json",
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
			setup:      nil, // GetByID must NOT be reached
		},
		{
			name:       "error - invalid content id returns VALIDATION_ERROR",
			contentID:  "0",
			body:       marshalJSON(map[string]any{"comment": "hi"}),
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
			setup:      nil,
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			contentID:  "5",
			body:       marshalJSON(map[string]any{"comment": "hi"}),
			withUser:   false,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
			setup:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newCommentHandler(svc)
			target := "/api/v1/content/" + tt.contentID + "/comments"
			var r *http.Request
			if tt.withUser {
				r = newAuthenticatedRequestAs(http.MethodPost, target, tt.body, tt.userID, tt.role)
			} else {
				r = newAuthenticatedRequest(http.MethodPost, target, tt.body, false)
			}
			r.SetPathValue("id", tt.contentID)
			w := httptest.NewRecorder()
			handler.Create(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantCommID != 0 {
				assert.Equal(t, tt.wantCommID, envelopeDataCommentID(t, w), "envelope data.comment.id")
			}
		})
	}
}

func TestCommentHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		contentID  string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockCommentService)
		wantStatus int
		wantCode   string
		wantLen    int
		wantBody   string // substring expected in the body ("" = skip)
	}{
		{
			name:      "success - returns all comments (any status) for visible content",
			contentID: "5",
			userID:    testUserID,
			role:      contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: true,
				}, nil)
				// Admin sees the moderation queue: every status.
				svc.EXPECT().GetCommentsForModeration(mock.Anything, 5).Return([]*contentdomain.Comment{
					buildComment(1, 5, 11, contentdomain.CommentStatusApproved, "ok"),
					buildComment(2, 5, 12, contentdomain.CommentStatusPending, "waiting"),
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    2,
		},
		{
			name:      "success - non-admin sees only approved comments (no moderation queue)",
			contentID: "5",
			userID:    testUserID,
			role:      "Commentator",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: true,
				}, nil)
				// GetCommentsForModeration must NOT be called — the non-admin path uses the
				// approved-only reader, so the pending/spam queue and its author/role metadata
				// are never exposed to a Commentator-level key.
				svc.EXPECT().GetCommentsForContent(mock.Anything, 5).Return([]*contentdomain.Comment{
					buildComment(1, 5, 11, contentdomain.CommentStatusApproved, "ok"),
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
		},
		{
			name:      "success - empty list renders data as an empty array",
			contentID: "5",
			userID:    testUserID,
			role:      "",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: testUserID, Status: contentdomain.StatusPublished, AllowComments: true,
				}, nil)
				svc.EXPECT().GetCommentsForContent(mock.Anything, 5).Return([]*contentdomain.Comment{}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
			wantBody:   `"data":[]`,
		},
		{
			name:      "success - non-admin on comments-disabled content gets an empty list",
			contentID: "4",
			userID:    testUserID,
			role:      "Commentator",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 4).Return(&contentdomain.Content{
					ID: 4, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: false,
				}, nil)
				// No comment reader must be called — AllowComments=false short-circuits to empty.
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
			wantBody:   `"data":[]`,
		},
		{
			name:      "success - admin still sees the moderation queue on comments-disabled content",
			contentID: "4",
			userID:    testUserID,
			role:      contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 4).Return(&contentdomain.Content{
					ID: 4, UserID: 999, Status: contentdomain.StatusPublished, AllowComments: false,
				}, nil)
				// Admin bypasses the AllowComments gate and moderates regardless.
				svc.EXPECT().GetCommentsForModeration(mock.Anything, 4).Return([]*contentdomain.Comment{
					buildComment(1, 4, 11, contentdomain.CommentStatusApproved, "ok"),
					buildComment(2, 4, 12, contentdomain.CommentStatusPending, "waiting"),
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    2,
		},
		{
			name:      "error - content not found returns NOT_FOUND",
			contentID: "5",
			userID:    testUserID,
			role:      "",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - another user's draft returns NOT_FOUND (no disclosure)",
			contentID: "7",
			userID:    testUserID,
			role:      "",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetByID(mock.Anything, 7).Return(&contentdomain.Content{
					ID: 7, UserID: 999, Status: contentdomain.StatusDraft, AllowComments: true,
				}, nil)
				// No comment reader must be called.
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "error - invalid content id returns VALIDATION_ERROR",
			contentID:  "abc",
			userID:     testUserID,
			role:       "",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newCommentHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodGet, "/api/v1/content/"+tt.contentID+"/comments", "", tt.userID, tt.role)
			r.SetPathValue("id", tt.contentID)
			w := httptest.NewRecorder()
			handler.List(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
				return
			}
			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}
			var env struct {
				Data []map[string]any `json:"data"`
			}
			require.NoError(t, json.NewDecoder(w.Body).Decode(&env))
			assert.Len(t, env.Data, tt.wantLen)
		})
	}
}

func TestCommentHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		contentID  string
		commentID  string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockCommentService)
		wantStatus int
		wantCode   string
	}{
		{
			name:      "success - admin deletes any comment returns 204",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, contentdomain.CommentStatusPending, "hi"), nil)
				svc.EXPECT().DeleteComment(mock.Anything, 9).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:      "success - owner deletes own comment returns 204",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      "",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, testUserID, contentdomain.CommentStatusPending, "hi"), nil)
				svc.EXPECT().DeleteOwnComment(mock.Anything, 9, testUserID).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:      "error - non-owner non-admin delete returns NOT_FOUND (no disclosure)",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      "",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 999, contentdomain.CommentStatusPending, "hi"), nil)
				svc.EXPECT().DeleteOwnComment(mock.Anything, 9, testUserID).Return(contentdomain.ErrCommentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - admin delete missing comment returns NOT_FOUND",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockCommentService) {
				// The missing comment fails the binding fetch before DeleteComment is reached.
				svc.EXPECT().GetComment(mock.Anything, 9).Return(nil, contentdomain.ErrCommentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - comment bound to different content returns NOT_FOUND (no disclosure)",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 8, 11, contentdomain.CommentStatusPending, "hi"), nil)
				// DeleteComment must NOT be called — the path content id does not match.
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "error - invalid comment id returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "0",
			userID:     testUserID,
			role:       "",
			setup:      nil, // GetComment must NOT be reached
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - invalid content id returns VALIDATION_ERROR",
			contentID:  "0",
			commentID:  "9",
			userID:     testUserID,
			role:       "",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newCommentHandler(svc)
			r := newAuthenticatedRequestAs(
				http.MethodDelete,
				"/api/v1/content/"+tt.contentID+"/comments/"+tt.commentID,
				"",
				tt.userID,
				tt.role,
			)
			r.SetPathValue("id", tt.contentID)
			r.SetPathValue("commentId", tt.commentID)
			w := httptest.NewRecorder()
			handler.Delete(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusNoContent {
				assert.Empty(t, w.Body.String(), "204 must have an empty body")
				return
			}
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
		})
	}
}

func TestCommentHandler_UpdateStatus(t *testing.T) {
	tests := []struct {
		name              string
		contentID         string
		commentID         string
		role              string
		body              string
		setup             func(svc *agentmocks.MockCommentService)
		wantStatus        int
		wantCode          string
		wantCommID        int    // expected data.comment.id (0 = skip)
		wantCommentStatus string // expected data.comment.status ("" = skip)
	}{
		{
			name:              "admin success - approves a comment and echoes the new status",
			contentID:         "5",
			commentID:         "9",
			role:              contentdomain.RoleAdmin,
			body:              marshalJSON(map[string]any{"status": "approved"}),
			wantStatus:        http.StatusOK,
			wantCommID:        9,
			wantCommentStatus: "approved",
			setup: func(svc *agentmocks.MockCommentService) {
				// Binding fetch (must match the path content id) …
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, contentdomain.CommentStatusPending, "ok"), nil).Once()
				svc.EXPECT().UpdateCommentStatus(mock.Anything, 9, contentdomain.CommentStatusApproved).Return(nil)
				// … then the authoritative re-fetch for the response.
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, contentdomain.CommentStatusApproved, "ok"), nil).Once()
			},
		},
		{
			name:       "error - comment bound to different content returns NOT_FOUND (no disclosure)",
			contentID:  "5",
			commentID:  "9",
			role:       contentdomain.RoleAdmin,
			body:       marshalJSON(map[string]any{"status": "approved"}),
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 8, 11, contentdomain.CommentStatusPending, "ok"), nil)
				// UpdateCommentStatus must NOT be called — the path content id does not match.
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "non-admin role is forbidden",
			contentID:  "5",
			commentID:  "9",
			role:       "Commentator",
			body:       marshalJSON(map[string]any{"status": "approved"}),
			setup:      nil, // service must NOT be called
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "missing role is forbidden",
			contentID:  "5",
			commentID:  "9",
			role:       "",
			body:       marshalJSON(map[string]any{"status": "approved"}),
			setup:      nil,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "invalid status returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "9",
			role:       contentdomain.RoleAdmin,
			body:       marshalJSON(map[string]any{"status": "bogus"}),
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, contentdomain.CommentStatusPending, "ok"), nil)
				svc.EXPECT().UpdateCommentStatus(mock.Anything, 9, contentdomain.CommentStatus("bogus")).
					Return(contentdomain.ErrInvalidCommentStatus)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "malformed body returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "9",
			role:       contentdomain.RoleAdmin,
			body:       "{bad",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "invalid comment id returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "0",
			role:       contentdomain.RoleAdmin,
			body:       marshalJSON(map[string]any{"status": "approved"}),
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newCommentHandler(svc)
			r := newAuthenticatedRequestAs(
				http.MethodPut,
				"/api/v1/content/"+tt.contentID+"/comments/"+tt.commentID+"/status",
				tt.body,
				testUserID,
				tt.role,
			)
			r.SetPathValue("id", tt.contentID)
			r.SetPathValue("commentId", tt.commentID)
			w := httptest.NewRecorder()
			handler.UpdateStatus(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
				return
			}
			if tt.wantCommID != 0 {
				assert.Equal(t, tt.wantCommID, envelopeDataCommentID(t, w), "envelope data.comment.id")
			}
			if tt.wantCommentStatus != "" {
				assert.Equal(t, tt.wantCommentStatus, envelopeDataCommentStatus(t, w), "envelope data.comment.status")
			}
		})
	}
}
