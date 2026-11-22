package modgen

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// moduleEnvelope mirrors the JSON envelope the model returns: the module
// name, the kernel surfaces it implements, a one-paragraph rationale, and
// the relative-path → content file map.
type moduleEnvelope struct {
	Name      string            `json:"name"`
	Surfaces  []string          `json:"surfaces"`
	Rationale string            `json:"rationale"`
	Files     map[string]string `json:"files"`
}

// stripJSONFences removes markdown fences the model sometimes wraps around
// JSON output. The info string varies (```json, ```JSON, ```go, ...), so
// anything before the first "{" and after the last "}" is trimmed — the
// envelope itself must be one JSON object.
func stripJSONFences(responseText string) string {
	stripped := strings.TrimSpace(responseText)
	stripped = strings.TrimPrefix(stripped, "```json")
	stripped = strings.TrimPrefix(stripped, "```JSON")
	stripped = strings.TrimPrefix(stripped, "```")
	stripped = strings.TrimSuffix(stripped, "```")
	stripped = strings.TrimSpace(stripped)
	if i := strings.IndexByte(stripped, '{'); i > 0 {
		stripped = strings.TrimSpace(stripped[i:])
	}
	if j := strings.LastIndexByte(stripped, '}'); j >= 0 && j < len(stripped)-1 {
		stripped = strings.TrimSpace(stripped[:j+1])
	}
	return stripped
}

// duplicateKeys scans every object in the response — top level and nested —
// for repeated keys. encoding/json silently last-wins on duplicates, which
// could drop half the generated module (e.g. a second "files" map, or two
// "module.go" entries inside "files") with no validation finding.
func duplicateKeys(cleaned string) string {
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	return duplicateKeysValue(decoder)
}

// duplicateKeysValue walks one JSON value, checking every object it
// contains for repeated keys.
func duplicateKeysValue(decoder *json.Decoder) string {
	token, err := decoder.Token()
	if err != nil {
		return ""
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return ""
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return ""
			}
			// The decoder rejects non-string object keys itself ("object
			// member name must be a string), so a nil-error token here is
			// always a string.
			key := token.(string)
			if _, exists := seen[key]; exists {
				return key
			}
			seen[key] = struct{}{}
			if dup := duplicateKeysValue(decoder); dup != "" {
				return dup
			}
		}
		// Consume the closing delimiter so the parent continues in sync.
		_, _ = decoder.Token()
	case '[':
		for decoder.More() {
			if dup := duplicateKeysValue(decoder); dup != "" {
				return dup
			}
		}
		// Consume the closing delimiter so the parent continues in sync.
		_, _ = decoder.Token()
	}
	return ""
}

// ParseModuleEnvelope validates the model's JSON envelope and splits it into
// a Project. The name and files run the slug and allowlist validators;
// markdown fences and surrounding commentary are rejected.
func ParseModuleEnvelope(responseText string) (Project, error) {
	cleaned := stripJSONFences(strings.TrimSpace(responseText))
	if cleaned == "" {
		return Project{}, fmt.Errorf("%w: empty response", ErrGenerationFailed)
	}
	if key := duplicateKeys(cleaned); key != "" {
		return Project{}, fmt.Errorf("%w: duplicate key %q in response", ErrGenerationFailed, key)
	}
	var envelope moduleEnvelope
	if err := json.Unmarshal([]byte(cleaned), &envelope); err != nil {
		return Project{}, fmt.Errorf("%w: response is not valid JSON: %w", ErrGenerationFailed, err)
	}
	name := strings.TrimSpace(envelope.Name)
	if err := ValidateProjectName(name); err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrInvalidProjectName, err)
	}
	if len(envelope.Files) == 0 {
		return Project{}, fmt.Errorf("%w: no files in response", ErrGenerationFailed)
	}
	files := make(map[string]string, len(envelope.Files))
	for filepath, content := range envelope.Files {
		trimmed := strings.TrimSpace(filepath)
		if trimmed != filepath || filepath == "" {
			return Project{}, fmt.Errorf("%w: %q", ErrPathNotAllowed, filepath)
		}
		files[filepath] = content
	}
	surfaces := make([]string, 0, len(envelope.Surfaces))
	for _, surface := range envelope.Surfaces {
		trimmed := strings.TrimSpace(surface)
		if !IsKnownSurface(trimmed) {
			return Project{}, fmt.Errorf("%w: unknown surface %q", ErrInvalidSurfaces, surface)
		}
		if !slices.Contains(surfaces, trimmed) {
			surfaces = append(surfaces, trimmed)
		}
	}
	if err := ValidateProjectFiles(files); err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}
	return NewProject(name, surfaces, files, strings.TrimSpace(envelope.Rationale)), nil
}
