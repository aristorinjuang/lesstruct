package main

// A content transformation plugin that uppercases post titles.
//
// This plugin hooks into before_save and transforms the "title" field
// of content data to uppercase.
//
// Build:
//
//	tinygo build -o content-transform.wasm -target=wasi main.go
//
// Install:
//
//	cp content-transform.wasm <lesstruct-project>/plugins/
import (
	"unsafe"
)

// resultBuf holds the transformed output written back to the host.
// Kept well under wasmBaseOffset (65536) to avoid overlapping with host write area.
var resultBuf [4096]byte

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
	input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

	// Copy input, transform title in-place, write to result
	n := copy(resultBuf[:], input)
	transformTitle(resultBuf[:n])

	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//export __hook_result_len
func hookResultLen(inputLen uint32) uint32 {
	return inputLen
}

// transformTitle finds "title":"value" in JSON and uppercases the value.
func transformTitle(data []byte) {
	titleKey := []byte(`"title":"`)
	start := indexOf(data, titleKey)
	if start == -1 {
		return
	}
	start += len(titleKey)

	// Find the closing quote
	end := start
	for end < len(data) && data[end] != '"' {
		end++
	}

	// Uppercase each character in the title value
	for i := start; i < end; i++ {
		if data[i] >= 'a' && data[i] <= 'z' {
			data[i] -= 32
		}
	}
}

func indexOf(haystack, needle []byte) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

// Required for WASI
func main() {}
