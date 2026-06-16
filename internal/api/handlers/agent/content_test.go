package agent_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/internal/content/markdown"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testUserID is the owning user injected into request context for the authenticated
// test cases. It mirrors the shared middleware identity contract (UserIDKey holds
// the decimal id as a string).
const testUserID = 42

// marshalJSON marshals v to a JSON string. It is a test-only helper for building
// request bodies with correct escaping (e.g. embedding a Tiptap JSON string into
// the "body" field); the inputs never fail to marshal.
func marshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// mustConvert returns the canonical Tiptap JSON the handler stores for a Markdown
// body, so handler tests share the converter as the single source of truth and no
// drifting Tiptap literal is hardcoded. The converter only errors on an internal
// marshal failure (effectively unreachable), so this never fails in practice.
func mustConvert(t *testing.T, md string) string {
	t.Helper()
	out, err := markdown.Convert(md)
	require.NoError(t, err)
	return out
}

// newContentHandler builds a ContentHandler with a discard logger so handler unit
// tests do not require stdout wiring.
func newContentHandler(svc agent.ContentService) *agent.ContentHandler {
	return agent.NewContentHandler(svc, util.NewLogger(io.Discard))
}

// newAuthenticatedRequest builds a request whose body is body, and — when withUser
// is true — injects testUserID via the shared middleware identity key (the same key
// the Bearer/JWT middleware writes), mirroring how the handler reads identity in
// production.
func newAuthenticatedRequest(method, target, body string, withUser bool) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if withUser {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, strconv.Itoa(testUserID))
		r = r.WithContext(ctx)
	}
	return r
}

// envelopeError decodes the response envelope and returns the error code ("" when
// the response carries no error).
func envelopeError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp response.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode response envelope")
	errInfo, ok := resp.Error.(map[string]any)
	if !ok {
		return ""
	}
	code, _ := errInfo["code"].(string)
	return code
}

// envelopeDataContentID decodes the envelope and returns the id nested under
// data.content.id (0 when absent). Used to assert the created/fetched entity is
// returned in the ContentResponse wrapper.
func envelopeDataContentID(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp response.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode response envelope")
	data, ok := resp.Data.(map[string]any)
	if !ok {
		return 0
	}
	content, ok := data["content"].(map[string]any)
	if !ok {
		return 0
	}
	id, _ := content["id"].(float64)
	return int(id)
}

// newAuthenticatedRequestAs builds a request whose body is body, injecting the given
// userID (and role, when non-empty) via the shared middleware identity keys — mirroring
// how the Bearer middleware writes identity in production. Used for Update/Delete, which
// read both userID and role for the ownership check.
func newAuthenticatedRequestAs(method, target, body string, userID int, role string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, strconv.Itoa(userID))
	if role != "" {
		ctx = context.WithValue(ctx, middleware.RoleKey, role)
	}
	return r.WithContext(ctx)
}

// encodeCursorForTest mirrors the agent package's opaque cursor encoding (unpadded
// base64 URL of the decimal id) so tests can feed the List handler a realistic
// client-supplied token. It lives in the test package because encodeCursor is unexported
// and CLAUDE.md forbids testing private functions directly — the helpers are exercised
// through the List handler.
func encodeCursorForTest(id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(id)))
}

// decodeCursorForTest inverts encodeCursorForTest so tests can assert the nextCursor the
// List handler emits points at the expected id.
func decodeCursorForTest(t *testing.T, cursor string) int {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	require.NoError(t, err, "test cursor must be valid base64")
	id, err := strconv.Atoi(string(b))
	require.NoError(t, err, "test cursor must decode to an integer")
	return id
}

// buildContents builds a slice of *contentdomain.Content with the given ids (in the order
// supplied) for List handler fixtures.
func buildContents(ids ...int) []*contentdomain.Content {
	items := make([]*contentdomain.Content, 0, len(ids))
	for _, id := range ids {
		items = append(items, &contentdomain.Content{ID: id, UserID: testUserID, Title: "Item " + strconv.Itoa(id)})
	}
	return items
}

