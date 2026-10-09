package response_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appresponse "github.com/aristorinjuang/lesstruct/internal/api/response"
	"github.com/aristorinjuang/lesstruct/kernel/httpx"
	"github.com/stretchr/testify/assert"
)

// TestKernelCompat is the wire-format contract between core and modules: every
// envelope the kernel's httpx package writes must be byte-identical to the
// core writer's output for the same inputs. Clients cannot tell whether a
// route is served by core or by a module. If this test fails, one side
// changed the envelope — update both in the same change.
func TestKernelCompat(t *testing.T) {
	cases := []struct {
		name string
		core func(w *httptest.ResponseRecorder)
		mod  func(w *httptest.ResponseRecorder)
	}{
		{
			name: "success - object data",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.Success(w, map[string]any{"id": float64(1)})
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.Success(w, map[string]any{"id": float64(1)})
			},
		},
		{
			name: "success - nil data",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.Success(w, nil)
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.Success(w, nil)
			},
		},
		{
			name: "error - without details",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Denied", nil)
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.Error(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Denied", nil)
			},
		},
		{
			name: "error - with details",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad", map[string]any{"field": "title"})
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad", map[string]any{"field": "title"})
			},
		},
		{
			name: "list - empty slice keeps data key",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.SuccessList(w, []string{}, nil)
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.SuccessList(w, []string{}, nil)
			},
		},
		{
			name: "list - items with pagination meta",
			core: func(w *httptest.ResponseRecorder) {
				appresponse.SuccessList(w, []string{"a"}, appresponse.ListMeta{
					Pagination: appresponse.Pagination{HasMore: true},
				})
			},
			mod: func(w *httptest.ResponseRecorder) {
				httpx.SuccessList(w, []string{"a"}, httpx.ListMeta{
					Pagination: httpx.Pagination{HasMore: true},
				})
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			coreRecorder := httptest.NewRecorder()
			tt.core(coreRecorder)
			modRecorder := httptest.NewRecorder()
			tt.mod(modRecorder)

			assert.Equal(t, coreRecorder.Code, modRecorder.Code, "status codes must match")
			assert.Equal(
				t,
				coreRecorder.Header().Get("Content-Type"),
				modRecorder.Header().Get("Content-Type"),
				"content types must match",
			)
			assert.JSONEq(t, coreRecorder.Body.String(), modRecorder.Body.String(), "envelope bytes must match")
		})
	}
}
