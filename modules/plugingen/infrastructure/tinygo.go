package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	plugingendomain "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
)

const (
	// defaultTinyGoPath is the TinyGo binary name resolved through PATH when
	// the operator does not set TINYGO_PATH.
	defaultTinyGoPath = "tinygo"
	// defaultBuildTimeout bounds one server-side TinyGo compile. Cold TinyGo
	// compiles can run past a minute, so the budget carries headroom over the
	// API's own 5-minute request deadline.
	defaultBuildTimeout = 5 * time.Minute
	// maxBuildOutputBytes caps how much compiler output a build failure
	// carries: the tail that usually names the broken file and line.
	maxBuildOutputBytes = 8 << 10
	// wasmFilename is the compiled artifact name inside the temp build dir.
	wasmFilename = "plugin.wasm"
)

// runFunc executes a command in dir with env and returns its combined
// output. It is the seam unit tests stub to avoid shelling out to a real
// toolchain; the env parameter lets tests assert the sandbox.
type runFunc func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error)

// defaultRun shells out to the named binary with a minimal, offline-safe
// environment: the build inherits PATH, HOME, TMPDIR, and the Go module
// cache locations, forces GOPROXY=off, and inherits nothing else, so a
// generated go.mod cannot leak the host's credentials or reach the network.
func defaultRun(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

// buildEnv assembles the build subprocess environment.
func buildEnv() []string {
	env := []string{"GOPROXY=off"}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "GOCACHE", "GOPATH", "GOMODCACHE"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// writeProjectFiles materializes the project file map into dir. Paths are
// validated on the generation path, so a traversal here is defense in depth.
func writeProjectFiles(dir string, files map[string]string) error {
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if !strings.HasPrefix(full, dir+string(os.PathSeparator)) {
			return fmt.Errorf("%w: %q", plugingendomain.ErrPathNotAllowed, path)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("failed to create build subdir: %w", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write build file: %w", err)
		}
	}
	return nil
}

// buildOutputTail keeps the failure-signaling tail of compiler output.
func buildOutputTail(output []byte) string {
	trimmed := strings.TrimSpace(string(output))
	if len(trimmed) > maxBuildOutputBytes {
		trimmed = trimmed[len(trimmed)-maxBuildOutputBytes:]
	}
	return trimmed
}

// TinyGoBuilder implements plugingendomain.Builder by compiling the project
// with TinyGo (WASI target), load-checking the result in wazero, and
// installing the .wasm plus the synthesized capability manifest into the
// host's plugins directory.
type TinyGoBuilder struct {
	tinygoPath   string
	pluginsDir   string
	buildTimeout time.Duration
	devMode      bool
	run          runFunc
}

func (b *TinyGoBuilder) buildTimeoutOrDefault() time.Duration {
	if b.buildTimeout > 0 {
		return b.buildTimeout
	}
	return defaultBuildTimeout
}

func (b *TinyGoBuilder) runOrDefault() runFunc {
	if b.run != nil {
		return b.run
	}
	return defaultRun
}

// installFile atomically installs one build artifact under name into the
// plugins directory: write to a temp sibling, then rename over it.
func (b *TinyGoBuilder) installFile(name string, data []byte) error {
	tmp, err := os.CreateTemp(b.pluginsDir, ".plugingen-install-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to stage install file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to stage install file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to stage install file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("failed to stage install file: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(b.pluginsDir, name)); err != nil {
		return fmt.Errorf("failed to install %s: %w", name, err)
	}
	return nil
}

// Build compiles the project, load-checks the result, and installs the
// manifest plus the .wasm into the host's plugins directory. The manifest
// installs first: a DEV_MODE watcher reloads on .wasm changes, so a
// wasm-first order could hot-load a plugin whose manifest (and grants) is
// not installed yet. An existing installed plugin refuses with
// ErrPluginInstalled; a compile or load failure wraps ErrBuildFailed with
// the signaling tail.
func (b *TinyGoBuilder) Build(ctx context.Context, project plugingendomain.Project) (plugingendomain.BuildResult, error) {
	manifest, err := plugingendomain.SynthesizeManifest(project)
	if err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	if _, err := os.Stat(filepath.Join(b.pluginsDir, project.Name+".wasm")); err == nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %q is already installed", plugingendomain.ErrPluginInstalled, project.Name)
	}
	buildDir, err := os.MkdirTemp("", "plugingen-build-*")
	if err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	defer func() { _ = os.RemoveAll(buildDir) }()
	if err := writeProjectFiles(buildDir, project.Files); err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	buildCtx, cancel := context.WithTimeout(ctx, b.buildTimeoutOrDefault())
	defer cancel()
	output, err := b.runOrDefault()(buildCtx, buildDir, buildEnv(), b.tinygoPath, "build", "-o", wasmFilename, "-target", "wasi", ".")
	if err != nil {
		reason := buildOutputTail(output)
		if reason == "" {
			reason = err.Error()
		}
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: tinygo build failed: %s", plugingendomain.ErrBuildFailed, reason)
	}
	wasmBytes, err := os.ReadFile(filepath.Join(buildDir, wasmFilename))
	if err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	if err := loadCheckWasm(ctx, wasmBytes); err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	// Re-check immediately before installing: the minutes-long compile
	// window above admits a manually installed same-name plugin that the
	// pre-check could not see.
	if _, err := os.Stat(filepath.Join(b.pluginsDir, project.Name+".wasm")); err == nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %q appeared during the build", plugingendomain.ErrPluginInstalled, project.Name)
	}
	if err := b.installFile(project.Name+".manifest", []byte(manifest)); err != nil {
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	if err := b.installFile(project.Name+".wasm", wasmBytes); err != nil {
		_ = os.Remove(filepath.Join(b.pluginsDir, project.Name+".manifest"))
		return plugingendomain.BuildResult{}, fmt.Errorf("%w: %v", plugingendomain.ErrBuildFailed, err)
	}
	return plugingendomain.NewBuildResult(true, !b.devMode), nil
}

// NewTinyGoBuilder builds a server-side WASM builder. tinygoPath names the
// TinyGo binary ("" resolves to PATH lookup); pluginsDir is the host's
// plugins directory; buildTimeout caps one compile (<=0 selects the
// default); devMode reports no restart (the watcher hot-reloads instead).
// The run seam defaults to a real subprocess; tests stub it.
func NewTinyGoBuilder(tinygoPath, pluginsDir string, buildTimeout time.Duration, devMode bool, run runFunc) *TinyGoBuilder {
	if strings.TrimSpace(tinygoPath) == "" {
		tinygoPath = defaultTinyGoPath
	}
	return &TinyGoBuilder{
		tinygoPath:   tinygoPath,
		pluginsDir:   pluginsDir,
		buildTimeout: buildTimeout,
		devMode:      devMode,
		run:          run,
	}
}
