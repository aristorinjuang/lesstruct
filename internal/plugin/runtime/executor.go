package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

type ExecutionResult struct {
	Results []uint64
	Err     error
}

var ErrResourceLimitsExceeded = errors.New("plugin exceeded resource limits")

func (r Runtime) ExecuteFunc(
	ctx context.Context,
	module api.Module,
	funcName string,
	args ...uint64,
) ExecutionResult {
	fn := module.ExportedFunction(funcName)
	if fn == nil {
		return ExecutionResult{
			Err: fmt.Errorf("function %q not found in module", funcName),
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.MaxExecutionTime)
	defer cancel()

	results, err := fn.Call(timeoutCtx, args...)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			r.config.Logger("Plugin exceeded resource limits")
			return ExecutionResult{Err: ErrResourceLimitsExceeded}
		}
		return ExecutionResult{Err: err}
	}

	return ExecutionResult{Results: results}
}
