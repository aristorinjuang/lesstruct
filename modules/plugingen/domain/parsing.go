package plugingen

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// projectEnvelope mirrors the JSON envelope the model returns: the plugin
// slug, the language, the hook list, a one-paragraph rationale, the
// capability grants the TOML sidecar is synthesized from, and the
// relative-path → content file map.
type projectEnvelope struct {
	Name          string            `json:"name"`
	Language      string            `json:"language"`
	Hooks         []string          `json:"hooks"`
	Capabilities  []string          `json:"capabilities"`
	HTTPPatterns  []string          `json:"httpPatterns"`
	DBPermissions []string          `json:"dbPermissions"`
	Rationale     string            `json:"rationale"`
	Files         map[string]string `json:"files"`
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

// duplicateKeys scans the top-level object for repeated keys. encoding/json
// silently last-wins on duplicates, which could drop half the generated
// project (e.g. a second "files" map) with no validation finding.
func duplicateKeys(cleaned string) string {
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	token, err := decoder.Token()
	if err != nil {
		return ""
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return ""
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		// The decoder rejects non-string object keys itself ("object
		// member name must be a string"), so a nil-error token here is
		// always a string.
		key := token.(string)
		if _, exists := seen[key]; exists {
			return key
		}
		seen[key] = struct{}{}
		var skip any
		if err := decoder.Decode(&skip); err != nil {
			return ""
		}
	}
	return ""
}

// ParseProjectEnvelope validates the model's JSON envelope and splits it into
// a Project. The name and files run the slug and allowlist validators;
// markdown fences and surrounding commentary are rejected.
func ParseProjectEnvelope(responseText string) (Project, error) {
	cleaned := stripJSONFences(strings.TrimSpace(responseText))
	if cleaned == "" {
		return Project{}, fmt.Errorf("%w: empty response", ErrGenerationFailed)
	}
	if key := duplicateKeys(cleaned); key != "" {
		return Project{}, fmt.Errorf("%w: duplicate key %q in response", ErrGenerationFailed, key)
	}
	var envelope projectEnvelope
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
	for path, content := range envelope.Files {
		trimmed := strings.TrimSpace(path)
		if trimmed != path || path == "" {
			return Project{}, fmt.Errorf("%w: %q", ErrPathNotAllowed, path)
		}
		files[path] = content
	}
	language := strings.TrimSpace(envelope.Language)
	if language == "" {
		language = "go"
	}
	if language != "go" {
		return Project{}, fmt.Errorf("%w: unsupported language %q", ErrGenerationFailed, envelope.Language)
	}
	if len(envelope.Hooks) == 0 {
		return Project{}, fmt.Errorf("%w: no hooks in response", ErrGenerationFailed)
	}
	hooks := make([]string, 0, len(envelope.Hooks))
	for _, hook := range envelope.Hooks {
		trimmed := strings.TrimSpace(hook)
		if !IsKnownHook(trimmed) {
			return Project{}, fmt.Errorf("%w: unknown hook %q", ErrInvalidHooks, hook)
		}
		if !slices.Contains(hooks, trimmed) {
			hooks = append(hooks, trimmed)
		}
	}
	capabilities := make([]string, 0, len(envelope.Capabilities))
	for _, capability := range envelope.Capabilities {
		trimmed := strings.TrimSpace(capability)
		if !IsKnownCapability(trimmed) {
			return Project{}, fmt.Errorf("%w: unknown capability %q", ErrInvalidCapabilities, capability)
		}
		if !slices.Contains(capabilities, trimmed) {
			capabilities = append(capabilities, trimmed)
		}
	}
	httpPatterns := make([]string, 0, len(envelope.HTTPPatterns))
	for _, pattern := range envelope.HTTPPatterns {
		trimmed := strings.TrimSpace(pattern)
		if !IsValidHTTPPattern(trimmed) {
			return Project{}, fmt.Errorf("%w: invalid http pattern %q", ErrInvalidHTTPPatterns, pattern)
		}
		httpPatterns = append(httpPatterns, trimmed)
	}
	dbPermissions := make([]string, 0, len(envelope.DBPermissions))
	for _, permission := range envelope.DBPermissions {
		trimmed := strings.TrimSpace(permission)
		if !IsKnownDBPermission(trimmed) {
			return Project{}, fmt.Errorf("%w: unknown db permission %q", ErrInvalidDBPermissions, permission)
		}
		dbPermissions = append(dbPermissions, trimmed)
	}
	if err := ValidateProjectFiles(files); err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}
	project := NewProject(name, language, hooks, capabilities, files, strings.TrimSpace(envelope.Rationale))
	project.HTTPPatterns = httpPatterns
	project.DBPermissions = dbPermissions
	return project, nil
}
