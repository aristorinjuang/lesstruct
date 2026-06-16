package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Runtime struct {
	runtime wazero.Runtime
	config  RuntimeConfig
}

func (r Runtime) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}

func (r Runtime) Runtime() wazero.Runtime {
	return r.runtime
}

func (r Runtime) Config() RuntimeConfig {
	return r.config
}

func (r Runtime) GetVersion() string {
	return defaultWazeroVersion
}

func NewRuntime(ctx context.Context, config RuntimeConfig) (Runtime, error) {
	if config.MaxMemoryBytes == 0 {
		config.MaxMemoryBytes = DefaultMaxMemory
	}
	if config.MaxExecutionTime == 0 {
		config.MaxExecutionTime = DefaultMaxExecTime
	}
	if config.Logger == nil {
		config.Logger = func(string) {}
	}

	r := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfigCompiler().WithCloseOnContextDone(true),
	)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return Runtime{}, fmt.Errorf("instantiating WASI: %w", err)
	}

	config.Logger(fmt.Sprintf(
		"wazero runtime initialized successfully (version %s)",
		defaultWazeroVersion,
	))

	return Runtime{
		runtime: r,
		config:  config,
	}, nil
}
