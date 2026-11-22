package modgen_test

import (
	"strings"
	"testing"

	modgen "github.com/aristorinjuang/lesstruct/modules/modgen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envelopeJSON(name, surfaces, files string) string {
	return `{"name":` + name + `,"surfaces":` + surfaces + `,"rationale":"R.","files":` + files + `}`
}

func TestParseModuleEnvelope(t *testing.T) {
	validFiles := `{"go.mod":"module x","module.go":"package forum","README.md":"r"}`
	tests := []struct {
		name         string
		input        string
		wantName     string
		wantSurfaces []string
		wantErr      bool
	}{
		{
			name:         "success - valid envelope",
			input:        envelopeJSON(`"forum"`, `["routes","migrations"]`, validFiles),
			wantName:     "forum",
			wantSurfaces: []string{"routes", "migrations"},
			wantErr:      false,
		},
		{
			name:         "success - fences stripped",
			input:        "```json\n" + envelopeJSON(`"forum"`, `[]`, validFiles) + "\n```",
			wantName:     "forum",
			wantSurfaces: []string{},
			wantErr:      false,
		},
		{
			name:         "success - commentary trimmed",
			input:        "here you go {" + strings.TrimPrefix(envelopeJSON(`"forum"`, `[]`, validFiles), "{") + " enjoy",
			wantName:     "forum",
			wantSurfaces: []string{},
			wantErr:      false,
		},
		{
			name:         "success - duplicate surfaces deduped",
			input:        envelopeJSON(`"forum"`, `["routes","routes"]`, validFiles),
			wantName:     "forum",
			wantSurfaces: []string{"routes"},
			wantErr:      false,
		},
		{
			name:    "error - empty response",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "error - truncated object",
			input:   "{",
			wantErr: true,
		},
		{
			name:    "error - broken value",
			input:   `{"a": }`,
			wantErr: true,
		},
		{
			name:    "error - duplicate key",
			input:   `{"name":"forum","name":"other","files":{}}`,
			wantErr: true,
		},
		{
			name:    "error - duplicate nested key",
			input:   `{"name":"forum","files":{"module.go":"package forum","module.go":"package forum"}}`,
			wantErr: true,
		},
		{
			name:    "error - duplicate key in array",
			input:   `{"name":"forum","surfaces":["routes"],"files":{"module.go":"package forum"},"extra":[{"a":1},{"a":1,"a":2}]}`,
			wantErr: true,
		},
		{
			name:    "error - not json",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "error - bad name",
			input:   envelopeJSON(`"Bad Name"`, `[]`, validFiles),
			wantErr: true,
		},
		{
			name:    "error - no files",
			input:   envelopeJSON(`"forum"`, `[]`, `{}`),
			wantErr: true,
		},
		{
			name:    "error - unknown surface",
			input:   envelopeJSON(`"forum"`, `["hooks"]`, validFiles),
			wantErr: true,
		},
		{
			name:    "error - untrimmed path",
			input:   envelopeJSON(`"forum"`, `[]`, `{" go.mod":"module x","module.go":"package forum","README.md":"r"}`),
			wantErr: true,
		},
		{
			name:    "error - files fail contract",
			input:   envelopeJSON(`"forum"`, `[]`, `{"module.go":"package forum"}`),
			wantErr: true,
		},
		{
			name:    "error - non object",
			input:   `[1,2,3]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := modgen.ParseModuleEnvelope(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, project.Name)
			assert.Equal(t, tt.wantSurfaces, project.Surfaces)
			assert.Equal(t, "R.", project.Rationale)
		})
	}
}
