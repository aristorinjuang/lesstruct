package plugin

import (
	"errors"
	"time"

	"github.com/tetratelabs/wazero/api"
)

type Status string

const (
	StatusLoaded   Status = "loaded"
	StatusFailed   Status = "failed"
	StatusUnloaded Status = "unloaded"
)

func (s Status) String() string {
	return string(s)
}

type Plugin struct {
	Name     string
	FilePath string
	Module   api.Module
	Status   Status
	LoadedAt time.Time
}

var (
	ErrPluginNotFound     = errors.New("plugin not found")
	ErrPluginInvalidFormat = errors.New("invalid WASM format")
	ErrPluginLoadFailed    = errors.New("plugin load failed")
)
