package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	commentagent "github.com/aristorinjuang/lesstruct/modules/comments/api/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/modules/comments/api/agent/mocks"
	commentdomain "github.com/aristorinjuang/lesstruct/modules/comments/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testUserID is the owning user injected into request context for the
// authenticated test cases.
const testUserID = 42

// stubRoles is a test-only role registry granting the comment capability to
// a fixed set of roles. It satisfies agent.RoleCapabilities without the core
// role service, which the module cannot import.
type stubRoles struct {
	comments map[string]bool
}

func (s *stubRoles) CanComment(name string) bool {
	return s.comments[name]
}

// newCommentHandler builds a CommentHandler with a discard logger so handler
// unit tests do not require stdout wiring.
func newCommentHandler(svc commentagent.CommentService, content commentagent.ContentLookup) *commentagent.CommentHandler {
	if content == nil {
		content = mustNotLookup(nil)
	}
	return commentagent.NewCommentHandler(svc, content, nil)
}

func mustNotLookup(t *testing.T) commentagent.ContentLookup {
	return func(context.Context, int) (commentagent.ContentDetail, error) {
		if t != nil {
			t.Fatal("content lookup must not be called")
		}
		panic("content lookup must not be called")
	}
}

func staticLookup(detail commentagent.ContentDetail, err error) commentagent.ContentLookup {
	return func(context.Context, int) (commentagent.ContentDetail, error) {
		return detail, err
	}
}

// newAuthenticatedRequest builds a request whose body is body, and — when
// withUser is true — injects testUserID via the kernel principal, mirroring
// how the Bearer middleware writes identity in production.
func newAuthenticatedRequest(method, target, body string, withUser bool) *http.Request {
	r := httptest.NewRequest(method, target, jsonBody(body))
	if withUser {
		ctx := kernel.ContextWithPrincipal(r.Context(), kernel.Principal{
			ID:       strconv.Itoa(testUserID),
			Username: "testuser",
		})
		r = r.WithContext(ctx)
	}
	return r
}

// newAuthenticatedRequestAs builds a request whose body is body, injecting
// the given userID (and role, when non-empty) via the kernel principal —
// mirroring how the Bearer middleware writes identity in production.
func newAuthenticatedRequestAs(method, target, body string, userID int, role string) *http.Request {
	r := httptest.NewRequest(method, target, jsonBody(body))
	roles := make([]string, 0, 1)
	if role != "" {
		roles = append(roles, role)
	}
	ctx := kernel.ContextWithPrincipal(r.Context(), kernel.Principal{
		ID:       strconv.Itoa(userID),
		Username: "testuser",
		Roles:    roles,
	})
	return r.WithContext(ctx)
}

func jsonBody(body string) *strings.Reader {
	return strings.NewReader(body)
}

// envelopeError decodes the response envelope and returns the error code (""
// when the response carries no error).
func envelopeError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error map[string]any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "failed to decode response envelope")
	code, _ := resp.Error["code"].(string)
	return code
}

