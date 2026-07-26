package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type publicFieldConfig struct {
	PublicFields []PublicField `toml:"public_field"`
}

// normalisePublicField validates and normalises a single PublicField entry.
// Resource is canonicalised to lowercase. PostType is cleared when Resource is
// "user" (the post_type concept does not apply to user records). Operations
// are canonicalised to lowercase and de-duplicated.
func normalisePublicField(pf PublicField) (PublicField, error) {
	resource := strings.ToLower(strings.TrimSpace(pf.Resource))
	field := strings.TrimSpace(pf.Field)
	postType := strings.ToLower(strings.TrimSpace(pf.PostType))

	if resource != PublicFieldResourceUser && resource != PublicFieldResourceContent {
		return PublicField{}, fmt.Errorf("resource must be %q or %q, got %q",
			PublicFieldResourceUser, PublicFieldResourceContent, pf.Resource)
	}
	if field == "" {
		return PublicField{}, fmt.Errorf("field must be a non-empty slug")
	}

	seenOps := make(map[string]bool)
	ops := make([]string, 0, len(pf.Operations))
	for _, op := range pf.Operations {
		op = strings.ToLower(strings.TrimSpace(op))
		if op != PublicFieldOperationSort && op != PublicFieldOperationFilter && op != PublicFieldOperationExpose {
			return PublicField{}, fmt.Errorf("operations entry must be %q, %q, or %q, got %q",
				PublicFieldOperationSort, PublicFieldOperationFilter, PublicFieldOperationExpose, op)
		}
		if seenOps[op] {
			continue
		}
		seenOps[op] = true
		ops = append(ops, op)
	}
	if len(ops) == 0 {
		return PublicField{}, fmt.Errorf("operations must contain at least one of %q, %q, or %q",
			PublicFieldOperationSort, PublicFieldOperationFilter, PublicFieldOperationExpose)
	}

	if resource == PublicFieldResourceContent && slices.Contains(ops, PublicFieldOperationExpose) {
		return PublicField{}, fmt.Errorf("operations entry %q is only supported for resource %q, not %q",
			PublicFieldOperationExpose, PublicFieldResourceUser, PublicFieldResourceContent)
	}

	if resource == PublicFieldResourceUser {
		postType = ""
	}

	return PublicField{
		Resource:   resource,
		Field:      field,
		PostType:   postType,
		Operations: ops,
	}, nil
}

// PublicFieldResourceUser and PublicFieldResourceContent are the two supported
// values for PublicField.Resource. They map to the two public query endpoints:
// the authors endpoint (users) and the content_items endpoint (content).
const (
	PublicFieldResourceUser    = "user"
	PublicFieldResourceContent = "content"
)

// PublicFieldOperationSort, PublicFieldOperationFilter, and
// PublicFieldOperationExpose are the supported values for PublicField.Operations.
// A field can be allowlisted for any combination.
const (
	PublicFieldOperationSort   = "sort"
	PublicFieldOperationFilter = "filter"
	PublicFieldOperationExpose = "expose"
)

// PublicField is one [[public_field]] block from config.toml. It declares that
// a specific custom-field or system-field slug may be used in a public query
// (filter, sort, or expose) against one of the public query endpoints. Without an
// entry here, the public endpoints reject any cf_<field> / cf_<field>_min /
// cf_<field>_max / sort_by=cf:<field> parameter that references the field with
// a 400 field_not_queryable error. Fields with the "expose" operation are
// included in the response body (e.g. in the publicAuthors[:].publicFields map).
//
// Admin endpoints (e.g. /api/v1/content_items) are NOT gated by this allowlist
// — they remain unrestricted, matching pre-existing behaviour.
type PublicField struct {
	Resource   string   `toml:"resource"`
	Field      string   `toml:"field"`
	PostType   string   `toml:"post_type"`
	Operations []string `toml:"operations"`
}

// PublicFieldRegistry is the immutable, lookup-optimised form of a slice of
// PublicField entries. Hand one of these to a handler and call IsQueryable to
// enforce the allowlist on incoming public queries.
type PublicFieldRegistry struct {
	entries []PublicField
}

// IsQueryable reports whether the named field may be used with the given
// operation against the named resource. When resource is "content", postType
// is matched against the entry's PostType; an entry with an empty PostType
// matches every post type. When resource is "user", postType is ignored.
//
// An empty registry (no [[public_field]] blocks configured) returns false for
// every query — the public endpoints then reject every cf_*/sort_by=cf:*
// parameter. This is the safe default.
func (r *PublicFieldRegistry) IsQueryable(resource, postType, field, operation string) bool {
	if r == nil || field == "" || operation == "" {
		return false
	}

	for _, e := range r.entries {
		if e.Resource != resource {
			continue
		}
		if e.Field != field {
			continue
		}
		if resource == PublicFieldResourceContent && e.PostType != "" && e.PostType != postType {
			continue
		}
		if !slices.Contains(e.Operations, operation) {
			continue
		}
		return true
	}
	return false
}

// ExposedFields returns the field slugs that are allowlisted with the "expose"
// operation for the given resource (and optionally postType). When resource is
// "content", postType is matched against the entry's PostType; an entry with an
// empty PostType matches every post type. When resource is "user", postType is
// ignored. An empty registry returns nil.
func (r *PublicFieldRegistry) ExposedFields(resource, postType string) []string {
	if r == nil {
		return nil
	}

	var exposed []string
	seen := make(map[string]bool)
	for _, e := range r.entries {
		if e.Resource != resource {
			continue
		}
		if resource == PublicFieldResourceContent && e.PostType != "" && e.PostType != postType {
			continue
		}
		if !slices.Contains(e.Operations, PublicFieldOperationExpose) {
			continue
		}
		if seen[e.Field] {
			continue
		}
		seen[e.Field] = true
		exposed = append(exposed, e.Field)
	}

	return exposed
}

// LoadPublicFields reads the [[public_field]] blocks from the same config.toml
// that supplies post types and homepage sections. It mirrors LoadSiteConfig:
// a missing config directory or file yields an empty registry (fully backward
// compatible — every public cf_*/sort_by=cf:* query is then rejected).
func LoadPublicFields(cfg *Config) (*PublicFieldRegistry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if strings.Contains(cfg.ConfigFile, "/") || strings.Contains(cfg.ConfigFile, "\\") || strings.Contains(cfg.ConfigFile, "..") {
		return nil, fmt.Errorf("CONFIG_FILE must not contain path separators or parent directory references")
	}

	if _, err := os.Stat(cfg.ConfigDir); err != nil {
		if os.IsNotExist(err) {
			return &PublicFieldRegistry{}, nil
		}
		return nil, fmt.Errorf("failed to access config directory %s: %w", cfg.ConfigDir, err)
	}

	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &PublicFieldRegistry{}, nil
	}

	var pfc publicFieldConfig
	if err := toml.Unmarshal(data, &pfc); err != nil {
		return nil, fmt.Errorf("failed to parse public_field config: %w", err)
	}

	registry := &PublicFieldRegistry{entries: make([]PublicField, 0, len(pfc.PublicFields))}
	for _, pf := range pfc.PublicFields {
		normalised, err := normalisePublicField(pf)
		if err != nil {
			return nil, fmt.Errorf("public_field %q: %w", pf.Field, err)
		}
		registry.entries = append(registry.entries, normalised)
	}
	return registry, nil
}
