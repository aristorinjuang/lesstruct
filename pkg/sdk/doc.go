// Package sdk provides constants and documentation for developing WASM plugins for Lesstruct.
//
// Plugins are compiled to WebAssembly (WASM) and loaded at runtime by the Lesstruct
// plugin system. Each plugin exports hook functions that the host calls at specific
// lifecycle points.
//
// # Getting Started
//
// 1. Write a plugin in Go (TinyGo), Rust, or C/C++
// 2. Export one or more hook functions named hook_{snake_case_hook_name}
// 3. Compile to WASM targeting the WASI runtime
// 4. Place the .wasm file in the plugins/ directory
//
// # System Fields
//
// System fields are special custom field values managed exclusively by plugins. They
// are defined in post type TOML schemas with `system = true` and stored in the
// `customFields` JSON map alongside regular custom fields.
//
// Plugins write system field values through `before_save` hooks by including them in
// the `customFields` object of the returned JSON data. The host validates plugin-set
// system field values against their schema (type, options, min/max). User-submitted
// system field values are always stripped for security.
//
// For examples of reading and writing system fields in hook handlers, see
// [docs/plugin-development.md] and the go-system-fields example.
//
// See [docs/plugin-development.md] for the full plugin development guide.
package sdk