// listEnvelope is the shape of the agent v1 list response, used to assert pagination meta
// and the data array (which must always be present — never omitted for empty lists).
type listEnvelope struct {
	Data []map[string]any `json:"data"`
	Meta struct {
		Pagination struct {
			NextCursor string `json:"nextCursor"`
			HasMore    bool   `json:"hasMore"`
		} `json:"pagination"`
	} `json:"meta"`
}

// wantPreservedFields describes the server-managed fields the update handler
// is expected to copy from the existing item onto the domain request
// (MetaDescription / OGTitle / OGDescription / AllowComments /
// TranslationGroupID). A zero value on every field means "do not assert" — the
// test row does not care about preserved-field behavior.
type wantPreservedFields struct {
	MetaDescription string
	OGTitle         string
	OGDescription   string
	AllowComments   bool
	TranslationID   int
}

func (w wantPreservedFields) isZero() bool {
	return w == wantPreservedFields{}
}

// decodeListEnvelope decodes a recorded response into a listEnvelope.
func decodeListEnvelope(t *testing.T, w *httptest.ResponseRecorder) listEnvelope {
	t.Helper()
	var env listEnvelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&env), "failed to decode list envelope")
	return env
}

func TestContentHandler_Create(t *testing.T) {
	// Canonical Tiptap JSON — must be stored unchanged for format=tiptap.
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph"}]}`

	tests := []struct {
		name             string
		body             string
		withUser         bool
		setup            func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest)
		wantStatus       int
		wantCode         string
		wantMessageHas   string // substring expected in error message ("" = skip)
		wantBodyContent  string // asserts captured.Content when checkRequest is true
		wantDomainStatus contentdomain.Status
		checkRequest     bool
		wantContentID    int    // asserts data.content.id in the envelope (0 = skip)
		wantConvertBody  string // markdown body to convert for the expected stored content (overrides wantBodyContent)
		wantTags         []string
		wantLanguage     string
		wantPostType     string
	}{
		{
			name:     "success - tiptap body stored unchanged with draft status",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 7, Title: "Hello"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    7,
		},
		{
			name:     "success - isPublished true maps to published status",
			body:     marshalJSON(map[string]any{"title": "Published", "body": tiptapJSON, "isPublished": true}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 8, Title: "Published"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusPublished,
			checkRequest:     true,
			wantContentID:    8,
		},
		{
			name:     "success - explicit format tiptap passes body unchanged",
			body:     marshalJSON(map[string]any{"title": "Explicit", "body": tiptapJSON, "format": "tiptap"}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 9, Title: "Explicit"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    9,
		},
		{
			name:     "success - uppercase format TIPTAP is normalized and accepted",
			body:     marshalJSON(map[string]any{"title": "Upper", "body": tiptapJSON, "format": "TIPTAP"}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 10, Title: "Upper"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    10,
		},
		{
			name:     "success - whitespace-padded Markdown normalizes and converts to tiptap",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": "# hi", "format": "  Markdown  "}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 11, Title: "Hello"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantConvertBody:  "# hi",
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    11,
		},
		{
			name:     "error - validation error returns VALIDATION_ERROR",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Return(nil, contentdomain.ErrInvalidTitle)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - ownership returns FORBIDDEN",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Return(nil, contentdomain.ErrUnauthorized)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:     "error - custom-field validation error returns VALIDATION_ERROR not INTERNAL_ERROR",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Return(nil, fmt.Errorf("%w: Price: must be a number", contentdomain.ErrCustomFieldValidation))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - not found on create returns NOT_FOUND",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:     "error - unexpected error returns INTERNAL_ERROR",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.CreateContentRequest) {
				// A non-sentinel error so handleError falls through to the default
				// INTERNAL_ERROR branch (sentinel errors are covered by other rows).
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Return(nil, errors.New("boom"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
		{
			name:     "success - markdown format converts body to tiptap and stores draft",
			body:     marshalJSON(map[string]any{"title": "Hello", "body": "# hi", "format": "markdown"}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 12, Title: "Hello"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantConvertBody:  "# hi",
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    12,
		},
		{
			name:     "success - richer markdown body is converted end-to-end and published",
			body:     marshalJSON(map[string]any{"title": "Post", "body": "**bold** and `code`", "format": "markdown", "isPublished": true}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 13, Title: "Post"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantConvertBody:  "**bold** and `code`",
			wantDomainStatus: contentdomain.StatusPublished,
			checkRequest:     true,
			wantContentID:    13,
		},
		{
			name:       "error - unsupported format returns VALIDATION_ERROR",
			body:       marshalJSON(map[string]any{"title": "Hello", "body": "x", "format": "html"}),
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - malformed JSON body returns VALIDATION_ERROR",
			body:       "{bad json",
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:   "success - postType, tags, and language round-trip into domain request",
			body:   marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON, "postType": "post", "tags": []string{"a", "b"}, "language": "en"}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 14, Title: "Hello"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    14,
			wantPostType:     "post",
			wantTags:         []string{"a", "b"},
			wantLanguage:     "en",
		},
		{
			name:   "success - empty tags and language in body map to zero values",
			body:   marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON, "postType": "page"}),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.CreateContentRequest) {
				svc.EXPECT().Create(mock.Anything, testUserID, mock.Anything).
					Run(func(_ context.Context, _ int, req contentdomain.CreateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 15, Title: "Hello"}, nil)
			},
			wantStatus:       http.StatusOK,
			wantBodyContent:  tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			checkRequest:     true,
			wantContentID:    15,
			wantPostType:     "page",
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			body:       marshalJSON(map[string]any{"title": "Hello", "body": tiptapJSON}),
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			var captured contentdomain.CreateContentRequest
			if tt.setup != nil {
				tt.setup(svc, &captured)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequest(http.MethodPost, "/api/v1/content", tt.body, tt.withUser)
			w := httptest.NewRecorder()
			handler.Create(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			// Decode the envelope once — wantCode and wantMessageHas both read the
			// error object, and the response body can only be consumed a single time.
			if tt.wantCode != "" || tt.wantMessageHas != "" {
				var resp response.Response
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				errInfo, ok := resp.Error.(map[string]any)
				require.True(t, ok, "expected an error envelope")
				if tt.wantCode != "" {
					code, _ := errInfo["code"].(string)
					assert.Equal(t, tt.wantCode, code)
				}
				if tt.wantMessageHas != "" {
					msg, _ := errInfo["message"].(string)
					assert.Contains(t, msg, tt.wantMessageHas)
				}
			}
			if tt.checkRequest {
				// Markdown rows assert the converter output (single source of
				// truth via mustConvert); tiptap rows assert the literal body.
				wantBody := tt.wantBodyContent
				if tt.wantConvertBody != "" {
					wantBody = mustConvert(t, tt.wantConvertBody)
				}
				assert.Equal(t, wantBody, captured.Content, "body must be stored (converted for markdown)")
				assert.Equal(t, tt.wantDomainStatus, captured.Status, "status mapping")
				// The want* fields are zero-value sentinels: a non-empty wantPostType
				// / wantLanguage / non-nil wantTags means the test row explicitly
				// asserts that field on the domain request. Rows that omit the
				// metadata round-trip naturally assert the zero value (the v1
				// body did not set the field).
				assert.Equal(t, tt.wantPostType, captured.PostType, "PostType mapping")
				assert.Equal(t, tt.wantLanguage, captured.Language, "Language mapping")
				if tt.wantTags != nil {
					assert.Equal(t, tt.wantTags, captured.Tags, "Tags mapping")
				}
			}
			if tt.wantContentID != 0 {
				assert.Equal(t, tt.wantContentID, envelopeDataContentID(t, w), "envelope data.content.id")
			}
		})
	}
}

func TestContentHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         string // path value for {id}; "" → do not set
		withUser   bool
		setup      func(svc *agentmocks.MockContentService)
		wantStatus int
		wantCode   string
		wantID     int // expected data.content.id in the envelope (0 = skip)
	}{
		{
			name:     "success - published content is readable by any authenticated user",
			id:       "5",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				// Owned by another user (999) but published → visible to testUserID (42).
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, Status: contentdomain.StatusPublished, Title: "Public",
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     5,
		},
		{
			name:     "success - owner can read their own draft",
			id:       "6",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				// Draft owned by testUserID → visible to the owner.
				svc.EXPECT().GetByID(mock.Anything, 6).Return(&contentdomain.Content{
					ID: 6, UserID: testUserID, Status: contentdomain.StatusDraft, Title: "My Draft",
				}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     6,
		},
		{
			name:     "IDOR - another user's draft returns NOT_FOUND (existence not disclosed)",
			id:       "7",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				// Draft owned by another user (999) → must NOT be readable by testUserID (42).
				svc.EXPECT().GetByID(mock.Anything, 7).Return(&contentdomain.Content{
					ID: 7, UserID: 999, Status: contentdomain.StatusDraft, Title: "Their Draft",
				}, nil)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:     "not found returns NOT_FOUND",
			id:       "5",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "invalid id returns VALIDATION_ERROR",
			id:         "abc",
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "negative id returns VALIDATION_ERROR",
			id:         "-1",
			withUser:   true,
			setup:      nil, // rejected by the id<=0 guard before GetByID is called
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "zero id returns VALIDATION_ERROR",
			id:         "0",
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "missing user returns UNAUTHORIZED",
			id:         "5",
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequest(http.MethodGet, "/api/v1/content/"+tt.id, "", tt.withUser)
			if tt.id != "" {
				r.SetPathValue("id", tt.id)
			}
			w := httptest.NewRecorder()
			handler.Get(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantID != 0 {
				assert.Equal(t, tt.wantID, envelopeDataContentID(t, w), "envelope data.content.id")
			}
		})
	}
}

func TestContentHandler_List(t *testing.T) {
	zeroFilters := contentdomain.ContentFilters{}

	tests := []struct {
		name             string
		target           string // full path including query string
		withUser         bool
		userRole         string // role injected into the request context ("" = unset)
		setup            func(svc *agentmocks.MockContentService)
		wantStatus       int
		wantCode         string
		wantDataLen      int
		wantHasMore      bool
		wantNextCursorID int // decoded id; 0 means expect an empty nextCursor
		wantBodyHas      string
	}{
		{
			name:     "success - first page no cursor, hasMore true, scoped to caller",
			target:   "/api/v1/content",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				// Default limit 50 → handler requests limit+1 = 51, beforeID 0.
				ids := make([]int, 51)
				for i := range ids {
					ids[i] = 51 - i // 51, 50, ..., 1 (newest-first)
				}
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, zeroFilters).Return(buildContents(ids...), nil)
			},
			wantStatus:       http.StatusOK,
			wantDataLen:      50,
			wantHasMore:      true,
			wantNextCursorID: 2, // after trimming to 50, last item is id 2
		},
		{
			name:     "success - next page via cursor returns older items, nextCursor empty",
			target:   "/api/v1/content?cursor=" + encodeCursorForTest(10),
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				// Cursor decodes to beforeID=10 → handler requests id < 10 items.
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 10, zeroFilters).Return(buildContents(9, 8), nil)
			},
			wantStatus:       http.StatusOK,
			wantDataLen:      2,
			wantHasMore:      false,
			wantNextCursorID: 0,
		},
		{
			name:     "success - empty list renders data as an empty array, hasMore false",
			target:   "/api/v1/content",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, zeroFilters).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
			wantHasMore: false,
			wantBodyHas: `"data":[]`,
		},
		{
			name:     "success - negative limit clamped to default 50 (requests 51)",
			target:   "/api/v1/content?limit=-5",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, zeroFilters).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - over-max limit clamped to 100 (requests 101)",
			target:   "/api/v1/content?limit=999",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 101, 0, zeroFilters).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - single tag filter passed through to service",
			target:   "/api/v1/content?tag=go",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Tags: []string{"go"},
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - multiple tag filter (AND-of-tags) preserves order",
			target:   "/api/v1/content?tag=go&tag=tutorial",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Tags: []string{"go", "tutorial"},
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - empty tag values are dropped",
			target:   "/api/v1/content?tag=&tag=go",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Tags: []string{"go"},
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - language filter passed through",
			target:   "/api/v1/content?language=en",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Language: "en",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - status=draft filter passed through",
			target:   "/api/v1/content?status=draft",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Status: "draft",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - status=published filter passed through",
			target:   "/api/v1/content?status=published",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Status: "published",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:       "error - status=garbage returns VALIDATION_ERROR",
			target:     "/api/v1/content?status=garbage",
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "success - post_type filter passed through",
			target:   "/api/v1/content?post_type=post",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					PostType: "post",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - search filter passed through verbatim",
			target:   "/api/v1/content?search=golang",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Search: "golang",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - search shorter than 2 chars is dropped (no filter applied)",
			target:   "/api/v1/content?search=g",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, zeroFilters).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:     "success - author filter applied for admin",
			target:   "/api/v1/content?author=alice",
			withUser: true,
			userRole: contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, contentdomain.ContentFilters{
					Author: "alice",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:       "error - author filter rejected for non-admin",
			target:     "/api/v1/content?author=alice",
			withUser:   true,
			userRole:   "Editor",
			setup:      nil, // ListByCursor must NOT be reached
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "error - author filter rejected for caller with no role",
			target:     "/api/v1/content?author=alice",
			withUser:   true,
			userRole:   "",
			setup:      nil,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:     "success - all filters combined in one request",
			target: "/api/v1/content" +
				"?tag=go&tag=tutorial" +
				"&language=en" +
				"&status=draft" +
				"&post_type=post" +
				"&search=golang" +
				"&author=alice" +
				"&limit=5",
			withUser: true,
			userRole: contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 6, 0, contentdomain.ContentFilters{
					Tags:     []string{"go", "tutorial"},
					Language: "en",
					Status:   "draft",
					PostType: "post",
					Search:   "golang",
					Author:   "alice",
				}).Return([]*contentdomain.Content{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:       "error - malformed cursor returns VALIDATION_ERROR",
			target:     "/api/v1/content?cursor=not-valid-base64!!!",
			withUser:   true,
			setup:      nil, // ListByCursor must NOT be reached
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			target:     "/api/v1/content",
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:     "error - service error maps to INTERNAL_ERROR",
			target:   "/api/v1/content",
			withUser: true,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0, zeroFilters).Return(nil, errors.New("boom"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newContentHandler(svc)
			var r *http.Request
			if tt.userRole != "" {
				r = newAuthenticatedRequestAs(http.MethodGet, tt.target, "", testUserID, tt.userRole)
			} else {
				r = newAuthenticatedRequest(http.MethodGet, tt.target, "", tt.withUser)
			}
			w := httptest.NewRecorder()
			handler.List(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
				return
			}
			if tt.wantBodyHas != "" {
				assert.Contains(t, w.Body.String(), tt.wantBodyHas, "body must contain")
			}

			env := decodeListEnvelope(t, w)
			assert.Len(t, env.Data, tt.wantDataLen, "data array length")
			assert.Equal(t, tt.wantHasMore, env.Meta.Pagination.HasMore, "hasMore")
			if tt.wantNextCursorID != 0 {
				require.NotEmpty(t, env.Meta.Pagination.NextCursor, "expected a nextCursor")
				assert.Equal(t, tt.wantNextCursorID, decodeCursorForTest(t, env.Meta.Pagination.NextCursor), "nextCursor id")
			} else {
				assert.Empty(t, env.Meta.Pagination.NextCursor, "expected no nextCursor")
			}
		})
	}
}

