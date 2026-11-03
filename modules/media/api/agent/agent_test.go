package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	mediaagent "github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/modules/media/api/agent/mocks"
	mediadomain "github.com/aristorinjuang/lesstruct/modules/media/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testUserID is the owning user injected into request context for the
// authenticated test cases.
const testUserID = 42

// stubRoles is a test-only role registry granting the media capability to a
// fixed set of roles. It satisfies mediaagent.RoleCapabilities without the
// core role service, which the module cannot import.
type stubRoles struct {
	media map[string]bool
}

func (s *stubRoles) CanMedia(name string) bool {
	return s.media[name]
}

// newMediaHandler builds a MediaHandler with a discard logger so handler unit tests do
// not require stdout wiring.
func newMediaHandler(svc mediaagent.MediaService) *mediaagent.MediaHandler {
	return mediaagent.NewMediaHandler(svc, nil)
}

// newMediaUploadRequest builds an authenticated multipart/form-data POST /api/v1/media
// request. A non-empty filename adds a `file` part (content is irrelevant — the service is
// mocked); a non-empty metadataJSON adds a `metadata` form field. userID/role are injected
// via the kernel principal, mirroring how the Bearer middleware writes identity.
func newMediaUploadRequest(
	t *testing.T,
	filename string,
	fileBytes []byte,
	metadataJSON string,
	userID int,
	role string,
) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		require.NoError(t, err, "create file part")
		_, _ = part.Write(fileBytes)
	}
	if metadataJSON != "" {
		require.NoError(t, writer.WriteField("metadata", metadataJSON), "write metadata field")
	}
	require.NoError(t, writer.Close(), "close multipart writer")

	r := httptest.NewRequest(http.MethodPost, "/api/v1/media", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
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

// newAuthenticatedRequest builds a request whose body is body, and — when withUser
// is true — injects testUserID via the kernel principal, mirroring how the
// Bearer middleware writes identity in production.
func newAuthenticatedRequest(method, target, body string, withUser bool) *http.Request {
	r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if withUser {
		ctx := kernel.ContextWithPrincipal(r.Context(), kernel.Principal{
			ID:       strconv.Itoa(testUserID),
			Username: "testuser",
		})
		r = r.WithContext(ctx)
	}
	return r
}

// newAuthenticatedRequestAs builds a request whose body is body, injecting the given
// userID (and role, when non-empty) via the kernel principal — mirroring how
// the Bearer middleware writes identity in production. Used for Get, which
// reads both userID and role for the ownership check.
func newAuthenticatedRequestAs(method, target, body string, userID int, role string) *http.Request {
	r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
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

// envelopeError decodes the response envelope and returns the error code ("" when
// the response carries no error).
func envelopeError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp httpx.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode response envelope")
	errInfo, ok := resp.Error.(map[string]any)
	if !ok {
		return ""
	}
	code, _ := errInfo["code"].(string)
	return code
}

// envelopeDataMediaID decodes the envelope and returns the id nested under data.media.id
// (0 when absent). Used to assert the uploaded/fetched entity is returned in the
// MediaResponse wrapper.
func envelopeDataMediaID(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp httpx.Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode response envelope")
	data, ok := resp.Data.(map[string]any)
	if !ok {
		return 0
	}
	mediaMap, ok := data["media"].(map[string]any)
	if !ok {
		return 0
	}
	id, _ := mediaMap["id"].(float64)
	return int(id)
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

// decodeListEnvelope decodes a recorded response into a listEnvelope.
func decodeListEnvelope(t *testing.T, w *httptest.ResponseRecorder) listEnvelope {
	t.Helper()
	var env listEnvelope
	require.NoError(t, json.NewDecoder(w.Body).Decode(&env), "failed to decode list envelope")
	return env
}

func TestMediaHandler_Upload(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	tests := []struct {
		name        string
		filename    string
		metadata    string
		withUser    bool
		setup       func(svc *agentmocks.MockMediaService)
		wantStatus  int
		wantCode    string
		wantMediaID int
	}{
		{
			name:       "success - file + metadata parts stored via service, projected",
			filename:   "photo.png",
			metadata:   `{"altText":"a sunset"}`,
			withUser:   true,
			wantStatus: http.StatusOK,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).
					Return(&mediadomain.Media{ID: 7, AltText: "a sunset", URL: "http://x/media/7.webp"}, nil)
			},
			wantMediaID: 7,
		},
		{
			name:       "error - missing file part returns VALIDATION_ERROR",
			filename:   "", // no file part
			metadata:   `{"altText":"x"}`,
			withUser:   true,
			setup:      nil, // Upload must NOT be reached
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - invalid mime type returns VALIDATION_ERROR",
			filename: "photo.png",
			metadata: `{"altText":"x"}`,
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrInvalidMimeType)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - file too large returns VALIDATION_ERROR",
			filename: "photo.png",
			metadata: `{"altText":"x"}`,
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrFileTooLarge)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - missing alt text returns VALIDATION_ERROR",
			filename: "photo.png",
			metadata: "", // no metadata → empty altText → service rejects
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrInvalidAltText)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:     "error - duplicate media returns 409 CONFLICT",
			filename: "photo.png",
			metadata: `{"altText":"x"}`,
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).
					Return(nil, &mediadomain.DuplicateMediaError{Existing: &mediadomain.Media{ID: 9}})
			},
			wantStatus: http.StatusConflict,
			wantCode:   "CONFLICT",
		},
		{
			name:     "error - unexpected service error returns INTERNAL_ERROR",
			filename: "photo.png",
			metadata: `{"altText":"x"}`,
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, errors.New("disk full"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
		{
			name:     "error - privacy scan failure returns PRIVACY_SCAN_FAILED",
			filename: "photo.png",
			metadata: `{"altText":"x"}`,
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrPrivacyScanFailed)
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "PRIVACY_SCAN_FAILED",
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			filename:   "photo.png",
			metadata:   `{"altText":"x"}`,
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newMediaHandler(svc)
			var r *http.Request
			if tt.withUser {
				r = newMediaUploadRequest(t, tt.filename, pngHeader, tt.metadata, testUserID, "")
			} else {
				r = newMediaUploadRequest(t, tt.filename, pngHeader, tt.metadata, 0, "")
				r = r.WithContext(context.Background()) // strip any identity
			}
			w := httptest.NewRecorder()
			handler.Upload(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantMediaID != 0 {
				assert.Equal(t, tt.wantMediaID, envelopeDataMediaID(t, w), "envelope data.media.id")
			}
		})
	}
}

func TestMediaHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		userID     int
		role       string
		setup      func(svc *agentmocks.MockMediaService)
		wantStatus int
		wantCode   string
		wantID     int
	}{
		{
			name:   "success - owner reads their own media",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&mediadomain.Media{ID: 5, UserID: testUserID, URL: "http://x/5"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     5,
		},
		{
			name:   "success - admin reads another user's media",
			id:     "5",
			userID: testUserID,
			role:   "Admin", // == contentdomain.RoleAdmin (the shared role string)
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&mediadomain.Media{ID: 5, UserID: 999, URL: "http://x/5"}, nil)
			},
			wantStatus: http.StatusOK,
			wantID:     5,
		},
		{
			name:   "not found returns 404",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(nil, mediadomain.ErrMediaNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:   "not owned (non-admin) returns 404, existence not disclosed",
			id:     "5",
			userID: testUserID,
			role:   "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().GetByID(mock.Anything, 5).Return(&mediadomain.Media{ID: 5, UserID: 999}, nil)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "invalid id returns VALIDATION_ERROR",
			id:         "abc",
			userID:     testUserID,
			role:       "Editor",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newMediaHandler(svc)
			r := newAuthenticatedRequestAs(http.MethodGet, "/api/v1/media/"+tt.id, "", tt.userID, tt.role)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			handler.Get(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantID != 0 {
				assert.Equal(t, tt.wantID, envelopeDataMediaID(t, w), "envelope data.media.id")
			}
		})
	}
}

