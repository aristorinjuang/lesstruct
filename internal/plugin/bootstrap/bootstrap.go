package bootstrap

import (
	"context"
	"fmt"
	"sync"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/devmode"
	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
)

type PluginSystem struct {
	runtime      runtime.Runtime
	registry     *registry.Registry
	loader       loader.Loader
	hostFuncs    *hostfunctions.Registry
	watcher      *devmode.Watcher
	plugins      []plugin.Plugin
	logger       func(string)
	closeOnce    sync.Once
}

func (ps *PluginSystem) Close(ctx context.Context) error {
	var err error
	ps.closeOnce.Do(func() {
		if ps.watcher != nil {
			ps.watcher.Stop()
		}

		for i := range ps.plugins {
			plg := &ps.plugins[i]
			ps.registry.Unregister(plg.Name)
			if unloadErr := ps.loader.Unload(ctx, plg); unloadErr != nil {
				ps.logger(fmt.Sprintf("Failed to unload plugin: %s: %v", plg.Name, unloadErr))
			}
		}

		if ps.hostFuncs != nil {
			_ = ps.hostFuncs.Close()
		}

		err = ps.runtime.Close(ctx)
	})
	return err
}

func (ps *PluginSystem) Registry() *registry.Registry {
	return ps.registry
}

func (ps *PluginSystem) Plugins() []plugin.Plugin {
	return ps.plugins
}

func NewPluginSystem(
	ctx context.Context,
	pluginsDir string,
	devMode bool,
	hostFuncs *hostfunctions.Registry,
	logger func(string),
) (*PluginSystem, error) {
	if logger == nil {
		logger = func(string) {}
	}

	rt, err := runtime.NewRuntime(ctx, runtime.RuntimeConfig{Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("creating runtime: %w", err)
	}

	ldr := loader.NewLoader(rt, pluginsDir, hostFuncs, logger)

	if err := ldr.EnsurePluginsDir(); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("ensuring plugins directory: %w", err)
	}

	reg := registry.NewRegistry(logger)

	loadedPlugins, results := ldr.LoadAll(ctx)

	for _, r := range results {
		if r.Err != nil {
			logger(fmt.Sprintf("Failed to load plugin: %v", r.Err))
		}
	}

	for i := range loadedPlugins {
		plg := loadedPlugins[i]
		if err := registry.DiscoverHooks(ctx, plg, reg, rt, logger); err != nil {
			logger(fmt.Sprintf("Failed to discover hooks for plugin %s: %v", plg.Name, err))
		}
	}

	var w *devmode.Watcher
	if devMode {
		w = devmode.NewWatcher(ldr, reg, rt, pluginsDir, logger)
		if err := w.Start(ctx); err != nil {
			logger("Failed to start development mode watcher: " + err.Error())
		}
	} else {
		logger("Production mode activated - startup-load enabled")
	}

	return &PluginSystem{
		runtime:   rt,
		registry:  reg,
		loader:    ldr,
		hostFuncs: hostFuncs,
		watcher:   w,
		plugins:   loadedPlugins,
		logger:    logger,
	}, nil
}
