package infrastructure_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/aristorinjuang/lesstruct/modules/plugingen/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoBinary resolves a trivial executable for exercising the default run
// seam without a toolchain: echo exits 0 without writing plugin.wasm, so
// the build proceeds through the real subprocess path and fails at the
// missing-artifact gate.
func echoBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo binary not available")
	}
	return path
}

// hookWasm is a minimal WASM module with linear memory and one hook export:
// (memory 1) (func (export "hook_before_save") (param i32 i32) (result i32)
// i32.const 0). Hand-assembled like the host runtime's own fixtures.
var hookWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x05, 0x03, 0x01,
	0x00, 0x01, 0x07, 0x1d, 0x02, 0x06, 0x6d, 0x65,
	0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, 0x10, 0x68,
	0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65, 0x66, 0x6f,
	0x72, 0x65, 0x5f, 0x73, 0x61, 0x76, 0x65, 0x00,
	0x00, 0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x00,
	0x0b,
}

// memoryOnlyWasm has linear memory but no hook export: load-check must
// refuse it.
var memoryOnlyWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x05, 0x03, 0x01,
	0x00, 0x01, 0x07, 0x10, 0x02, 0x06, 0x6d, 0x65,
	0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, 0x03, 0x61,
	0x64, 0x64, 0x00, 0x00, 0x0a, 0x06, 0x01, 0x04,
	0x00, 0x41, 0x00, 0x0b,
}

// noMemoryWasm exports a function but declares no linear memory.
var noMemoryWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01,
	0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x07, 0x01,
	0x03, 0x61, 0x64, 0x64, 0x00, 0x00, 0x0a, 0x09,
	0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a,
	0x0b,
}

// hostImportWasm imports lesstruct.log_info alongside linear memory and a
// hook_before_save export: load-check must stub the host module instead of
// failing with "module[lesstruct] not instantiated".
var hostImportWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x0c, 0x02, 0x60, 0x02, 0x7f, 0x7f, 0x00,
	0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, 0x02, 0x16,
	0x01, 0x09, 0x6c, 0x65, 0x73, 0x73, 0x74, 0x72,
	0x75, 0x63, 0x74, 0x08, 0x6c, 0x6f, 0x67, 0x5f,
	0x69, 0x6e, 0x66, 0x6f, 0x00, 0x00, 0x03, 0x02,
	0x01, 0x01, 0x05, 0x03, 0x01, 0x00, 0x01, 0x07,
	0x1d, 0x02, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72,
	0x79, 0x02, 0x00, 0x10, 0x68, 0x6f, 0x6f, 0x6b,
	0x5f, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x5f,
	0x73, 0x61, 0x76, 0x65, 0x00, 0x01, 0x0a, 0x06,
	0x01, 0x04, 0x00, 0x41, 0x00, 0x0b,
}
// covers lesstruct, so load-check must still refuse it.
var foreignImportWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
	0x01, 0x0c, 0x02, 0x60, 0x02, 0x7f, 0x7f, 0x00,
	0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, 0x02, 0x0b,
	0x01, 0x03, 0x65, 0x6e, 0x76, 0x03, 0x6e, 0x6f,
	0x77, 0x00, 0x00, 0x03, 0x02, 0x01, 0x01, 0x05,
	0x03, 0x01, 0x00, 0x01, 0x07, 0x1d, 0x02, 0x06,
	0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00,
	0x10, 0x68, 0x6f, 0x6f, 0x6b, 0x5f, 0x62, 0x65,
	0x66, 0x6f, 0x72, 0x65, 0x5f, 0x73, 0x61, 0x76,
	0x65, 0x00, 0x01, 0x0a, 0x06, 0x01, 0x04, 0x00,
	0x41, 0x00, 0x0b,
}

func buildProject() plugingendomain.Project {
	return plugingendomain.NewProject(
		"my-plugin",
		"tinygo-test",
		[]string{"before_save"},
		nil,
		map[string]string{
			"main.go": "package main",
			"go.mod":  "module example",
		},
		"",
	)
}

// stubRun returns a run seam that writes wasm into the build dir. A nil wasm
// simulates a toolchain that produced no artifact. The received environment
// is recorded for sandbox assertions.
func stubRun(wasm []byte, err error, gotEnv *[]string) func(context.Context, string, []string, string, ...string) ([]byte, error) {
	return func(_ context.Context, dir string, env []string, _ string, _ ...string) ([]byte, error) {
		if gotEnv != nil {
			*gotEnv = env
		}
		if err != nil {
			return []byte("main.go:3:1: build failure here"), err
		}
		if wasm != nil {
			if writeErr := os.WriteFile(filepath.Join(dir, "plugin.wasm"), wasm, 0o644); writeErr != nil {
				return nil, writeErr
			}
		}
		return []byte("ok"), nil
	}
}