func TestMediaHandler_List(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		withUser         bool
		setup            func(svc *agentmocks.MockMediaService)
		wantStatus       int
		wantCode         string
		wantDataLen      int
		wantHasMore      bool
		wantNextCursorID int
		wantBodyHas      string
	}{
		{
			name:     "success - first page no cursor, hasMore true, scoped to caller",
			target:   "/api/v1/media",
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				ids := make([]int, 51)
				for i := range ids {
					ids[i] = 51 - i
				}
				items := make([]*mediadomain.Media, 0, len(ids))
				for _, id := range ids {
					items = append(items, &mediadomain.Media{ID: id, UserID: testUserID})
				}
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0).Return(items, nil)
			},
			wantStatus:       http.StatusOK,
			wantDataLen:      50,
			wantHasMore:      true,
			wantNextCursorID: 2,
		},
		{
			name:     "success - next page via cursor returns older items, nextCursor empty",
			target:   "/api/v1/media?cursor=" + httpx.EncodeCursor(10),
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 10).
					Return([]*mediadomain.Media{{ID: 9, UserID: testUserID}, {ID: 8, UserID: testUserID}}, nil)
			},
			wantStatus:       http.StatusOK,
			wantDataLen:      2,
			wantHasMore:      false,
			wantNextCursorID: 0,
		},
		{
			name:     "success - empty list renders data as an empty array",
			target:   "/api/v1/media",
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0).Return([]*mediadomain.Media{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
			wantHasMore: false,
			wantBodyHas: `"data":[]`,
		},
		{
			name:     "success - over-max limit clamped to 100 (requests 101)",
			target:   "/api/v1/media?limit=999",
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 101, 0).Return([]*mediadomain.Media{}, nil)
			},
			wantStatus:  http.StatusOK,
			wantDataLen: 0,
		},
		{
			name:       "error - malformed cursor returns VALIDATION_ERROR",
			target:     "/api/v1/media?cursor=not-valid-base64!!!",
			withUser:   true,
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - missing user returns UNAUTHORIZED",
			target:     "/api/v1/media",
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:     "error - service error maps to INTERNAL_ERROR",
			target:   "/api/v1/media",
			withUser: true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().ListByCursor(mock.Anything, testUserID, 51, 0).Return(nil, fmt.Errorf("boom"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			handler := newMediaHandler(svc)
			r := newAuthenticatedRequest(http.MethodGet, tt.target, "", tt.withUser)
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
				nextID, err := httpx.DecodeCursor(env.Meta.Pagination.NextCursor)
				require.NoError(t, err)
				assert.Equal(t, tt.wantNextCursorID, nextID, "nextCursor id")
			} else {
				assert.Empty(t, env.Meta.Pagination.NextCursor, "expected no nextCursor")
			}
		})
	}
}

