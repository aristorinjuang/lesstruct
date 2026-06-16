package hostfunctions

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const moduleName = "lesstruct"

// Registry holds the host-side resources that host functions can access.
// Each plugin instantiation creates a filtered view based on its manifest.
type Registry struct {
	httpClient *http.Client
	db         DBExecutor
	logger     func(string)
}

// Close cleans up resources held by the registry.
func (r *Registry) Close() error {
	if r.httpClient != nil {
		r.httpClient.CloseIdleConnections()
	}
	return nil
}

// InstantiateModule creates a wazero host module exposing only the host
// functions allowed by the given manifest. If the manifest is nil or
// declares no capabilities, no module is instantiated (returns nil, nil).
func (r *Registry) InstantiateModule(
	ctx context.Context,
	rt wazero.Runtime,
	manifest *capability.Manifest,
) (api.Module, error) {
	if manifest == nil {
		return nil, nil
	}

	b := rt.NewHostModuleBuilder(moduleName)

	if manifest.HasHTTP() {
		b.NewFunctionBuilder().
			WithGoModuleFunction(
				httpGet(manifest, r.httpClient, r.logger),
				[]api.ValueType{
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
				},
				[]api.ValueType{api.ValueTypeI32},
			).
			Export("http_get")

		b.NewFunctionBuilder().
			WithGoModuleFunction(
				httpPost(manifest, r.httpClient, r.logger),
				[]api.ValueType{
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
				},
				[]api.ValueType{api.ValueTypeI32},
			).
			Export("http_post")
	}

	if manifest.HasDatabase() && r.db != nil {
		b.NewFunctionBuilder().
			WithGoModuleFunction(
				dbQuery(manifest, r.db, r.logger),
				[]api.ValueType{
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
				},
				[]api.ValueType{api.ValueTypeI32},
			).
			Export("db_query")

		b.NewFunctionBuilder().
			WithGoModuleFunction(
				dbExec(manifest, r.db, r.logger),
				[]api.ValueType{
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
					api.ValueTypeI32,
				},
				[]api.ValueType{api.ValueTypeI32},
			).
			Export("db_exec")
	}

	// Logging functions are always available when any manifest exists
	b.NewFunctionBuilder().
		WithGoModuleFunction(
			logInfo(r.logger),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{},
		).
		Export("log_info")

	b.NewFunctionBuilder().
		WithGoModuleFunction(
			logError(r.logger),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{},
		).
		Export("log_error")

	mod, err := b.Instantiate(ctx)
	if err != nil {
		return nil, fmt.Errorf("instantiating host functions: %w", err)
	}

	r.logger(fmt.Sprintf(
		"Host functions module %q instantiated (http=%v, db=%v)",
		moduleName,
		manifest.HasHTTP(),
		manifest.HasDatabase(),
	))

	return mod, nil
}

// NewRegistry creates a new host function registry.
func NewRegistry(
	httpClient *http.Client,
	db DBExecutor,
	logger func(string),
) *Registry {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = func(string) {}
	}
	return &Registry{
		httpClient: httpClient,
		db:         db,
		logger:     logger,
	}
}