func TestContentHandler_Update(t *testing.T) {
	tiptapJSON := `{"type":"doc","content":[{"type":"paragraph"}]}`

	tests := []struct {
		name             string
		id               string
		body             string
		userID           int
		role             string
		setup            func(svc *agentmocks.MockContentService, captured *contentdomain.UpdateContentRequest)
		wantStatus       int
		wantCode         string
		checkReq         bool
		wantPostType     string
		wantTags         []string
		wantLanguage     string
		wantContent      string
		wantDomainStatus contentdomain.Status // domain status mapped from isPublished
		wantConvertBody  string               // markdown body to convert for expected stored content (overrides wantContent)
		wantPreserved    wantPreservedFields  // server-managed fields asserted after mapping
	}{
		{
			name:   "success - owner update with no v1 metadata uses zero values for postType/tags/language",
			id:     "5",
			body:   marshalJSON(map[string]any{"title": "New Title", "body": tiptapJSON}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: testUserID, PostType: "page", Tags: []string{"keep"},
					Language: "fr", AllowComments: false,
				}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, "Editor", mock.Anything).
					Run(func(_ context.Context, _ int, _ int, _ string, req contentdomain.UpdateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 5, UserID: testUserID, Title: "New Title"}, nil)
			},
			wantStatus:       http.StatusOK,
			checkReq:         true,
			wantContent:      tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			// wantPostType / wantTags / wantLanguage all zero → handler should
			// forward "" / nil / "" (the v1 body did not set them; the existing
			// item's values are NOT preserved — that's the contract change).
		},
		{
			name:   "success - owner update with v1 metadata uses supplied postType/tags/language and preserves server-managed fields",
			id:     "5",
			body: marshalJSON(map[string]any{
				"title":    "New Title",
				"body":     tiptapJSON,
				"postType": "article",
				"tags":     []string{"alpha", "beta"},
				"language": "id",
			}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.UpdateContentRequest) {
				groupID := 7
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: testUserID, PostType: "page", Tags: []string{"old"},
					Language: "fr", AllowComments: false,
					MetaDescription: "old meta", OGTitle: "old og", OGDescription: "old og desc",
					TranslationGroupID: &groupID,
				}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, "Editor", mock.Anything).
					Run(func(_ context.Context, _ int, _ int, _ string, req contentdomain.UpdateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 5, UserID: testUserID, Title: "New Title"}, nil)
			},
			wantStatus:       http.StatusOK,
			checkReq:         true,
			wantPostType:     "article",
			wantTags:         []string{"alpha", "beta"},
			wantLanguage:     "id",
			wantContent:      tiptapJSON,
			wantDomainStatus: contentdomain.StatusDraft,
			wantPreserved: wantPreservedFields{
				MetaDescription: "old meta",
				OGTitle:         "old og",
				OGDescription:   "old og desc",
				AllowComments:   false,
				TranslationID:   7,
			},
		},
		{
			name:   "success - admin can update another user's content with v1 metadata",
			id:     "5",
			body: marshalJSON(map[string]any{
				"title":    "T",
				"body":     tiptapJSON,
				"isPublished": true,
				"postType": "post",
			}),
			userID: testUserID,
			role:   contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: 999, PostType: "page", Tags: []string{"old"},
				}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, contentdomain.RoleAdmin, mock.Anything).
					Run(func(_ context.Context, _ int, _ int, _ string, req contentdomain.UpdateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 5, UserID: 999, Title: "T"}, nil)
			},
			wantStatus:       http.StatusOK,
			checkReq:         true,
			wantPostType:     "post",
			wantContent:      tiptapJSON,
			wantDomainStatus: contentdomain.StatusPublished,
		},
		{
			name:   "success - owner markdown update converts to tiptap and forwards v1 metadata",
			id:     "5",
			body: marshalJSON(map[string]any{
				"title":    "T",
				"body":     "# hi",
				"format":   "markdown",
				"postType": "post",
				"tags":     []string{"new"},
				"language": "en",
			}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, captured *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{
					ID: 5, UserID: testUserID, PostType: "page", Tags: []string{"old"},
					Language: "fr", AllowComments: true,
				}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, "Editor", mock.Anything).
					Run(func(_ context.Context, _ int, _ int, _ string, req contentdomain.UpdateContentRequest) {
						*captured = req
					}).
					Return(&contentdomain.Content{ID: 5, UserID: testUserID, Title: "T"}, nil)
			},
			wantStatus:       http.StatusOK,
			checkReq:         true,
			wantPostType:     "post",
			wantTags:         []string{"new"},
			wantLanguage:     "en",
			wantConvertBody:  "# hi",
			wantDomainStatus: contentdomain.StatusDraft,
		},
		{
			name:   "error - not found returns 404",
			id:     "5",
			body:   marshalJSON(map[string]any{"title": "T", "body": tiptapJSON}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - not owned (non-admin) returns 404, existence not disclosed",
			id:     "5",
			body:   marshalJSON(map[string]any{"title": "T", "body": tiptapJSON}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: 999}, nil)
				// Update must NOT be reached (no EXPECT on it).
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - service ownership error after pre-fetch maps to FORBIDDEN",
			id:     "5",
			body:   marshalJSON(map[string]any{"title": "T", "body": tiptapJSON}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: testUserID}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, "Editor", mock.Anything).
					Return(nil, contentdomain.ErrUnauthorized)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:   "error - custom-field validation error returns VALIDATION_ERROR",
			id:     "5",
			body:   marshalJSON(map[string]any{"title": "T", "body": tiptapJSON}),
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService, _ *contentdomain.UpdateContentRequest) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: testUserID}, nil)
				svc.EXPECT().Update(mock.Anything, 5, testUserID, "Editor", mock.Anything).
					Return(nil, fmt.Errorf("%w: Price: must be a number", contentdomain.ErrCustomFieldValidation))
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - malformed JSON body returns VALIDATION_ERROR",
			id:         "5",
			body:       "{bad json",
			userID:     testUserID,
			role:       "Editor",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - invalid id returns VALIDATION_ERROR",
			id:         "abc",
			body:       marshalJSON(map[string]any{"title": "T", "body": tiptapJSON}),
			userID:     testUserID,
			role:       "Editor",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			var captured contentdomain.UpdateContentRequest
			if tt.setup != nil {
				tt.setup(svc, &captured)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodPut, "/api/v1/content/"+tt.id, tt.body, tt.userID, tt.role)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			handler.Update(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
				return
			}
			if tt.checkReq {
				// v1 metadata fields use zero-value sentinels: a non-empty
				// wantPostType / wantLanguage / non-nil wantTags means the row
				// explicitly asserts the v1-supplied value. The postType /
				// tags / language flow from the v1 body; the existing item's
				// values are NOT preserved.
				assert.Equal(t, tt.wantPostType, captured.PostType, "PostType from v1 body")
				assert.Equal(t, tt.wantLanguage, captured.Language, "Language from v1 body")
				if tt.wantTags != nil {
					assert.Equal(t, tt.wantTags, captured.Tags, "Tags from v1 body")
				}
				// Server-managed fields (SEO / allowComments / translationGroupID)
				// are preserved from the existing item; rows that opt in via
				// a non-zero wantPreserved assert the copy.
				if !tt.wantPreserved.isZero() {
					assert.Equal(t, tt.wantPreserved.MetaDescription, captured.MetaDescription, "MetaDescription preserved")
					assert.Equal(t, tt.wantPreserved.OGTitle, captured.OGTitle, "OGTitle preserved")
					assert.Equal(t, tt.wantPreserved.OGDescription, captured.OGDescription, "OGDescription preserved")
					require.NotNil(t, captured.AllowComments, "AllowComments pointer must be set when an existing item has a value")
					assert.Equal(t, tt.wantPreserved.AllowComments, *captured.AllowComments, "AllowComments preserved")
					if tt.wantPreserved.TranslationID != 0 {
						require.NotNil(t, captured.TranslationGroupID, "TranslationGroupID pointer must be set when the existing item has a group")
						assert.Equal(t, tt.wantPreserved.TranslationID, *captured.TranslationGroupID, "TranslationGroupID preserved")
					}
				}
				// Markdown rows assert the converter output (single source of
				// truth via mustConvert); tiptap rows assert the literal body.
				wantContent := tt.wantContent
				if tt.wantConvertBody != "" {
					wantContent = mustConvert(t, tt.wantConvertBody)
				}
				assert.Equal(t, wantContent, captured.Content, "Content mapped from body")
				assert.Equal(t, tt.wantDomainStatus, captured.Status, "Status mapped from isPublished")
			}
		})
	}
}

func TestContentHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockContentService)
		wantStatus int
		wantCode   string
	}{
		{
			name:   "success - owner deletes returns 204 no content",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: testUserID}, nil)
				svc.EXPECT().DeleteContent(mock.Anything, 5, testUserID, "Editor").Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "success - admin deletes another user's content returns 204",
			id:     "5",
			userID: testUserID,
			role:   contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: 999}, nil)
				svc.EXPECT().DeleteContent(mock.Anything, 5, testUserID, contentdomain.RoleAdmin).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "error - not found returns 404",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - not owned (non-admin) returns 404, existence not disclosed",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&contentdomain.Content{ID: 5, UserID: 999}, nil)
				// DeleteContent must NOT be reached.
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "error - invalid id returns VALIDATION_ERROR",
			id:         "0",
			userID:     testUserID,
			role:       "Editor",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodDelete, "/api/v1/content/"+tt.id, "", tt.userID, tt.role)
			r.SetPathValue("id", tt.id)
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

func TestContentHandler_Publish(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockContentService)
		wantStatus int
		wantCode   string
	}{
		{
			name:   "success - owner publishes draft returns 200 with the published projection",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Publish(mock.Anything, 5, testUserID, "Editor").
					Return(&contentdomain.Content{
						ID:     5,
						UserID: testUserID,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusPublished,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "success - admin publishes another user's draft",
			id:     "5",
			userID: testUserID,
			role:   contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Publish(mock.Anything, 5, testUserID, contentdomain.RoleAdmin).
					Return(&contentdomain.Content{
						ID:     5,
						UserID: 999,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusPublished,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "idempotent - publish already-published post returns 200, no hook side-effect at handler layer",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Publish(mock.Anything, 5, testUserID, "Editor").
					Return(&contentdomain.Content{
						ID:     5,
						UserID: testUserID,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusPublished,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "error - not owned by non-admin maps to NOT_FOUND, no disclosure",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Publish(mock.Anything, 5, testUserID, "Editor").
					Return(nil, contentdomain.ErrUnauthorized)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - not found maps to NOT_FOUND",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Publish(mock.Anything, 5, testUserID, "Editor").
					Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - invalid id returns VALIDATION_ERROR",
			id:     "0",
			userID: testUserID,
			role:   "Editor",
			setup:  nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodPost, "/api/v1/content/"+tt.id+"/publish", "", tt.userID, tt.role)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			handler.Publish(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, 5, envelopeDataContentID(t, w), "200 must include the published content projection")
			}
		})
	}
}

func TestContentHandler_Unpublish(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockContentService)
		wantStatus int
		wantCode   string
	}{
		{
			name:   "success - owner unpublishes published returns 200 with the draft projection",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Unpublish(mock.Anything, 5, testUserID, "Editor").
					Return(&contentdomain.Content{
						ID:     5,
						UserID: testUserID,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusDraft,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "success - admin unpublishes another user's published post",
			id:     "5",
			userID: testUserID,
			role:   contentdomain.RoleAdmin,
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Unpublish(mock.Anything, 5, testUserID, contentdomain.RoleAdmin).
					Return(&contentdomain.Content{
						ID:     5,
						UserID: 999,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusDraft,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "idempotent - unpublish already-draft post returns 200",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Unpublish(mock.Anything, 5, testUserID, "Editor").
					Return(&contentdomain.Content{
						ID:     5,
						UserID: testUserID,
						Title:  "Hello",
						Slug:   "hello",
						Status: contentdomain.StatusDraft,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "error - not owned by non-admin maps to NOT_FOUND, no disclosure",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Unpublish(mock.Anything, 5, testUserID, "Editor").
					Return(nil, contentdomain.ErrUnauthorized)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - not found maps to NOT_FOUND",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockContentService) {
				svc.EXPECT().
					Unpublish(mock.Anything, 5, testUserID, "Editor").
					Return(nil, contentdomain.ErrContentNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "error - invalid id returns VALIDATION_ERROR",
			id:     "0",
			userID: testUserID,
			role:   "Editor",
			setup:  nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockContentService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newContentHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodPost, "/api/v1/content/"+tt.id+"/unpublish", "", tt.userID, tt.role)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			handler.Unpublish(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, 5, envelopeDataContentID(t, w), "200 must include the unpublished content projection")
			}
		})
	}
}
