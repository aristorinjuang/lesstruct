package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuccess(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected string
	}{
		{
			name:     "success - object data",
			data:     map[string]any{"id": float64(1)},
			expected: `{"data":{"id":1}}`,
		},
		{
			name:     "success - nil data omits key",
			data:     nil,
			expected: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.Success(w, tt.data)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expected, w.Body.String())
		})
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Denied", nil)

	require.Equal(t, http.StatusForbidden, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "INSUFFICIENT_PERMISSIONS", body["error"].(map[string]any)["code"])
	assert.Equal(t, "Denied", body["error"].(map[string]any)["message"])
	_, hasData := body["data"]
	assert.False(t, hasData, "error envelope must not carry data")
}

func TestSuccessList(t *testing.T) {
	tests := []struct {
		name     string
		items    any
		meta     any
		expected string
	}{
		{
			name:     "success - empty list keeps data key",
			items:    []string{},
			meta:     nil,
			expected: `{"data":[]}`,
		},
		{
			name:  "success - list with pagination meta",
			items: []string{"a"},
			meta: httpx.ListMeta{
				Pagination: httpx.Pagination{HasMore: true},
			},
			expected: `{"data":["a"],"meta":{"pagination":{"hasMore":true}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.SuccessList(w, tt.items, tt.meta)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.JSONEq(t, tt.expected, w.Body.String())
		})
	}
}
