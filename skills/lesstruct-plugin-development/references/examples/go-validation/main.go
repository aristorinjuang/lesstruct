package main

// A validation plugin that rejects content with empty titles.
//
// This plugin hooks into before_save and validates that the "title" field
// is not empty. If validation fails, it returns an error response.
//
// Build:
//
//	tinygo build -o validation.wasm -target=wasi main.go
//
// Install:
//
//	cp validation.wasm <lesstruct-project>/plugins/
import (
	"unsafe"
)

// resultBuf holds the validation result written back to the host.
var resultBuf [4096]byte

// lastValidationFailed tracks whether the last hook call returned an error response.
var lastValidationFailed bool

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
	// Read input data from WASM memory
	input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

	// Check if title is empty
	if titleIsEmpty(input) {
		lastValidationFailed = true
		errResp := []byte(`{"error":"validation failed","message":"title must not be empty"}`)
		copy(resultBuf[:], errResp)
		return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
	}

	// Validation passed — return original offset unchanged
	lastValidationFailed = false
	return offset
}

//export __hook_result_len
func hookResultLen(inputLen uint32) uint32 {
	if lastValidationFailed {
		return uint32(len(`{"error":"validation failed","message":"title must not be empty"}`))
	}
	return inputLen
}

func titleIsEmpty(data []byte) bool {
	return contains(data, []byte(`"title":""`)) || contains(data, []byte(`"title": ""`))
}

func contains(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

// Required for WASI
func main() {}
