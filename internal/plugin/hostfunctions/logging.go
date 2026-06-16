package hostfunctions

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

// logInfo is the host function for lesstruct.log_info.
// WASM signature: (message_ptr: i32, message_len: i32) -> void
func logInfo(logger func(string)) api.GoModuleFunc {
	return api.GoModuleFunc(func(_ context.Context, mod api.Module, stack []uint64) {
		msgPtr := api.DecodeU32(stack[0])
		msgLen := api.DecodeU32(stack[1])

		mem := mod.Memory()
		if mem == nil {
			return
		}

		msgBytes, ok := mem.Read(msgPtr, msgLen)
		if !ok {
			return
		}

		logger(fmt.Sprintf("[plugin] %s", string(msgBytes)))
	})
}

// logError is the host function for lesstruct.log_error.
// WASM signature: (message_ptr: i32, message_len: i32) -> void
func logError(logger func(string)) api.GoModuleFunc {
	return api.GoModuleFunc(func(_ context.Context, mod api.Module, stack []uint64) {
		msgPtr := api.DecodeU32(stack[0])
		msgLen := api.DecodeU32(stack[1])

		mem := mod.Memory()
		if mem == nil {
			return
		}

		msgBytes, ok := mem.Read(msgPtr, msgLen)
		if !ok {
			return
		}

		logger(fmt.Sprintf("[plugin-error] %s", string(msgBytes)))
	})
}