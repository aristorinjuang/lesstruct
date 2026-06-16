package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/tetratelabs/wazero"
)

type Loader struct {
	rt             runtime.Runtime
	pluginsDir     string
	hostFunctions  *hostfunctions.Registry
	logger         func(string)
}

func (l Loader) Discover(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(l.pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("reading plugins directory: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}
		paths = append(paths, filepath.Join(l.pluginsDir, entry.Name()))
	}

	return paths, nil
}

// Load loads a single .wasm plugin, along with its optional capability manifest.
// The returned PluginLoadResult includes the parsed manifest if one was found.
func (l Loader) Load(
	ctx context.Context,
	filePath string,
) (plugin.Plugin, *capability.Manifest, error) {
	wasmBytes, err := os.ReadFile(filePath)
	if err != nil {
		return plugin.Plugin{Status: plugin.StatusFailed}, nil, fmt.Errorf(
			"reading %s: %w",
			filePath,
			err,
		)
	}

	compiled, err := l.rt.Runtime().CompileModule(ctx, wasmBytes)
	if err != nil {
		return plugin.Plugin{Status: plugin.StatusFailed}, nil, fmt.Errorf(
			"%w: %v",
			plugin.ErrPluginInvalidFormat,
			err,
		)
	}

	pluginName := strings.TrimSuffix(filepath.Base(filePath), ".wasm")

	// Load capability manifest if present
	manifestPath := strings.TrimSuffix(filePath, ".wasm") + ".manifest"
	manifest, manifestErr := capability.LoadManifest(manifestPath)
	if manifestErr != nil {
		_ = compiled.Close(ctx)
		return plugin.Plugin{Status: plugin.StatusFailed}, nil, fmt.Errorf(
			"loading manifest %s: %w",
			manifestPath,
			manifestErr,
		)
	}

	// If manifest exists, instantiate host functions before the plugin module
	if manifest != nil && l.hostFunctions != nil {
		if _, err := l.hostFunctions.InstantiateModule(ctx, l.rt.Runtime(), manifest); err != nil {
			_ = compiled.Close(ctx)
			return plugin.Plugin{Status: plugin.StatusFailed}, nil, fmt.Errorf(
				"instantiating host functions for %s: %w",
				pluginName,
				err,
			)
		}
		l.logger(fmt.Sprintf(
			"Capability manifest loaded for %s (http=%v, db=%v)",
			pluginName,
			manifest.HasHTTP(),
			manifest.HasDatabase(),
		))
	}

	mod, err := l.rt.Runtime().InstantiateModule(
		ctx,
		compiled,
		wazero.NewModuleConfig().
			WithName(pluginName).
			WithStartFunctions(),
	)
	if err != nil {
		_ = compiled.Close(ctx)
		return plugin.Plugin{Status: plugin.StatusFailed}, manifest, fmt.Errorf(
			"%w: %v",
			plugin.ErrPluginLoadFailed,
			err,
		)
	}

	return plugin.Plugin{
		Name:     pluginName,
		FilePath: filePath,
		Module:   mod,
		Status:   plugin.StatusLoaded,
		LoadedAt: time.Now(),
	}, manifest, nil
}

func (l Loader) LoadAll(
	ctx context.Context,
) ([]plugin.Plugin, []PluginLoadResult) {
	var (
		loaded  []plugin.Plugin
		results []PluginLoadResult
	)

	paths, err := l.Discover(ctx)
	if err != nil {
		l.logger(fmt.Sprintf("Failed to discover plugins: %v", err))
		return nil, nil
	}

	for _, p := range paths {
		plg, manifest, loadErr := l.Load(ctx, p)
		result := PluginLoadResult{
			Plugin:   plg,
			Manifest: manifest,
			Err:      loadErr,
		}
		results = append(results, result)

		if loadErr != nil {
			l.logger(fmt.Sprintf("Failed to load plugin: %s - %v", filepath.Base(p), loadErr))
			continue
		}

		loaded = append(loaded, plg)
		l.logger(fmt.Sprintf("Loaded plugin: %s", filepath.Base(p)))
	}

	if len(loaded) > 1 {
		l.logger(fmt.Sprintf(
			"Loaded %d plugins (cross-platform WASM)",
			len(loaded),
		))
	} else if len(loaded) == 1 {
		l.logger("Loaded plugin: cross-platform WASM compatible")
	}

	return loaded, results
}

func (l Loader) Unload(ctx context.Context, plg *plugin.Plugin) error {
	if plg.Module == nil {
		return fmt.Errorf("cannot unload plugin %q: module is nil", plg.Name)
	}

	if err := plg.Module.Close(ctx); err != nil {
		return fmt.Errorf("closing plugin %q: %w", plg.Name, err)
	}

	plg.Status = plugin.StatusUnloaded
	return nil
}

func (l Loader) EnsurePluginsDir() error {
	if _, err := os.Stat(l.pluginsDir); err != nil {
		if err := os.MkdirAll(l.pluginsDir, 0o755); err != nil {
			return fmt.Errorf("creating plugins directory: %w", err)
		}
		l.logger("Created plugins/ directory")
	}

	return nil
}

func NewLoader(
	rt runtime.Runtime,
	pluginsDir string,
	hostFunctions *hostfunctions.Registry,
	logger func(string),
) Loader {
	if logger == nil {
		logger = func(string) {}
	}
	return Loader{
		rt:            rt,
		pluginsDir:    pluginsDir,
		hostFunctions: hostFunctions,
		logger:        logger,
	}
}
