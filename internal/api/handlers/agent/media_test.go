package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers/agent"
	agentmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/agent/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/api/response"
	mediadomain "github.com/aristorinjuang/lesstruct/internal/domain/media"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newMediaHandler builds a MediaHandler with a discard logger so handler unit tests do
// not require stdout wiring.
func newMediaHandler(svc agent.MediaService) *agent.MediaHandler {
	return agent.NewMediaHandler(svc, util.NewLogger(io.Discard))
}

// newMediaUploadRequest builds an authenticated multipart/form-data POST /api/v1/media
// request. A non-empty filename adds a `file` part (content is irrelevant — the service is
// mocked); a non-empty metadataJSON adds a `metadata` form field. userID/role are injected
// via the shared middleware identity keys.
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
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, strconv.Itoa(userID))
	if role != "" {
		ctx = context.WithValue(ctx, middleware.RoleKey, role)
	}
	return r.WithContext(ctx)
}

// envelopeDataMediaID decodes the envelope and returns the id nested under data.media.id
// (0 when absent). Used to assert the uploaded/fetched entity is returned in the
// MediaResponse wrapper.
func envelopeDataMediaID(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp response.Response
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

func TestMediaHandler_Upload(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	tests := []struct {
		name       string
		filename   string
		metadata   string
		withUser   bool
		setup      func(svc *agentmocks.MockMediaService)
		wantStatus int
		wantCode   string
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
			name:       "error - invalid mime type returns VALIDATION_ERROR",
			filename:   "photo.png",
			metadata:   `{"altText":"x"}`,
			withUser:   true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrInvalidMimeType)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - file too large returns VALIDATION_ERROR",
			filename:   "photo.png",
			metadata:   `{"altText":"x"}`,
			withUser:   true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrFileTooLarge)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - missing alt text returns VALIDATION_ERROR",
			filename:   "photo.png",
			metadata:   "", // no metadata → empty altText → service rejects
			withUser:   true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, mediadomain.ErrInvalidAltText)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "error - duplicate media returns 409 CONFLICT",
			filename:   "photo.png",
			metadata:   `{"altText":"x"}`,
			withUser:   true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).
					Return(nil, &mediadomain.DuplicateMediaError{Existing: &mediadomain.Media{ID: 9}})
			},
			wantStatus: http.StatusConflict,
			wantCode:   "CONFLICT",
		},
		{
			name:       "error - unexpected service error returns INTERNAL_ERROR",
			filename:   "photo.png",
			metadata:   `{"altText":"x"}`,
			withUser:   true,
			setup: func(svc *agentmocks.MockMediaService) {
				svc.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, errors.New("disk full"))
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
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
			target:   "/api/v1/media?cursor=" + encodeCursorForTest(10),
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
				assert.Equal(t, tt.wantNextCursorID, decodeCursorForTest(t, env.Meta.Pagination.NextCursor), "nextCursor id")
			} else {
				assert.Empty(t, env.Meta.Pagination.NextCursor, "expected no nextCursor")
			}
		})
	}
}
