package plugingen_test

import (
	"context"
	"strings"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemainingBranches(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - cover remaining branches",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// isAllowedProjectPath default false with clean unknown file.
			err := plugingen.ValidateProjectFiles(map[string]string{
				"main.go":   "package main",
				"go.mod":    "module x",
				"other.txt": "oops",
			})
			require.Error(t, err)
			// Total size exceeded: 5 files of 500KiB each (each under per-file cap, total over 2MiB).
			big := strings.Repeat("a", 500<<10)
			err = plugingen.ValidateProjectFiles(map[string]string{
				"main.go":      big,
				"go.mod":       big,
				"README.md":    big,
				"build.sh":     big,
				"project.json": big,
			})
			require.Error(t, err)
			// Validator files-invalid branch: missing go.mod.
			report := plugingen.NewValidator().Validate(context.Background(), plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}"},
				"",
			))
			assert.False(t, report.Valid)
			// validateGenerated scanned path: javascript URL passes validator but Scan flags it, so Generate repairs.
			jsProj := plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}\n// javascript:foo",
					"go.mod":  "module x",
				},
				"",
			)
			gen := &fakeGeneration{project: jsProj, repair: validProject()}
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			got, rep, err := svc.Generate(context.Background(), brief)
			require.NoError(t, err)
			assert.True(t, rep.Valid)
			assert.Equal(t, "my-plugin", got.Name)
			assert.Equal(t, tt.wantErr, false)
		})
	}
}
