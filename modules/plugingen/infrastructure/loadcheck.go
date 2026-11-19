package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const (
	// lesstructHostModuleName is the wazero host module name the production
	// loader registers capability functions under (see the hostfunctions
	// registry). The load-check recreates it as signature-derived no-ops
	// so capability-using plugins pass; it cannot share the registry
	// constant — nested modules must not depend on lesstruct/internal
	// (see make depguard).
	lesstructHostModuleName = "lesstruct"
)

// stubLesstructHostModule instantiates a no-op host module for every
// lesstruct import the guest declares, using the exact signature wazero
// reports for each. The stubs never execute (start functions are skipped);
// they only satisfy import resolution so the shape check below runs on
// capability-using plugins. Imports from any other module are left
// alone — an unknown import is a genuine validation failure.
func stubLesstructHostModule(ctx context.Context, rt wazero.Runtime, compiled wazero.CompiledModule) error {
	builder := rt.NewHostModuleBuilder(lesstructHostModuleName)
	stubbed := make(map[string]struct{})
	for _, fn := range compiled.ImportedFunctions() {
		// Imported functions carry their module/name on Import, not on
		// the ModuleName/Name accessors (those describe exports).
		moduleName, name, isImport := fn.Import()
		if !isImport || moduleName != lesstructHostModuleName {
			continue
		}
		if _, ok := stubbed[name]; ok {
			continue
		}
		stubbed[name] = struct{}{}
		builder.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(func(context.Context, api.Module, []uint64) {}), fn.ParamTypes(), fn.ResultTypes()).
			Export(name)
	}
	if len(stubbed) == 0 {
		return nil
	}
	if _, err := builder.Instantiate(ctx); err != nil {
		return fmt.Errorf("instantiating lesstruct host stubs: %w", err)
	}
	return nil
}

// loadCheckWasm instantiates compiled WASM in a throwaway wazero runtime and
// asserts the plugin shape the host loader needs: linear memory plus at
// least one hook_ export. Declared lesstruct imports are satisfied with
// signature-derived no-op stubs (mirroring the host functions the real
// loader instantiates from the manifest), so capability-using plugins
// pass. The memory check runs on the compiled module because an
// instantiated module reports a non-nil memory handle even when the guest
// defines none. WASI is instantiated without preopened dirs and start
// functions never run, so the check loads but never executes guest code.
func loadCheckWasm(ctx context.Context, wasmBytes []byte) error {
	if len(wasmBytes) == 0 {
		return fmt.Errorf("empty wasm module")
	}
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithCloseOnContextDone(true))
	defer func() { _ = runtime.Close(ctx) }()
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return fmt.Errorf("instantiating WASI: %w", err)
	}
	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compiling wasm module: %w", err)
	}
	defer func() { _ = compiled.Close(ctx) }()
	if len(compiled.ExportedMemories()) == 0 && len(compiled.ImportedMemories()) == 0 {
		return fmt.Errorf("wasm module declares no linear memory")
	}
	// Stub the lesstruct host module before instantiating: capability-using
	// plugins import host functions the real loader provides from their
	// manifest. Without the stub the instantiation fails with
	// "module[lesstruct] not instantiated" on every capability plugin.
	if err := stubLesstructHostModule(ctx, runtime, compiled); err != nil {
		return err
	}
	module, err := runtime.InstantiateModule(
		ctx,
		compiled,
		wazero.NewModuleConfig().WithName("plugingen-loadcheck").WithStartFunctions(),
	)
	if err != nil {
		return fmt.Errorf("instantiating wasm module: %w", err)
	}
	defer func() { _ = module.Close(ctx) }()
	hasHook := false
	for name := range module.ExportedFunctionDefinitions() {
		if strings.HasPrefix(name, "hook_") {
			hasHook = true
			break
		}
	}
	if !hasHook {
		return fmt.Errorf("wasm module exports no hook_ function")
	}
	return nil
}