// envelopeMessage decodes the response envelope and returns the error
// message ("" when the response carries no error).
func envelopeMessage(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error map[string]any `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "failed to decode response envelope")
	msg, _ := resp.Error["message"].(string)
	return msg
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

// buildComment builds a *commentdomain.Comment fixture with the given fields.
func buildComment(id, contentID, userID int, status commentdomain.CommentStatus, text string) *commentdomain.Comment {
	return &commentdomain.Comment{
		ID:        id,
		ContentID: contentID,
		UserID:    userID,
		Comment:   text,
		Status:    status,
		Author:    "Author " + strconv.Itoa(userID),
		Username:  "user" + strconv.Itoa(userID),
		CreatedAt: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC),
	}
}

func publishedContent(id, userID int, allowComments bool) commentagent.ContentDetail {
	return commentagent.ContentDetail{ID: id, UserID: userID, Published: true, AllowComments: allowComments}
}

func TestCommentHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		contentID  string
		body       string
		userID     int
		role       string
		withUser   bool
		lookup     func() (commentagent.ContentDetail, error)
		setup      func(svc *agentmocks.MockCommentService)
		wantStatus int
		wantCode   string
		wantMsg    string // expected error message ("" = skip)
		wantCommID int    // expected data.comment.id (0 = skip)
	}{
		{
			name:       "success - published content with comments enabled creates a pending comment",
			contentID:  "5",
			body:       `{"comment": "Nice post!"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusOK,
			wantCommID: 101,
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().SubmitComment(mock.Anything, 5, mock.Anything, mock.Anything).
					Return(buildComment(101, 5, testUserID, commentdomain.CommentStatusPending, "Nice post!"), nil)
			},
		},
		{
			name:       "success - owner can comment on their own draft that allows comments",
			contentID:  "6",
			body:       `{"comment": "Self note"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusOK,
			wantCommID: 102,
			lookup: func() (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{ID: 6, UserID: testUserID, Published: false, AllowComments: true}, nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().SubmitComment(mock.Anything, 6, mock.Anything, mock.Anything).
					Return(buildComment(102, 6, testUserID, commentdomain.CommentStatusPending, "Self note"), nil)
			},
		},
		{
			name:       "error - comments disabled returns FORBIDDEN before SubmitComment",
			contentID:  "5",
			body:       `{"comment": "hi"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, false), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				// SubmitComment must NOT be called.
			},
		},
		{
			name:       "error - another user's draft returns NOT_FOUND (existence not disclosed)",
			contentID:  "7",
			body:       `{"comment": "hi"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			lookup: func() (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{ID: 7, UserID: 999, Published: false, AllowComments: true}, nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
			},
		},
		{
			name:       "error - content not found returns NOT_FOUND",
			contentID:  "5",
			body:       `{"comment": "hi"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			lookup: func() (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{}, commentagent.ErrContentNotFound
			},
			setup: func(svc *agentmocks.MockCommentService) {
			},
		},
		{
			name:       "error - invalid comment text returns VALIDATION_ERROR",
			contentID:  "5",
			body:       `{"comment": ""}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().SubmitComment(mock.Anything, 5, mock.Anything, mock.Anything).
					Return(nil, commentdomain.ErrInvalidCommentText)
			},
		},
		{
			name:       "error - content deleted after lookup returns NOT_FOUND",
			contentID:  "5",
			body:       `{"comment": "hi"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
			wantMsg:    "Content not found",
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().SubmitComment(mock.Anything, 5, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("failed to get content: %w", commentdomain.ErrContentNotFound))
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
			lookup:     nil, // lookup must NOT be reached
			setup:      nil,
		},
		{
			name:       "error - invalid content id returns VALIDATION_ERROR",
			contentID:  "0",
			body:       `{"comment": "hi"}`,
			userID:     testUserID,
			role:       "",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
			lookup:     nil,
			setup:      nil,
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			contentID:  "5",
			body:       `{"comment": "hi"}`,
			withUser:   false,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
			lookup:     nil,
			setup:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			var lookup commentagent.ContentLookup
			if tt.lookup != nil {
				detail, err := tt.lookup()
				lookup = staticLookup(detail, err)
			}
			handler := newCommentHandler(svc, lookup)
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
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, envelopeMessage(t, w))
			}
			if tt.wantCommID != 0 {
				assert.Equal(t, tt.wantCommID, envelopeDataCommentID(t, w), "envelope data.comment.id")
			}
		})
	}
}

