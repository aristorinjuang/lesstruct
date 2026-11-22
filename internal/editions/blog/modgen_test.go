package blog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModuleSources(t *testing.T) {
	tests := []struct {
		name          string
		kernelVersion string
		wantErr       bool
	}{
		{
			name:          "success - pinned version",
			kernelVersion: "v1.6.0",
			wantErr:       false,
		},
		{
			name:          "success - empty version degrades",
			kernelVersion: "",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := newModuleSources(tt.kernelVersion)(context.Background())
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.kernelVersion, sources.KernelVersion)
		})
	}
}

func TestResolveKernelVersion(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - resolves without panic",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := resolveKernelVersion()
			require.Equal(t, tt.wantErr, false)
			assert.NotContains(t, version, "devel")
		})
	}
}

func TestNewModuleApplier(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func() *config.Config
		wantReady bool
		wantErr   bool
	}{
		{
			name: "success - disabled reads not ready",
			cfg: func() *config.Config {
				return &config.Config{}
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name: "success - unusable root reads not ready",
			cfg: func() *config.Config {
				return &config.Config{
					ModuleApplyEnabled: true,
					ModuleApplyRoot:    "\x00",
				}
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name: "success - missing toolchain reads not ready",
			cfg: func() *config.Config {
				return &config.Config{
					ModuleApplyEnabled: true,
					ModuleApplyRoot:    t.TempDir(),
					GoPath:             "definitely-not-a-go-binary",
				}
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name: "success - non-checkout root reads not ready",
			cfg: func() *config.Config {
				return &config.Config{
					ModuleApplyEnabled: true,
					ModuleApplyRoot:    t.TempDir(),
				}
			},
			wantReady: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier, ready := newModuleApplier(tt.cfg(), "v1.6.0")
			require.Equal(t, tt.wantErr, false)
			assert.Equal(t, tt.wantReady, ready)
			if tt.wantReady {
				assert.NotNil(t, applier)
				return
			}
			assert.Nil(t, applier)
		})
	}
}

func TestNewModuleApplierReady(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "success - enabled checkout with toolchain wires the applier",
			setup: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/aristorinjuang/lesstruct\n\ngo 1.27\n"), 0o644))
				require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
				return root
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setup(t)
			cfg := &config.Config{ModuleApplyEnabled: true, ModuleApplyRoot: root}
			applier, ready := newModuleApplier(cfg, "v1.6.0")
			require.Equal(t, tt.wantErr, false)
			assert.True(t, ready, "enabled checkout with toolchain must report ready")
			assert.NotNil(t, applier)
		})
	}
}

func TestResolveKernelVersionFromGoMod(t *testing.T) {
	tests := []struct {
		name    string
		goMod   string
		noFile  bool
		foreign bool
		want    string
		wantErr bool
	}{
		{
			name:    "success - single line require",
			goMod:   "module github.com/aristorinjuang/lesstruct\n\ngo 1.27\n\nrequire github.com/aristorinjuang/lesstruct/kernel v1.6.0\n",
			want:    "v1.6.0",
			wantErr: false,
		},
		{
			name:    "success - block require",
			goMod:   "module github.com/aristorinjuang/lesstruct\n\ngo 1.27\n\nrequire (\n\tgithub.com/aristorinjuang/lesstruct/kernel v1.5.2\n)\n",
			want:    "v1.5.2",
			wantErr: false,
		},
		{
			name:    "success - missing kernel require",
			goMod:   "module github.com/aristorinjuang/lesstruct\n\ngo 1.27\n",
			want:    "",
			wantErr: false,
		},
		{
			name:    "success - foreign module",
			want:    "",
			wantErr: false,
		},
		{
			name:    "success - missing file",
			noFile:  true,
			want:    "",
			wantErr: false,
		},
		{
			name:    "success - replace directives ignored",
			goMod:   "module github.com/aristorinjuang/lesstruct\n\nreplace github.com/aristorinjuang/lesstruct/kernel => ../kernel\n",
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if !tt.noFile {
				content := tt.goMod
				if tt.foreign {
					content = "module example.com/other\n"
				}
				require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o644))
			}
			assert.Equal(t, tt.want, resolveKernelVersionFromGoMod(root))
			require.Equal(t, tt.wantErr, false)
		})
	}
}
