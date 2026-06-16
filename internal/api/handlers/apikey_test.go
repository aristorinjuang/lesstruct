package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/api/handlers"
	handlersmocks "github.com/aristorinjuang/lesstruct/internal/api/handlers/mocks"
	"github.com/aristorinjuang/lesstruct/internal/api/middleware"
	"github.com/aristorinjuang/lesstruct/internal/domain/apikey"
	"github.com/aristorinjuang/lesstruct/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyHandler_CreateAPIKey(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		body          string
		setupMock     func(*handlersmocks.MockAPIKeyService)
		wantStatus    int
		wantErrCode   string
		wantKeyInData bool
	}{
		{
			name:   "success - key created and returned once",
			userID: "1",
			body:   `{"name":"My CI Key"}`,
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 1, "My CI Key").
					Return(
						"lesstruct_aabbccddeeff_0123456789abcdef0123456789abcdef",
						&apikey.APIKey{
							ID:     42,
							UserID: 1,
							Name:   "My CI Key",
							KeyID:  "aabbccddeeff",
						},
						nil,
					)
			},
			wantStatus:    http.StatusOK,
			wantKeyInData: true,
		},
		{
			name:        "error - no user in context returns 401 UNAUTHORIZED",
			userID:      "",
			body:        `{"name":"x"}`,
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
		{
			name:        "error - non-numeric user ID returns 500 INTERNAL_ERROR (server invariant)",
			userID:      "abc",
			body:        `{"name":"x"}`,
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
		{
			name:        "error - malformed JSON body returns 400 INVALID_REQUEST_BODY",
			userID:      "1",
			body:        `{bad json`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST_BODY",
		},
		{
			name:        "error - empty name flows to service and returns 400 VALIDATION_ERROR",
			userID:      "1",
			body:        `{"name":""}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "VALIDATION_ERROR",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 1, "").
					Return("", nil, apikey.ErrInvalidKeyName)
			},
		},
		{
			name:        "error - invalid name (too long) returns 400 VALIDATION_ERROR",
			userID:      "1",
			body:        `{"name":"` + strings.Repeat("x", 121) + `"}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "VALIDATION_ERROR",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 1, strings.Repeat("x", 121)).
					Return("", nil, apikey.ErrInvalidKeyName)
			},
		},
		{
			name:        "error - duplicate name returns 400 VALIDATION_ERROR",
			userID:      "2",
			body:        `{"name":"dupe"}`,
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "VALIDATION_ERROR",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 2, "dupe").
					Return("", nil, apikey.ErrDuplicateKeyName)
			},
		},
		{
			name:        "error - internal service error returns 500 INTERNAL_ERROR",
			userID:      "3",
			body:        `{"name":"boom"}`,
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 3, "boom").
					Return("", nil, errors.New("database exploded"))
			},
		},
		{
			name:        "error - service returns nil entity returns 500 INTERNAL_ERROR",
			userID:      "4",
			body:        `{"name":"ghost"}`,
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Create(mock.Anything, 4, "ghost").
					Return("lesstruct_aabbccddeeff_0123456789abcdef0123456789abcdef", nil, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockAPIKeyService(t)
			if tt.setupMock != nil {
				tt.setupMock(mockService)
			}

			handler := handlers.NewAPIKeyHandler(mockService, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodPost, "/api/admin/api-keys", strings.NewReader(tt.body))
			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.CreateAPIKey(w, req)

			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			var body map[string]any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

			if tt.wantErrCode != "" {
				errInfo, ok := body["error"].(map[string]any)
				require.True(t, ok, "expected error envelope")
				assert.Equal(t, tt.wantErrCode, errInfo["code"])
				return
			}

			data, ok := body["data"].(map[string]any)
			require.True(t, ok, "expected data envelope")
			if tt.wantKeyInData {
				key, _ := data["key"].(string)
				assert.NotEmpty(t, key, "full key must be returned on success")
				assert.True(t, strings.HasPrefix(key, apikey.KeyPrefix))
			}
		})
	}
}

