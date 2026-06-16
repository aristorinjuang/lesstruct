package devmode

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/loader"
	"github.com/aristorinjuang/lesstruct/internal/plugin/registry"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 150 * time.Millisecond

type reloadTimer struct {
	timer *time.Timer
	path  string
}

type Watcher struct {
	loader     loader.Loader
	registry   *registry.Registry
	rt         runtime.Runtime
	pluginsDir string
	logger     func(string)
	plugins    map[string]*plugin.Plugin
	mu         sync.Mutex
	fswatcher  *fsnotify.Watcher
	done       chan struct{}
	timers     map[string]*reloadTimer
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopped    bool
}

func (w *Watcher) reload(ctx context.Context, filePath string) {
	defer func() {
		if r := recover(); r != nil {
			w.logger(fmt.Sprintf("panic in reload: %v", r))
		}
	}()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}

	delete(w.timers, filePath)

	if ctx.Err() != nil {
		return
	}

	filename := filepath.Base(filePath)

	// wazero requires closing the old module before instantiating a new one
	// with the same name, so we must unload before loading.
	if oldPlg, exists := w.plugins[filePath]; exists && oldPlg != nil {
		w.registry.Unregister(oldPlg.Name)
		if err := w.loader.Unload(ctx, oldPlg); err != nil {
			w.logger(err.Error())
		}
		delete(w.plugins, filePath)
	}

	newPlg, _, err := w.loader.Load(ctx, filePath)
	if err != nil {
		w.logger("Failed to reload plugin: " + filename + ": " + err.Error())
		return
	}

	if err := registry.DiscoverHooks(ctx, newPlg, w.registry, w.rt, w.logger); err != nil {
		if unloadErr := w.loader.Unload(ctx, &newPlg); unloadErr != nil {
			w.logger("cleanup after failed hook discovery: " + unloadErr.Error())
		}
		w.logger("Failed to reload plugin: " + filename + ": " + err.Error())
		return
	}

	w.plugins[filePath] = &newPlg
	w.logger("Hot-reloaded plugin: " + filename)
}

func (w *Watcher) scheduleReload(ctx context.Context, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return
	}

	if !strings.HasSuffix(absPath, ".wasm") {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}

	if existing, ok := w.timers[absPath]; ok {
		existing.timer.Stop()
	}

	timer := time.AfterFunc(debounceInterval, func() {
		w.reload(ctx, absPath)
	})

	w.timers[absPath] = &reloadTimer{timer: timer, path: absPath}
}

func (w *Watcher) watchLoop(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fswatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.scheduleReload(ctx, event.Name)
			}
		case err, ok := <-w.fswatcher.Errors:
			if !ok {
				return
			}
			w.logger("filesystem watcher error: " + err.Error())
		}
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done != nil {
		return fmt.Errorf("watcher already started")
	}

	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fswatcher = fswatcher
	w.done = make(chan struct{})
	w.timers = make(map[string]*reloadTimer)
	w.stopped = false

	if err := fswatcher.Add(w.pluginsDir); err != nil {
		_ = fswatcher.Close()
		return err
	}

	w.wg.Add(1)
	go w.watchLoop(ctx)

	w.logger("Development mode activated - hot-reload enabled")
	return nil
}

func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		if w.fswatcher != nil {
			_ = w.fswatcher.Close()
		}

		w.mu.Lock()
		w.stopped = true
		for _, t := range w.timers {
			t.timer.Stop()
		}
		w.timers = nil
		w.mu.Unlock()

		if w.done != nil {
			close(w.done)
		}

		w.wg.Wait()

		w.mu.Lock()
		for filePath, plg := range w.plugins {
			w.registry.Unregister(plg.Name)
			if err := w.loader.Unload(context.Background(), plg); err != nil {
				w.logger(err.Error())
			}
			delete(w.plugins, filePath)
		}
		w.mu.Unlock()
	})
}

func NewWatcher(
	ldr loader.Loader,
	reg *registry.Registry,
	rt runtime.Runtime,
	pluginsDir string,
	logger func(string),
) *Watcher {
	if logger == nil {
		logger = func(string) {}
	}
	return &Watcher{
		loader:     ldr,
		registry:   reg,
		rt:         rt,
		pluginsDir: pluginsDir,
		logger:     logger,
		plugins:    make(map[string]*plugin.Plugin),
	}
}