func TestTinyGoBuilderBuild(t *testing.T) {
	tests := []struct {
		name         string
		project      func() plugingendomain.Project
		pluginsDir   func(t *testing.T) string
		run          func(context.Context, string, []string, string, ...string) ([]byte, error)
		devMode      bool
		tinygoPath   string
		wantWasm     []byte
		wantRestart  bool
		wantErr      bool
		wantErrIs    error
		wantErrMatch string
	}{
		{
			name:    "success - installs wasm and manifest without restart in dev mode",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:         stubRun(hookWasm, nil, nil),
			devMode:     true,
			tinygoPath:  "tinygo",
			wantWasm:    hookWasm,
			wantRestart: false,
			wantErr:     false,
		},
		{
			name:    "success - requires restart outside dev mode",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:         stubRun(hookWasm, nil, nil),
			devMode:     false,
			tinygoPath:  "",
			wantWasm:    hookWasm,
			wantRestart: true,
			wantErr:     false,
		},
		{
			name:    "success - lesstruct host imports stubbed for capability plugins",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:         stubRun(hostImportWasm, nil, nil),
			devMode:     true,
			tinygoPath:  "tinygo",
			wantWasm:    hostImportWasm,
			wantRestart: false,
			wantErr:     false,
		},
		{
			name: "error - invalid project name",
			project: func() plugingendomain.Project {
				project := buildProject()
				project.Name = "Bad!"
				return project
			},
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:       stubRun(hookWasm, nil, nil),
			wantErr:   true,
			wantErrIs: plugingendomain.ErrBuildFailed,
		},
		{
			name:    "error - already installed",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "my-plugin.wasm"), hookWasm, 0o644))
				return dir
			},
			run:       stubRun(hookWasm, nil, nil),
			wantErr:   true,
			wantErrIs: plugingendomain.ErrPluginInstalled,
		},
		{
			name:    "error - toolchain failure carries output tail",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:          stubRun(nil, errors.New("exit status 1"), nil),
			wantErr:      true,
			wantErrIs:    plugingendomain.ErrBuildFailed,
			wantErrMatch: "build failure here",
		},
		{
			name:    "error - missing artifact",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:       stubRun(nil, nil, nil),
			wantErr:   true,
			wantErrIs: plugingendomain.ErrBuildFailed,
		},
		{
			name:    "error - wasm without memory rejected",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:          stubRun(noMemoryWasm, nil, nil),
			wantErr:      true,
			wantErrIs:    plugingendomain.ErrBuildFailed,
			wantErrMatch: "linear memory",
		},
		{
			name:    "error - wasm without hook rejected",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:          stubRun(memoryOnlyWasm, nil, nil),
			wantErr:      true,
			wantErrIs:    plugingendomain.ErrBuildFailed,
			wantErrMatch: "hook_",
		},
		{
			name:    "error - garbage bytes rejected",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:       stubRun([]byte("not wasm"), nil, nil),
			wantErr:   true,
			wantErrIs: plugingendomain.ErrBuildFailed,
		},
		{
			name:    "error - empty wasm rejected",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:          stubRun([]byte{}, nil, nil),
			wantErr:      true,
			wantErrIs:    plugingendomain.ErrBuildFailed,
			wantErrMatch: "empty wasm module",
		},
		{
			name:    "error - unknown host module refused",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:          stubRun(foreignImportWasm, nil, nil),
			wantErr:      true,
			wantErrIs:    plugingendomain.ErrBuildFailed,
			wantErrMatch: "module[env]",
		},
		{
			name:    "error - default run seam executes and reports missing artifact",
			project: buildProject,
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:        nil,
			tinygoPath: echoBinary(t),
			wantErr:    true,
			wantErrIs:  plugingendomain.ErrBuildFailed,
		},
		{
			name: "error - traversal path refused",
			project: func() plugingendomain.Project {
				return plugingendomain.NewProject(
					"my-plugin",
					"tinygo-test",
					[]string{"before_save"},
					nil,
					map[string]string{
						"main.go": "package main",
						"go.mod":  "module example",
						"../evil": "x",
					},
					"",
				)
			},
			pluginsDir: func(t *testing.T) string {
				return t.TempDir()
			},
			run:       stubRun(hookWasm, nil, nil),
			wantErr:   true,
			wantErrIs: plugingendomain.ErrBuildFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginsDir := tt.pluginsDir(t)
			builder := infrastructure.NewTinyGoBuilder(tt.tinygoPath, pluginsDir, time.Minute, tt.devMode, tt.run)
			result, err := builder.Build(context.Background(), tt.project())
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.wantErrMatch != "" {
					assert.Contains(t, err.Error(), tt.wantErrMatch)
				}
				return
			}
			require.NoError(t, err)
			assert.True(t, result.Installed)
			assert.Equal(t, tt.wantRestart, result.RequiresRestart)
			wasm, err := os.ReadFile(filepath.Join(pluginsDir, "my-plugin.wasm"))
			require.NoError(t, err)
			assert.Equal(t, tt.wantWasm, wasm)
			manifest, err := os.ReadFile(filepath.Join(pluginsDir, "my-plugin.manifest"))
			require.NoError(t, err)
			expected, err := plugingendomain.SynthesizeManifest(tt.project())
			require.NoError(t, err)
			assert.Equal(t, expected, string(manifest))
		})
	}
}

