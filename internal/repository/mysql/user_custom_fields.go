package mysql

import (
	"encoding/json"
	"fmt"
	"strings"
)

// marshalCustomFields marshals custom fields map to JSON string.
func marshalCustomFields(fields map[string]any) (any, error) {
	if fields == nil {
		return nil, nil
	}
	cfBytes, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	return string(cfBytes), nil
}

// unmarshalCustomFields unmarshals custom fields from JSON string pointer.
func unmarshalCustomFields(raw *string) map[string]any {
	if raw == nil || *raw == "" {
		return nil
	}
	var fields map[string]any
	_ = json.Unmarshal([]byte(*raw), &fields)
	return fields
}

// isMySQLDuplicateError checks if an error is a MySQL duplicate entry error (Error 1062).
func isMySQLDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "Duplicate entry")
}
