package themegen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// themeEnvelope mirrors the JSON envelope the model returns: the theme slug,
// a one-paragraph rationale, and the relative-path → content file map.
type themeEnvelope struct {
	Name      string            `json:"name"`
	Rationale string            `json:"rationale"`
	Files     map[string]string `json:"files"`
}

// stripJSONFences removes markdown fences the model sometimes wraps around
// JSON output.
func stripJSONFences(responseText string) string {
	stripped := strings.TrimSpace(responseText)
	stripped = strings.TrimPrefix(stripped, "```json")
	stripped = strings.TrimPrefix(stripped, "```")
	stripped = strings.TrimSuffix(stripped, "```")
	return strings.TrimSpace(stripped)
}

// ParseThemeEnvelope validates the model's JSON envelope and splits it into
// a Theme. The name and files run the slug and allowlist validators;
// markdown fences and surrounding commentary are rejected.
func ParseThemeEnvelope(responseText string, edition Edition) (Theme, error) {
	cleaned := stripJSONFences(strings.TrimSpace(responseText))
	if cleaned == "" {
		return Theme{}, fmt.Errorf("%w: empty response", ErrGenerationFailed)
	}
	var envelope themeEnvelope
	if err := json.Unmarshal([]byte(cleaned), &envelope); err != nil {
		return Theme{}, fmt.Errorf("%w: response is not valid JSON: %w", ErrGenerationFailed, err)
	}
	name := strings.TrimSpace(envelope.Name)
	if err := ValidateThemeName(name); err != nil {
		return Theme{}, fmt.Errorf("%w: %v", ErrInvalidThemeName, err)
	}
	if len(envelope.Files) == 0 {
		return Theme{}, fmt.Errorf("%w: no files in response", ErrGenerationFailed)
	}
	files := make(map[string]string, len(envelope.Files))
	for path, content := range envelope.Files {
		trimmed := strings.TrimSpace(path)
		if trimmed != path || path == "" {
			return Theme{}, fmt.Errorf("%w: %q", ErrPathNotAllowed, path)
		}
		files[path] = content
	}
	if err := ValidateThemeFiles(files); err != nil {
		return Theme{}, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}
	return NewTheme(name, edition, files, strings.TrimSpace(envelope.Rationale)), nil
}