// TestTinyGoBuilderSandboxEnv pins the build subprocess sandbox: the
// compiler runs with GOPROXY=off and never inherits ambient credentials.
func TestTinyGoBuilderSandboxEnv(t *testing.T) {
	t.Setenv("PLUGGEN_SENTINEL_CREDENTIAL", "leak-me")

	var gotEnv []string
	project := plugingendomain.NewProject(
		"my-plugin",
		"go",
		[]string{"before_save"},
		nil,
		map[string]string{
			"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
			"go.mod":  "module example",
		},
		"",
	)
	pluginsDir := t.TempDir()
	builder := infrastructure.NewTinyGoBuilder("tinygo", pluginsDir, time.Minute, true, stubRun(hookWasm, nil, &gotEnv))
	result, err := builder.Build(context.Background(), project)
	require.NoError(t, err)
	assert.True(t, result.Installed)
	require.NotNil(t, gotEnv)
	assert.Contains(t, gotEnv, "GOPROXY=off")
	for _, entry := range gotEnv {
		assert.NotContains(t, entry, "PLUGGEN_SENTINEL_CREDENTIAL")
	}
}

// TestTinyGoBuilderIntegration compiles a real fixture with the system
// TinyGo. It skips when no toolchain is installed (CI stays hermetic), and
// when the installed TinyGo predates the system Go release.
func TestTinyGoBuilderInstallOrder(t *testing.T) {
	t.Run("manifest install failure leaves no wasm behind", func(t *testing.T) {
		pluginsDir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(pluginsDir, "my-plugin.manifest"), 0o755))
		builder := infrastructure.NewTinyGoBuilder("tinygo", pluginsDir, time.Minute, true, stubRun(hookWasm, nil, nil))
		_, err := builder.Build(context.Background(), buildProject())
		require.Error(t, err)
		assert.ErrorIs(t, err, plugingendomain.ErrBuildFailed)
		_, statErr := os.Stat(filepath.Join(pluginsDir, "my-plugin.wasm"))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("plugin appearing mid-build is refused", func(t *testing.T) {
		pluginsDir := t.TempDir()
		planter := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
			require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "my-plugin.wasm"), hookWasm, 0o644))
			return stubRun(hookWasm, nil, nil)(ctx, dir, env, name, args...)
		}
		builder := infrastructure.NewTinyGoBuilder("tinygo", pluginsDir, time.Minute, true, planter)
		_, err := builder.Build(context.Background(), buildProject())
		require.Error(t, err)
		assert.ErrorIs(t, err, plugingendomain.ErrPluginInstalled)
	})
}

// TestTinyGoBuilderIntegration compiles a real fixture with the system
// TinyGo. It skips when no toolchain is installed (CI stays hermetic), and
// when the installed TinyGo predates the system Go release.
func TestTinyGoBuilderIntegration(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo not installed")
	}
	project := plugingendomain.NewProject(
		"tinygo-e2e",
		"tinygo-test",
		[]string{"before_save"},
		nil,
		map[string]string{
			"main.go": "package main\n\nimport \"unsafe\"\n\nvar resultBuf [4096]byte\n\n//export hook_before_save\nfunc hookBeforeSave(offset uint32, length uint32) uint32 {\n\tinput := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)\n\tn := copy(resultBuf[:], input)\n\t_ = n\n\treturn uint32(uintptr(unsafe.Pointer(&resultBuf[0])))\n}\n\n//export __hook_result_len\nfunc hookResultLen(inputLen uint32) uint32 { return uint32(len(resultBuf)) }\n\nfunc main() {}\n",
			"go.mod":  "module tinygo-e2e\n\ngo 1.23\n",
		},
		"",
	)
	pluginsDir := t.TempDir()
	builder := infrastructure.NewTinyGoBuilder("", pluginsDir, 4*time.Minute, true, nil)
	result, err := builder.Build(context.Background(), project)
	if err != nil && strings.Contains(err.Error(), "requires go version") {
		t.Skipf("system tinygo predates system go: %v", err)
	}
	require.NoError(t, err)
	assert.True(t, result.Installed)
	assert.False(t, result.RequiresRestart)
	info, err := os.Stat(filepath.Join(pluginsDir, "tinygo-e2e.wasm"))
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}
