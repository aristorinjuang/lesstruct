package repository

import "strings"

// SplitCSV splits a comma-separated string (typically the output of a SQL
// GROUP_CONCAT / STRING_AGG(DISTINCT …) aggregation) into a clean []string,
// dropping empty entries. It returns a non-nil empty slice for blank input so
// callers always get a slice that serialises to JSON as [] rather than null.
//
// It is shared across the sqlite, mysql, and postgresql content repositories,
// which all produce the same comma-joined shape for aggregated columns.
func SplitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
