package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMySQLDSN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DSN without any params",
			input:    "user:pass@tcp(localhost:3306)/mydb",
			expected: "user:pass@tcp(localhost:3306)/mydb?clientFoundRows=true",
		},
		{
			name:     "DSN with existing params",
			input:    "user:pass@tcp(localhost:3306)/mydb?parseTime=true&charset=utf8mb4",
			expected: "user:pass@tcp(localhost:3306)/mydb?parseTime=true&charset=utf8mb4&clientFoundRows=true",
		},
		{
			name:     "DSN already has clientFoundRows=true",
			input:    "user:pass@tcp(localhost:3306)/mydb?clientFoundRows=true",
			expected: "user:pass@tcp(localhost:3306)/mydb?clientFoundRows=true",
		},
		{
			name:     "DSN with clientFoundRows=true among other params",
			input:    "user:pass@tcp(localhost:3306)/mydb?parseTime=true&clientFoundRows=true&charset=utf8mb4",
			expected: "user:pass@tcp(localhost:3306)/mydb?parseTime=true&clientFoundRows=true&charset=utf8mb4",
		},
		{
			name:     "DSN with clientFoundRows=1 (already truthy)",
			input:    "user:pass@tcp(localhost:3306)/mydb?clientFoundRows=1",
			expected: "user:pass@tcp(localhost:3306)/mydb?clientFoundRows=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMySQLDSN(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
