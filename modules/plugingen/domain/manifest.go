package plugingen

import (
	"fmt"
	"strings"
)

const (
	// manifestVersion is the plugin version stamped into every synthesized
	// sidecar: generated projects always ship the first revision.
	manifestVersion = "0.1.0"
	// manifestMaxMemoryMB is the advisory memory ceiling stamped into every
	// synthesized sidecar. It documents the host runtime's default budget;
	// the host parses but does not enforce it.
	manifestMaxMemoryMB = 64
)

// renderQuotedList renders one TOML array of quoted strings.
func renderQuotedList(key string, values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", strings.TrimSpace(value)))
	}
	return fmt.Sprintf("%s = [%s]\n", key, strings.Join(quoted, ", "))
}

// SynthesizeManifest renders the host capability TOML sidecar for a
// validated project: name, version, advisory memory ceiling, and the [capabilities]
// http/database grants the build step installs next to the .wasm. It is the
// single renderer — server, CLI, and docs all describe this shape.
func SynthesizeManifest(project Project) (string, error) {
	if err := ValidateProjectName(project.Name); err != nil {
		return "", err
	}
	for _, pattern := range project.HTTPPatterns {
		if !IsValidHTTPPattern(pattern) {
			return "", fmt.Errorf("%w: invalid http pattern %q", ErrInvalidManifest, pattern)
		}
	}
	for _, permission := range project.DBPermissions {
		if !IsKnownDBPermission(permission) {
			return "", fmt.Errorf("%w: unknown db permission %q", ErrInvalidManifest, permission)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name = %q\n", project.Name)
	fmt.Fprintf(&b, "version = %q\n", manifestVersion)
	fmt.Fprintf(&b, "max_memory_mb = %d\n", manifestMaxMemoryMB)
	b.WriteString("\n[capabilities]\n")
	b.WriteString(renderQuotedList("http", project.HTTPPatterns))
	b.WriteString(renderQuotedList("database", project.DBPermissions))
	return b.String(), nil
}
