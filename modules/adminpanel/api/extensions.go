package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aristorinjuang/lesstruct/kernel"
)

// ExtensionsVersion is the manifest payload version served by
// GetExtensions. Version policy: bump ONLY on a breaking shape change
// (renamed/removed fields, changed types); a future shell must not render
// unknown major versions. Additive fields never bump — decoders ignore
// unknown JSON fields.
const ExtensionsVersion = 1

// menuEntry is one navigation item contributed to the admin shell.
type menuEntry struct {
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon"`
}

// moduleExtensions is one module's aggregated admin extensions. Empty
// collections serialize as [] (never null) so the contract is stable.
type moduleExtensions struct {
	Name             string      `json:"name"`
	MenuEntries      []menuEntry `json:"menuEntries"`
	SettingsSections []string    `json:"settingsSections"`
	EditorExtensions []string    `json:"editorExtensions"`
}

// extensionsPayload is the versioned manifest aggregation.
type extensionsPayload struct {
	Version int                `json:"version"`
	Modules []moduleExtensions `json:"modules"`
}

func toMenuEntries(entries []kernel.AdminMenuEntry) []menuEntry {
	out := make([]menuEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, menuEntry{
			Label: entry.Label,
			Path:  entry.Path,
			Icon:  entry.Icon,
		})
	}
	return out
}

func toStrings(values []string) []string {
	out := make([]string, 0, len(values))
	out = append(out, values...)
	return out
}

// ExtensionsHandler serves the frozen admin-manifest aggregation.
type ExtensionsHandler struct {
	registry *kernel.Registry
}

// GetExtensions aggregates every registered AdminExtensionProvider in
// registry name order into the versioned manifest payload.
func (h *ExtensionsHandler) GetExtensions(w http.ResponseWriter, r *http.Request) {
	_ = r

	payload := extensionsPayload{
		Version: ExtensionsVersion,
		Modules: []moduleExtensions{},
	}
	if h.registry != nil {
		for _, provider := range kernel.Capabilities[kernel.AdminExtensionProvider](h.registry) {
			manifest := provider.AdminManifest()
			payload.Modules = append(payload.Modules, moduleExtensions{
				Name:             provider.Name(),
				MenuEntries:      toMenuEntries(manifest.MenuEntries),
				SettingsSections: toStrings(manifest.SettingsSections),
				EditorExtensions: toStrings(manifest.EditorExtensions),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  payload,
		"error": nil,
		"meta": map[string]string{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
}

// NewExtensionsHandler creates an ExtensionsHandler over the module
// registry. Aggregation reads the registry live, so late-registered modules
// appear without a rebuild.
func NewExtensionsHandler(registry *kernel.Registry) *ExtensionsHandler {
	return &ExtensionsHandler{
		registry: registry,
	}
}
