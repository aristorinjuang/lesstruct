package api_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProvider is a test double for the AdminExtensionProvider capability.
type stubProvider struct {
	name     string
	manifest kernel.AdminExtensionManifest
}

func (s *stubProvider) Name() string {
	return s.name
}

func (s *stubProvider) Version() string {
	return "0.1.0"
}

func (s *stubProvider) Requires() []string {
	return nil
}

func (s *stubProvider) Optional() []string {
	return nil
}

func (s *stubProvider) Init(kernel.Kernel) error {
	return nil
}

func (s *stubProvider) Shutdown() error {
	return nil
}

func (s *stubProvider) AdminManifest() kernel.AdminExtensionManifest {
	return s.manifest
}

func decodeExtensionsBody(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Nil(t, envelope["error"])
	meta, ok := envelope["meta"].(map[string]any)
	require.True(t, ok, "envelope must carry a meta object")
	_, err := time.Parse(time.RFC3339, meta["timestamp"].(string))
	require.NoError(t, err, "meta.timestamp must be RFC3339")

	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok, "envelope must carry a data object")
	return data
}

func TestGetExtensions(t *testing.T) {
	tests := []struct {
		name     string
		registry func() *kernel.Registry
		wantData string
	}{
		{
			name: "success - nil registry yields versioned empty payload",
			registry: func() *kernel.Registry {
				return nil
			},
			wantData: `{"version":1,"modules":[]}`,
		},
		{
			name: "success - empty registry yields versioned empty payload",
			registry: func() *kernel.Registry {
				return kernel.NewRegistry()
			},
			wantData: `{"version":1,"modules":[]}`,
		},
		{
			name: "success - providers aggregate in registry name order",
			registry: func() *kernel.Registry {
				registry := kernel.NewRegistry()
				require.NoError(t, registry.Register(&stubProvider{name: "textgen"}))
				require.NoError(t, registry.Register(&stubProvider{
					name: "media",
					manifest: kernel.AdminExtensionManifest{
						MenuEntries: []kernel.AdminMenuEntry{
							{Label: "Media", Path: "/media", Icon: "photo"},
						},
					},
				}))
				return registry
			},
			wantData: `{"version":1,"modules":[` +
				`{"name":"media","menuEntries":[{"label":"Media","path":"/media","icon":"photo"}],"settingsSections":[],"editorExtensions":[]},` +
				`{"name":"textgen","menuEntries":[],"settingsSections":[],"editorExtensions":[]}` +
				`]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.NewExtensionsHandler(tt.registry())

			req := httptest.NewRequest(http.MethodGet, "/api/admin/extensions", nil)
			w := httptest.NewRecorder()
			handler.GetExtensions(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			data := decodeExtensionsBody(t, w.Body.Bytes())
			raw, err := json.Marshal(data)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantData, string(raw))
		})
	}
}

func TestGetExtensions_EnvelopeKeys(t *testing.T) {
	// The top-level envelope keys are part of the frozen contract: a shell
	// built against them must never see a renamed or missing key.
	handler := api.NewExtensionsHandler(kernel.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/extensions", nil)
	w := httptest.NewRecorder()
	handler.GetExtensions(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	assert.ElementsMatch(t, []string{"data", "error", "meta"}, slices.Sorted(maps.Keys(envelope)))

	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok, "data must be an object")
	assert.ElementsMatch(t, []string{"version", "modules"}, slices.Sorted(maps.Keys(data)))
	assert.Equal(t, float64(api.ExtensionsVersion), data["version"])
}
