package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendJSON(t *testing.T) {
	w := httptest.NewRecorder()

	resp := appresponse.Response{
		Data: map[string]string{"message": "test"},
	}

	appresponse.SendJSON(w, http.StatusOK, resp)

	assert.Equal(t, http.StatusOK, w.Code, "Status code")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Content-Type")

	var decoded appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&decoded)
	require.NoError(t, err, "Failed to decode response")

	assert.Nil(t, decoded.Error, "Error should be nil for success response")
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"message": "test"}
	appresponse.Success(w, data)

	assert.Equal(t, http.StatusOK, w.Code, "Status code")

	var decoded appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&decoded)
	require.NoError(t, err, "Failed to decode response")

	assert.NotNil(t, decoded.Data, "Data should not be nil for success response")
	assert.Nil(t, decoded.Error, "Error should be nil for success response")
}

func TestSuccessList(t *testing.T) {
	tests := []struct {
		name           string
		items          any
		meta           any
		wantStatus     int
		wantDataLen    int
		wantHasDataKey bool
		wantBodyHas    string // substring the raw body must contain
		wantBodyLacks  string // substring the raw body must NOT contain
		wantNextCursor string
		wantHasMore    bool
	}{
		{
			name:           "empty list renders data as an empty array (no omitempty)",
			items:          []any{},
			meta:           appresponse.ListMeta{Pagination: appresponse.Pagination{HasMore: false}},
			wantStatus:     http.StatusOK,
			wantDataLen:    0,
			wantHasDataKey: true,
			wantBodyHas:    `"data":[]`,
			wantBodyLacks:  `"nextCursor"`,
			wantHasMore:    false,
		},
		{
			name:           "non-empty list renders data array with pagination meta",
			items:          []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
			meta:           appresponse.ListMeta{Pagination: appresponse.Pagination{NextCursor: "MTAw", HasMore: true}},
			wantStatus:     http.StatusOK,
			wantDataLen:    2,
			wantHasDataKey: true,
			wantBodyHas:    `"data":[`,
			wantNextCursor: "MTAw",
			wantHasMore:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			appresponse.SuccessList(w, tt.items, tt.meta)

			assert.Equal(t, tt.wantStatus, w.Code, "status code")
			assert.Contains(t, w.Body.String(), tt.wantBodyHas, "body must contain")
			if tt.wantBodyLacks != "" {
				assert.NotContains(t, w.Body.String(), tt.wantBodyLacks, "body must NOT contain")
			}

			var decoded map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&decoded), "decode list envelope")

			if tt.wantHasDataKey {
				data, ok := decoded["data"].([]any)
				require.True(t, ok, "data must be a JSON array (key present, never omitted)")
				assert.Len(t, data, tt.wantDataLen, "data array length")
			}

			meta, ok := decoded["meta"].(map[string]any)
			require.True(t, ok, "meta must be present")
			pagination, ok := meta["pagination"].(map[string]any)
			require.True(t, ok, "meta.pagination must be present")

			if tt.wantNextCursor != "" {
				assert.Equal(t, tt.wantNextCursor, pagination["nextCursor"], "nextCursor")
			}
			assert.Equal(t, tt.wantHasMore, pagination["hasMore"], "hasMore")
		})
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	appresponse.Error(w, http.StatusUnauthorized, "AUTH_FAILED", "invalid credentials", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Status code")

	var decoded appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&decoded)
	require.NoError(t, err, "Failed to decode response")

	assert.Nil(t, decoded.Data, "Data should be nil for error response")
	require.NotNil(t, decoded.Error, "Error should not be nil for error response")

	errorInfo, ok := decoded.Error.(map[string]any)
	require.True(t, ok, "Error should be a map")

	assert.Equal(t, "AUTH_FAILED", errorInfo["code"], "Error code")
	assert.Equal(t, "invalid credentials", errorInfo["message"], "Error message")
}

func TestError_WithDetails(t *testing.T) {
	w := httptest.NewRecorder()

	details := map[string]string{"field": "username"}
	appresponse.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid input", details)

	var decoded appresponse.Response
	err := json.NewDecoder(w.Body).Decode(&decoded)
	require.NoError(t, err, "Failed to decode response")

	errorInfo, ok := decoded.Error.(map[string]any)
	require.True(t, ok, "Error should be a map")

	assert.NotNil(t, errorInfo["details"], "Error details should not be nil")
}
