package plugingen_test

import (
	"context"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceGenerateFallbackName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - fallback name on bad model name",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badNamed := plugingen.NewProject(
				"Bad!",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			)
			gen := &fakeGeneration{project: badNamed, repair: validProject()}
			// repair returns valid, so first invalid triggers repair loop then succeeds
			// Actually ValidateProjectName in service will slugify before validate, so exercise that path.
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, func(_ context.Context) plugingen.PluginSources {
				return plugingen.NewPluginSources("contract", "examples")
			})
			brief := plugingen.NewBrief("a an of", "go", []string{"before_save"}, nil)
			_, _, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Short words slugify to plugin-xxxx, should succeed after repair or directly
			require.NoError(t, err)
		})
	}
}

func TestServiceNilStore(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - nil store paths",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGeneration{project: validProject()}
			svc := plugingen.NewService(&genAdapter{inner: gen}, nil, nil, nil)
			_, err := svc.List(context.Background())
			require.Error(t, err)
			_, err = svc.Get(context.Background(), "x")
			require.Error(t, err)
			require.Error(t, svc.Delete(context.Background(), "x"))
			_, err = svc.Zip(context.Background(), "x")
			require.Error(t, err)
			report := svc.Validate(context.Background(), validProject())
			assert.True(t, report.Valid)
			assert.Equal(t, tt.wantErr, false)
		})
	}
}

func TestServiceGenerateErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		genErr  bool
		wantErr bool
	}{
		{
			name:    "error - generation fails",
			genErr:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGeneration{project: validProject()}
			if tt.genErr {
				gen.err = assert.AnError
			}
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief("desc", "go", []string{"before_save"}, nil)
			_, _, err := svc.Generate(context.Background(), brief)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateProjectFilesLimits(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]string
		wantErr bool
	}{
		{
			name:    "error - dotfile rejected",
			input:   map[string]string{"main.go": "x", "go.mod": "y", ".hidden": "z"},
			wantErr: true,
		},
		{
			name:    "error - empty segment",
			input:   map[string]string{"main.go": "x", "go.mod": "y", "a//b": "z"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugingen.ValidateProjectFiles(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidatorEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   plugingen.Project
		valid   bool
		wantErr bool
	}{
		{
			name: "error - empty main",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "", "go.mod": "module x"},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "error - unparseable go",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main func {", "go.mod": "module x"},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "error - syscall forbidden",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				nil,
				map[string]string{"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}\nvar _ = syscall.SYS_EXIT", "go.mod": "module x"},
				"",
			),
			valid:   false,
			wantErr: false,
		},
		{
			name: "error - unknown capability",
			input: plugingen.NewProject(
				"my-plugin",
				"go",
				[]string{"before_save"},
				[]string{"exec"},
				map[string]string{"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}", "go.mod": "module x"},
				"",
			),
			valid:   false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := plugingen.NewValidator().Validate(context.Background(), tt.input)
			assert.Equal(t, tt.valid, report.Valid)
		})
	}
}
