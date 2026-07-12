package repository

import (
	"encoding/json"
	"fmt"
)

// MarshalCustomFields serialises a custom-fields map into the JSON string shape
// persisted in content_items.custom_fields. A nil map maps to a nil parameter
// (SQL NULL); any other map is JSON-marshalled to a string.
//
// It returns the value as `any` so the result can be passed directly as a SQL
// driver argument (either nil or a string). Shared across the sqlite, mysql,
// and postgresql repositories.
func MarshalCustomFields(fields map[string]any) (any, error) {
	if fields == nil {
		return nil, nil
	}
	cfBytes, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom fields: %w", err)
	}
	return string(cfBytes), nil
}
