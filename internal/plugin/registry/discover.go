package registry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/plugin/runtime"
)

const wasmBaseOffset = 65536

func makeWasmHandler(
	plg plugin.Plugin,
	funcName string,
	rt runtime.Runtime,
) plugin.HookHandler {
	return func(ctx context.Context, data []byte) ([]byte, error) {
		mem := plg.Module.Memory()
		if mem == nil {
			return nil, fmt.Errorf("plugin %q has no memory", plg.Name)
		}

		if !mem.Write(wasmBaseOffset, data) {
			return nil, fmt.Errorf("failed to write data to plugin %q memory", plg.Name)
		}

		dataLen := uint64(len(data))
		result := rt.ExecuteFunc(ctx, plg.Module, funcName, wasmBaseOffset, dataLen)
		if result.Err != nil {
			return nil, result.Err
		}

		if len(result.Results) == 0 {
			return nil, fmt.Errorf("plugin %q hook %q returned no results", plg.Name, funcName)
		}

		resultOffset := uint32(result.Results[0])
		if resultOffset == 0 {
			return nil, nil
		}

		resultLen := wasmResultLen(plg, rt, ctx, uint32(dataLen))

		resultData, ok := mem.Read(resultOffset, resultLen)
		if !ok {
			return nil, fmt.Errorf("failed to read result from plugin %q memory", plg.Name)
		}

		return resultData, nil
	}
}

func wasmResultLen(
	plg plugin.Plugin,
	rt runtime.Runtime,
	ctx context.Context,
	inputLen uint32,
) uint32 {
	fn := plg.Module.ExportedFunction("__hook_result_len")
	if fn == nil {
		return inputLen
	}

	result := rt.ExecuteFunc(ctx, plg.Module, "__hook_result_len", uint64(inputLen))
	if result.Err != nil || len(result.Results) == 0 {
		return inputLen
	}

	return uint32(result.Results[0])
}

func DiscoverHooks(
	ctx context.Context,
	plg plugin.Plugin,
	reg *Registry,
	rt runtime.Runtime,
	logger func(string),
) error {
	exports := plg.Module.ExportedFunctionDefinitions()
	var discovered []string

	for name := range exports {
		if !strings.HasPrefix(name, "hook_") {
			continue
		}

		displayName := strings.TrimPrefix(name, "hook_")
		if displayName == "" {
			continue
		}

		canonicalName := plugin.ResolveWasmHookName(displayName)
		handler := makeWasmHandler(plg, name, rt)

		if err := reg.Register(plugin.HookRegistration{
			PluginName:  plg.Name,
			HookName:    canonicalName,
			Priority:    plugin.DefaultPriority,
			Handler:     handler,
			FailureMode: plugin.DefaultFailureMode,
		}); err != nil {
			return fmt.Errorf("registering hook %q: %w", displayName, err)
		}

		discovered = append(discovered, displayName)
	}

	if len(discovered) > 0 {
		sort.Strings(discovered)
		logger(fmt.Sprintf(
			"Registered hooks: %s",
			strings.Join(discovered, ", "),
		))
	}

	return nil
}
