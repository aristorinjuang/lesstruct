package main

import (
	"encoding/json"
	"unsafe"
)

var resultBuf [4096]byte

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
	input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

	var data map[string]any
	if err := json.Unmarshal(input, &data); err != nil {
		return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
	}

	cf, _ := data["customFields"].(map[string]any)

	// Read a regular custom field set by the user
	if price, ok := cf["price"].(float64); ok {
		_ = price
	}

	// Read an existing system field value
	if sku, ok := cf["internal_sku"].(string); ok {
		_ = sku
	}

	// Write system field values — these will be validated against the TOML schema
	// and stored in the custom_fields database column
	if cf == nil {
		cf = make(map[string]any)
		data["customFields"] = cf
	}
	cf["internal_sku"] = "SKU-AUTO-001"
	cf["sync_status"] = "synced"

	modified, _ := json.Marshal(data)
	copy(resultBuf[:], modified)
	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//export hook_after_create
func hookAfterCreate(offset uint32, length uint32) uint32 {
	// Notification-style hook — read the created content but do not modify
	// Results from after_create are not stored
	input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

	var data map[string]any
	_ = json.Unmarshal(input, &data)

	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

func main() {}
