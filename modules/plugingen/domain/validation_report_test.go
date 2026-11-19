package plugingen_test

import (
	"context"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator(t *testing.T) {
	tests := []struct {
		name    string
		input   plugingen.Project
		valid   bool
		wantErr bool
	}{
		{
			name: "success - valid project",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"log_info"},
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  "module example",
				},
				"rationale",
			),
			valid:   true,
			wantErr: false,
		},
		{
			name: "error - bad name",
			input: plugingen.NewProject(
				"Bad!",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "error - missing hook export",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "error - unknown hook",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"nope"},
				nil,
				map[string]string{
					"main.go": "package main\n// hook_nope\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			),
			valid:   false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := plugingen.NewValidator()
			report := validator.Validate(context.Background(), tt.input)
			assert.Equal(t, tt.valid, report.Valid)
			require.NotNil(t, report)
		})
	}
}