// TestCommentHandler_Create_RoleGate verifies the agent comment-create endpoint
// is gated by the caller's role capability when a role service is configured. A
// role without the comments capability gets FORBIDDEN before any service call;
// without a role service every authenticated user may comment (legacy behavior,
// covered by TestCommentHandler_Create).
func TestCommentHandler_Create_RoleGate(t *testing.T) {
	roleService := &stubRoles{comments: map[string]bool{}}
	roleServiceWithComments := &stubRoles{comments: map[string]bool{"Commentator": true}}

	tests := []struct {
		name         string
		role         string
		withRoleSvc  bool
		roleSvc      *stubRoles
		wantStatus   int
		wantCode     string
		wantCommID   int
		expectSubmit bool
	}{
		{
			name:         "role cannot comment is forbidden",
			role:         "Journalist",
			withRoleSvc:  true,
			roleSvc:      roleService,
			wantStatus:   http.StatusForbidden,
			wantCode:     "FORBIDDEN",
			expectSubmit: false,
		},
		{
			name:         "role can comment succeeds",
			role:         "Commentator",
			withRoleSvc:  true,
			roleSvc:      roleServiceWithComments,
			wantStatus:   http.StatusOK,
			wantCommID:   101,
			expectSubmit: true,
		},
		{
			name:         "no role service preserves legacy behavior",
			role:         "Journalist",
			withRoleSvc:  false,
			wantStatus:   http.StatusOK,
			wantCommID:   102,
			expectSubmit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockCommentService(t)
			if tt.expectSubmit {
				svc.EXPECT().SubmitComment(mock.Anything, 5, mock.Anything, mock.Anything).
					Return(buildComment(tt.wantCommID, 5, testUserID, commentdomain.CommentStatusPending, "hi"), nil)
			}

			var opts []commentagent.CommentHandlerOption
			if tt.withRoleSvc {
				opts = append(opts, commentagent.WithCommentRoleService(tt.roleSvc))
			}
			handler := commentagent.NewCommentHandler(
				svc,
				staticLookup(publishedContent(5, 999, true), nil),
				nil,
				opts...,
			)

			r := newAuthenticatedRequestAs(http.MethodPost, "/api/v1/content/5/comments", `{"comment": "hi"}`, testUserID, tt.role)
			r.SetPathValue("id", "5")
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
		lookup     func() (commentagent.ContentDetail, error)
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
			role:      "Admin",
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				// Admin sees the moderation queue: every status.
				svc.EXPECT().GetCommentsForModeration(mock.Anything, 5).Return([]*commentdomain.Comment{
					buildComment(1, 5, 11, commentdomain.CommentStatusApproved, "ok"),
					buildComment(2, 5, 12, commentdomain.CommentStatusPending, "waiting"),
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
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, 999, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				// GetCommentsForModeration must NOT be called — the non-admin path uses the
				// approved-only reader, so the pending/spam queue and its author/role metadata
				// are never exposed to a Commentator-level key.
				svc.EXPECT().GetCommentsForContent(mock.Anything, 5).Return([]*commentdomain.Comment{
					buildComment(1, 5, 11, commentdomain.CommentStatusApproved, "ok"),
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
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(5, testUserID, true), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetCommentsForContent(mock.Anything, 5).Return([]*commentdomain.Comment{}, nil)
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
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(4, 999, false), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
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
			role:      "Admin",
			lookup: func() (commentagent.ContentDetail, error) {
				return publishedContent(4, 999, false), nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
				// Admin bypasses the AllowComments gate and moderates regardless.
				svc.EXPECT().GetCommentsForModeration(mock.Anything, 4).Return([]*commentdomain.Comment{
					buildComment(1, 4, 11, commentdomain.CommentStatusApproved, "ok"),
					buildComment(2, 4, 12, commentdomain.CommentStatusPending, "waiting"),
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
			lookup: func() (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{}, commentagent.ErrContentNotFound
			},
			setup: func(svc *agentmocks.MockCommentService) {
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - another user's draft returns NOT_FOUND (no disclosure)",
			contentID: "7",
			userID:    testUserID,
			role:      "",
			lookup: func() (commentagent.ContentDetail, error) {
				return commentagent.ContentDetail{ID: 7, UserID: 999, Published: false, AllowComments: true}, nil
			},
			setup: func(svc *agentmocks.MockCommentService) {
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
			lookup:     nil,
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

			var lookup commentagent.ContentLookup
			if tt.lookup != nil {
				detail, err := tt.lookup()
				lookup = staticLookup(detail, err)
			}
			handler := newCommentHandler(svc, lookup)
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
			role:      "Admin",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, commentdomain.CommentStatusPending, "hi"), nil)
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
					Return(buildComment(9, 5, testUserID, commentdomain.CommentStatusPending, "hi"), nil)
				svc.EXPECT().DeleteOwnComment(mock.Anything, 9, mock.Anything).Return(nil)
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
					Return(buildComment(9, 5, 999, commentdomain.CommentStatusPending, "hi"), nil)
				svc.EXPECT().DeleteOwnComment(mock.Anything, 9, mock.Anything).Return(commentdomain.ErrCommentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - admin delete missing comment returns NOT_FOUND",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      "Admin",
			setup: func(svc *agentmocks.MockCommentService) {
				// The missing comment fails the binding fetch before DeleteComment is reached.
				svc.EXPECT().GetComment(mock.Anything, 9).Return(nil, commentdomain.ErrCommentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:      "error - comment bound to different content returns NOT_FOUND (no disclosure)",
			contentID: "5",
			commentID: "9",
			userID:    testUserID,
			role:      "Admin",
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 8, 11, commentdomain.CommentStatusPending, "hi"), nil)
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

			handler := newCommentHandler(svc, nil)
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
			role:              "Admin",
			body:              `{"status": "approved"}`,
			wantStatus:        http.StatusOK,
			wantCommID:        9,
			wantCommentStatus: "approved",
			setup: func(svc *agentmocks.MockCommentService) {
				// Binding fetch (must match the path content id) …
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, commentdomain.CommentStatusPending, "ok"), nil).Once()
				svc.EXPECT().UpdateCommentStatus(mock.Anything, 9, commentdomain.CommentStatusApproved).Return(nil)
				// … then the authoritative re-fetch for the response.
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, commentdomain.CommentStatusApproved, "ok"), nil).Once()
			},
		},
		{
			name:      "error - comment bound to different content returns NOT_FOUND (no disclosure)",
			contentID: "5",
			commentID: "9",
			role:      "Admin",
			body:      `{"status": "approved"}`,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 8, 11, commentdomain.CommentStatusPending, "ok"), nil)
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
			body:       `{"status": "approved"}`,
			setup:      nil, // service must NOT be called
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "missing role is forbidden",
			contentID:  "5",
			commentID:  "9",
			role:       "",
			body:       `{"status": "approved"}`,
			setup:      nil,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:      "invalid status returns VALIDATION_ERROR",
			contentID: "5",
			commentID: "9",
			role:      "Admin",
			body:      `{"status": "bogus"}`,
			setup: func(svc *agentmocks.MockCommentService) {
				svc.EXPECT().GetComment(mock.Anything, 9).
					Return(buildComment(9, 5, 11, commentdomain.CommentStatusPending, "ok"), nil)
				svc.EXPECT().UpdateCommentStatus(mock.Anything, 9, commentdomain.CommentStatus("bogus")).
					Return(commentdomain.ErrInvalidCommentStatus)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "malformed body returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "9",
			role:       "Admin",
			body:       "{bad",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "invalid comment id returns VALIDATION_ERROR",
			contentID:  "5",
			commentID:  "0",
			role:       "Admin",
			body:       `{"status": "approved"}`,
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

			handler := newCommentHandler(svc, nil)
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