func TestAPIKeyHandler_ListAPIKeys(t *testing.T) {
	// Fixed (non-monotonic) timestamps so JSON round-tripping is deterministic.
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	lastUsed := base.Add(-1 * time.Hour)
	expires := base.Add(24 * time.Hour)
	revoked := base.Add(-2 * time.Hour)

	keys := []*apikey.APIKey{
		{
			ID:         1,
			UserID:     1,
			Name:       "Alpha",
			KeyID:      "aabbccddeeff",
			CreatedAt:  base.Add(-48 * time.Hour),
			LastUsedAt: &lastUsed,
			ExpiresAt:  &expires,
		},
		{
			ID:        2,
			UserID:    1,
			Name:      "Beta",
			KeyID:     "112233445566",
			CreatedAt: base.Add(-72 * time.Hour),
			RevokedAt: &revoked,
		},
	}

	tests := []struct {
		name        string
		userID      string
		setupMock   func(*handlersmocks.MockAPIKeyService)
		wantStatus  int
		wantErrCode string
		wantArray   bool
	}{
		{
			name:   "success - returns masked list",
			userID: "1",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					List(mock.Anything, 1).
					Return(keys, nil)
			},
			wantStatus: http.StatusOK,
			wantArray:  true,
		},
		{
			name:        "error - no user in context returns 401 UNAUTHORIZED",
			userID:      "",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
		{
			name:   "error - service returns error returns 500 INTERNAL_ERROR",
			userID: "1",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					List(mock.Anything, 1).
					Return(nil, errors.New("database exploded"))
			},
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockAPIKeyService(t)
			if tt.setupMock != nil {
				tt.setupMock(mockService)
			}

			handler := handlers.NewAPIKeyHandler(mockService, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodGet, "/api/admin/api-keys", nil)
			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.ListAPIKeys(w, req)

			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			var body map[string]any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

			if tt.wantErrCode != "" {
				errInfo, ok := body["error"].(map[string]any)
				require.True(t, ok, "expected error envelope")
				assert.Equal(t, tt.wantErrCode, errInfo["code"])
				return
			}

			if tt.wantArray {
				dataArr, ok := body["data"].([]any)
				require.True(t, ok, "expected data to be an array")
				require.Len(t, dataArr, len(keys))

				// Raw-map checks: forbidden fields never leak and the prefix is masked.
				forbiddenKeys := []string{"keyHash", "secret", "lastUsedIp"}
				for i, item := range dataArr {
					itemMap, ok := item.(map[string]any)
					require.True(t, ok, "expected each item to be an object")
					for _, fk := range forbiddenKeys {
						assert.NotContains(t, itemMap, fk, "list DTO must never contain %s", fk)
					}
					expectedPrefix := apikey.KeyPrefix + keys[i].KeyID + "••••"
					assert.Equal(t, expectedPrefix, itemMap["prefix"])
				}

				// Typed decode: pin the full AC1 field set so a regression that
				// drops a field from the DTO mapping is caught.
				dataBytes, err := json.Marshal(body["data"])
				require.NoError(t, err)
				var got []handlers.APIKeyListItem
				require.NoError(t, json.Unmarshal(dataBytes, &got))
				require.Len(t, got, len(keys))
				for i, want := range keys {
					assert.Equal(t, want.ID, got[i].ID, "id mismatch on item %d", i)
					assert.Equal(t, want.Name, got[i].Name, "name mismatch on item %d", i)
					assert.Equal(t, apikey.DisplayPrefix(want.KeyID), got[i].Prefix, "prefix mismatch on item %d", i)
					assert.True(t, want.CreatedAt.Equal(got[i].CreatedAt), "createdAt mismatch on item %d", i)
					assertTimePtrEqual(t, want.LastUsedAt, got[i].LastUsedAt, "lastUsedAt", i)
					assertTimePtrEqual(t, want.ExpiresAt, got[i].ExpiresAt, "expiresAt", i)
					assertTimePtrEqual(t, want.RevokedAt, got[i].RevokedAt, "revokedAt", i)
				}
			}
		})
	}
}

