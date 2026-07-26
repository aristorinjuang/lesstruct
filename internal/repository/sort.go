package repository

import (
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

// SortDirectionSQL returns the SQL keyword ("ASC" or "DESC") for the given
// domain SortOrder string. An empty or unrecognised value yields "DESC" —
// the safer default for "top N" ranking patterns. The handler layer is
// expected to validate the order string before constructing the filter, but
// the repository stays defensive in case it is bypassed.
//
// This helper is shared across the sqlite, postgresql, and mysql driver
// packages so the defensive default behaviour stays consistent. The string
// it returns is safe to inline into SQL because it can only be one of two
// hardcoded constants — never caller-controlled input.
func SortDirectionSQL(sortOrder string) string {
	if sortOrder == string(contentdomain.SortOrderAsc) {
		return "ASC"
	}
	return "DESC"
}
