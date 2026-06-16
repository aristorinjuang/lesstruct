package main

// A minimal "Hello World" plugin that responds when loaded.
//
// Build:
//
//	tinygo build -o hello-world.wasm -target=wasi main.go
//
// Install:
//
//	cp hello-world.wasm <lesstruct-project>/plugins/
import (
	"unsafe"
)

// resultBuf holds the response written back to the host.
var resultBuf [4096]byte

// resultLen tracks the actual length of the last response written.
var resultLen uint32

//export hook_on_plugin_loaded
func hookOnPluginLoaded(offset uint32, length uint32) uint32 {
	// Build a response acknowledging the plugin was loaded
	msg := `{"status":"loaded","plugin":"hello-world","input_size":` + itoa(length) + `}`
	resultLen = uint32(copy(resultBuf[:], msg))

	// Return the offset where the result data starts
	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//export __hook_result_len
func hookResultLen(inputLen uint32) uint32 {
	return resultLen
}

func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}
	return string(buf[i:])
}

// Required for WASI
func main() {}