func TestAPIKeyHandler_RevokeAPIKey(t *testing.T) {
	revokedAt := time.Now().UTC()

	tests := []struct {
		name        string
		userID      string
		idStr       string
		setupMock   func(*handlersmocks.MockAPIKeyService)
		wantStatus  int
		wantErrCode string
		wantRevoked bool
	}{
		{
			name:   "success - active key revoked",
			userID: "1",
			idStr:  "5",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Revoke(mock.Anything, 5, 1).
					Return(&apikey.APIKey{
						ID:        5,
						UserID:    1,
						KeyID:     "aabbccddeeff",
						RevokedAt: &revokedAt,
					}, nil)
			},
			wantStatus:  http.StatusOK,
			wantRevoked: true,
		},
		{
			name:   "success - idempotent revoke returns 200",
			userID: "1",
			idStr:  "6",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				prev := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				m.EXPECT().
					Revoke(mock.Anything, 6, 1).
					Return(&apikey.APIKey{
						ID:        6,
						UserID:    1,
						KeyID:     "112233445566",
						RevokedAt: &prev,
					}, nil)
			},
			wantStatus:  http.StatusOK,
			wantRevoked: true,
		},
		{
			name:   "error - service returns ErrKeyNotFound returns 404 NOT_FOUND",
			userID: "1",
			idStr:  "999",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Revoke(mock.Anything, 999, 1).
					Return(nil, apikey.ErrKeyNotFound)
			},
			wantStatus:  http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:        "error - non-numeric id returns 400 VALIDATION_ERROR",
			userID:      "1",
			idStr:       "abc",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "VALIDATION_ERROR",
		},
		{
			name:        "error - no user in context returns 401 UNAUTHORIZED",
			userID:      "",
			idStr:       "5",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
		{
			name:   "error - service returns generic error returns 500 INTERNAL_ERROR",
			userID: "1",
			idStr:  "7",
			setupMock: func(m *handlersmocks.MockAPIKeyService) {
				m.EXPECT().
					Revoke(mock.Anything, 7, 1).
					Return(nil, errors.New("database exploded"))
			},
			wantStatus:  http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := handlersmocks.NewMockAPIKeyService(t)
			if tt.setupMock != nil {
				tt.setupMock(mockService)
			}

			handler := handlers.NewAPIKeyHandler(mockService, util.NewLogger(os.Stdout))

			req := httptest.NewRequest(http.MethodDelete, "/api/admin/api-keys/"+tt.idStr, nil)
			req.SetPathValue("id", tt.idStr)
			if tt.userID != "" {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.RevokeAPIKey(w, req)

			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			var body map[string]any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

			if tt.wantErrCode != "" {
				errInfo, ok := body["error"].(map[string]any)
				require.True(t, ok, "expected error envelope")
				assert.Equal(t, tt.wantErrCode, errInfo["code"])
				return
			}

			data, ok := body["data"].(map[string]any)
			require.True(t, ok, "expected data envelope")
			if tt.wantRevoked {
				assert.NotEmpty(t, data["id"], "revoked key id must be present")
				assert.NotNil(t, data["revokedAt"], "revokedAt must be populated")
				// Secret hygiene: the response must never leak keyHash or secret.
			assert.NotContains(t, data, "keyHash")
			assert.NotContains(t, data, "secret")
		}
	})
	}
}

// assertTimePtrEqual compares two *time.Time values by instant (monotonic-clock
// safe), treating nil == nil as equal. Used to pin the AC1 nullable timestamp
// fields without flakiness from JSON round-tripping.
func assertTimePtrEqual(t *testing.T, want, got *time.Time, field string, i int) {
	t.Helper()
	if want == nil || got == nil {
		assert.Equal(t, want == nil, got == nil, "%s presence mismatch on item %d", field, i)
		return
	}
	assert.True(t, want.Equal(*got), "%s mismatch on item %d", field, i)
}