// TestMediaHandler_Upload_RoleGate verifies the agent media-upload endpoint is
// gated by the caller's role capability when a role service is configured. A
// role without the media capability gets FORBIDDEN before any service call;
// without a role service every authenticated user may upload (legacy behavior,
// covered by TestMediaHandler_Upload).
func TestMediaHandler_Delete(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		withUser    bool
		userID      int
		role        string
		withRoleSvc bool
		roleSvc     *stubRoles
		setup       func(svc *agentmocks.MockMediaService)
		wantStatus  int
		wantCode    string
	}{
		{
			name:     "success - role manages media",
			id:       "5",
			withUser: true,
			userID:   testUserID,
			role:     "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Delete(mock.Anything, 5, mock.Anything).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:        "role cannot manage media is forbidden",
			id:          "5",
			withUser:    true,
			userID:      testUserID,
			role:        "Editor",
			withRoleSvc: true,
			roleSvc:     &stubRoles{media: map[string]bool{}},
			wantStatus:  http.StatusForbidden,
			wantCode:    "FORBIDDEN",
		},
		{
			name:     "unknown id returns NOT_FOUND",
			id:       "5",
			withUser: true,
			userID:   testUserID,
			role:     "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Delete(mock.Anything, 5, mock.Anything).Return(mediadomain.ErrMediaNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:     "non-owner returns FORBIDDEN",
			id:       "5",
			withUser: true,
			userID:   testUserID,
			role:     "Editor",
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Delete(mock.Anything, 5, mock.Anything).Return(mediadomain.ErrUnauthorized)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "invalid id returns VALIDATION_ERROR",
			id:         "abc",
			withUser:   true,
			userID:     testUserID,
			role:       "Editor",
			setup:      nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "unauthenticated returns UNAUTHORIZED",
			id:         "5",
			withUser:   false,
			setup:      nil,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.setup != nil {
				tt.setup(svc)
			}

			var opts []mediaagent.MediaHandlerOption
			if tt.withRoleSvc {
				opts = append(opts, mediaagent.WithMediaRoleService(tt.roleSvc))
			}
			handler := mediaagent.NewMediaHandler(svc, nil, opts...)

			var r *http.Request
			if tt.withUser {
				r = newAuthenticatedRequestAs(http.MethodDelete, "/api/v1/media/"+tt.id, "", tt.userID, tt.role)
			} else {
				r = httptest.NewRequest(http.MethodDelete, "/api/v1/media/"+tt.id, nil)
			}
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			handler.Delete(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
		})
	}
}

func TestMediaHandler_Upload_RoleGate(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	roleService := &stubRoles{media: map[string]bool{}}

	roleServiceWithMedia := &stubRoles{media: map[string]bool{"Editor": true}}

	tests := []struct {
		name         string
		withRoleSvc  bool
		roleSvc      *stubRoles
		role         string
		expectUpload bool
		wantStatus   int
		wantCode     string
		wantMediaID  int
	}{
		{
			name:         "role cannot manage media is forbidden",
			withRoleSvc:  true,
			roleSvc:      roleService,
			role:         "Editor",
			expectUpload: false,
			wantStatus:   http.StatusForbidden,
			wantCode:     "FORBIDDEN",
		},
		{
			name:         "role can manage media succeeds",
			withRoleSvc:  true,
			roleSvc:      roleServiceWithMedia,
			role:         "Editor",
			expectUpload: true,
			wantStatus:   http.StatusOK,
			wantMediaID:  7,
		},
		{
			name:         "no role service preserves legacy behavior",
			withRoleSvc:  false,
			role:         "Editor",
			expectUpload: true,
			wantStatus:   http.StatusOK,
			wantMediaID:  8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := agentmocks.NewMockMediaService(t)
			if tt.expectUpload {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).
					Return(&mediadomain.Media{ID: tt.wantMediaID, AltText: "x", URL: "http://x/media/7.webp"}, nil)
			}

			var opts []mediaagent.MediaHandlerOption
			if tt.withRoleSvc {
				opts = append(opts, mediaagent.WithMediaRoleService(tt.roleSvc))
			}
			handler := mediaagent.NewMediaHandler(svc, nil, opts...)

			r := newMediaUploadRequest(t, "photo.png", pngHeader, `{"altText":"x"}`, testUserID, tt.role)
			w := httptest.NewRecorder()
			handler.Upload(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantCode != "" {
				assert.Equal(t, tt.wantCode, envelopeError(t, w))
			}
			if tt.wantMediaID != 0 {
				assert.Equal(t, tt.wantMediaID, envelopeDataMediaID(t, w), "envelope data.media.id")
			}
		})
	}
}
