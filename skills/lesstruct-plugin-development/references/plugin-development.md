# Plugin Development Guide

> **Snapshot notice.** This is the **user-facing** snapshot of the Lesstruct
> plugin development guide, bundled with the `lesstruct-plugin-development`
> skill so the skill is self-contained. The canonical, developer-facing
> version lives in the Lesstruct repository at `docs/plugin-development.md`
> and references source-tree paths; this version is rewritten for someone
> with a binary Lesstruct install and a `plugins/` directory.
>
> If you have a newer Lesstruct release, re-export this file from the repo to
> refresh the snapshot.

Lesstruct supports WebAssembly (WASM) plugins that extend functionality through hooks. Plugins are compiled from Go, Rust, C/C++, or any language that targets WASI.

## How Plugins Work

1. Compile your plugin to a `.wasm` file targeting the WASI runtime.
2. Place the `.wasm` file (and an optional `.manifest` file) in the `plugins/` directory.
3. Lesstruct discovers and loads the plugin at startup.
4. The plugin system calls exported hook functions at the appropriate lifecycle points.

> **See also:** [Plugin Capabilities](plugin-capabilities.md) — host functions for HTTP, database, and logging. If your plugin needs to call external APIs or query the database, add a capability manifest.

The `plugins/` directory is a relative path from the Lesstruct working directory. It is not configurable via env or flag.

## Plugin Capabilities (Host Functions)

Plugins that need access to host resources (HTTP, database, logging) declare their requirements in a **capability manifest** — a TOML file placed alongside the `.wasm` file.

### Manifest File

Create `<plugin-name>.manifest` next to `<plugin-name>.wasm`:

```toml
name = "my-enrichment-plugin"
version = "1.0.0"

[capabilities]
http = ["https://api.example.com/*"]
database = ["read:content"]
```

If no `.manifest` file exists, the plugin runs with **zero** host functions — hooks only.

### Available Host Functions

| Function | Import Path | Description |
|----------|-------------|-------------|
| `lesstruct.http_get` | `lesstruct.http_get` | HTTP GET request (URL allowlist checked) |
| `lesstruct.http_post` | `lesstruct.http_post` | HTTP POST request (URL allowlist checked) |
| `lesstruct.db_query` | `lesstruct.db_query` | Execute SELECT query (table access checked) |
| `lesstruct.db_exec` | `lesstruct.db_exec` | Execute INSERT/UPDATE/DELETE (table access checked) |
| `lesstruct.log_info` | `lesstruct.log_info` | Log info message to host |
| `lesstruct.log_error` | `lesstruct.log_error` | Log error message to host |

The full reference lives in [Plugin Capabilities](plugin-capabilities.md).

### Host Function Import (Go/TinyGo)

Unlike hooks (which use `//export`), host functions use `//go:wasmimport`:

```go
//go:wasmimport lesstruct http_get
func httpGet(urlPtr, urlLen uint32, headersPtr, headersLen uint32) uint32

//go:wasmimport lesstruct log_info
func logInfo(msgPtr, msgLen uint32)
```

> **Note.** The Lesstruct SDK currently ships only the constant names for
> these host functions, not the `//go:wasmimport` declarations themselves.
> The full block is bundled with this skill at
> `references/host-function-imports.go.txt` — copy that into your
> plugin's `main.go`.

## Hook System

### Available Hooks

The host invokes three hooks today:

| Hook | WASM Export Name | Description |
|------|------------------|-------------|
| BeforeSaveContent | `hook_before_save` | Called before content is saved (create or update) |
| AfterCreateContent | `hook_after_create` | Called after content is created |
| AfterPublishContent | `hook_after_publish` | Called after content is published |

> **Reserved hooks — do not rely on them.** Two additional hooks
> (`on_plugin_loaded`, `before_delete`) are defined in the host but are
> not currently invoked by any production code path. If your plugin
> exports only one of these, the plugin will load successfully but
> the hook will never fire. Use one of the three invoked hooks
> instead.

### Failure Mode

When a hook returns an error, the request fails. The content service
maps the error to a 500 response and the content is not saved (for
`before_save`) or the error is logged (for `after_create` /
`after_publish`, whose results are not stored). There is no automatic
rollback of prior hooks in a chain.

## System Fields

System fields are special custom field values managed by plugins. They
are defined in the post type TOML schema with the `system = true` flag
and stored alongside regular custom fields in the `customFields` JSON
map.

### Hook Data Format

When `before_save` or `after_create` hooks execute, the host sends a
JSON object with eight fields:

```json
{
  "contentId": 0,
  "userId": 42,
  "title": "My Product",
  "content": "...",
  "tags": ["featured", "sale"],
  "status": "draft",
  "postType": "product",
  "customFields": {
    "price": 29.99,
    "internal_sku": "SKU-001",
    "sync_status": "synced"
  }
}
```

Field notes:

- `contentId` is `0` on create, the existing content's ID on update.
- `userId` is the authenticated user performing the action.
- `status` is one of `draft`, `published`, `archived`, or a custom value.
- `postType` is `post`, `page`, or a custom type.
- `customFields` contains both regular custom fields and system fields.

A complete example is bundled with this skill at
`references/hook-data-example.json`.

### Reading System Fields

In a `before_save` hook, read system field values from `customFields` in the JSON input:

```go
//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
    input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

    var data map[string]any
    json.Unmarshal(input, &data)

    if cf, ok := data["customFields"].(map[string]any); ok {
        if sku, ok := cf["internal_sku"].(string); ok {
            // Read existing system field value: "SKU-001"
            _ = sku
        }
    }

    modified, _ := json.Marshal(data)
    copy(resultBuf[:], modified)
    return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}
```

### Writing System Fields

A plugin can set or modify system field values in `customFields`. The
host validates plugin-set system field values against their schema
definition (type, required, options, min/max). Validation runs in the
content service **after** the hook returns — the hook itself does not
see validation errors.

> If the plugin writes an invalid system field value, the API call
> fails with a 500 and the content is not saved. The hook does not
> receive the validation error; the error is mapped to the API
> caller.

```go
//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
    input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)

    var data map[string]any
    json.Unmarshal(input, &data)

    cf, ok := data["customFields"].(map[string]any)
    if !ok {
        cf = make(map[string]any)
        data["customFields"] = cf
    }

    cf["internal_sku"] = "SKU-002"
    cf["sync_status"] = "pending"

    modified, _ := json.Marshal(data)
    copy(resultBuf[:], modified)
    return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}
```

### What the Host Reads Back from the Result

The host only reads back the `customFields` key from the hook's
result. The plugin may write other keys (`title`, `tags`, etc.) into
its result JSON, but the host ignores them. To mutate the content
item, the plugin must write `customFields` and return a JSON object
containing it.

### Important Notes

- System field values are only preserved when set through `before_save` hooks. User-submitted system field values are stripped for security.
- `after_create` and `after_publish` hooks can read but should not write system fields — their results are not stored (notification-style hooks).
- If no plugin handles a `before_save` hook, system fields are stripped as usual and content creation proceeds normally.

## Memory Protocol

### Hook Functions vs Host Functions

- **Hooks** use `//export` — the plugin exports them, the host calls them. Data flows via `(offset, length) -> resultOffset` at offset `65536` (64KB).
- **Host functions** use `//go:wasmimport` — the host exports them, the plugin calls them. The plugin manages offsets. The host writes results to a fixed offset of `4096`. See [Plugin Capabilities](plugin-capabilities.md).

### Hook Function Signature

Every hook function must follow this signature:

```
(offset: uint32, length: uint32) -> resultOffset: uint32
```

### Data Flow

1. The host writes input data to WASM linear memory at offset `65536` (64KB).
2. The host calls the hook function with `(65536, dataLength)`.
3. The plugin reads input from `(offset, offset+length)`.
4. The plugin writes the result to WASM memory.
5. The plugin returns the offset where the result starts.
6. The host reads the result from the returned offset.

If the plugin returns `0` (or empty bytes), the host treats the
result as "no change" and uses the original input.

### Variable-Length Results

If the result length differs from the input length, export `__hook_result_len`:

```
(inputLen: uint32) -> resultLen: uint32
```

The export is **optional**. If it is missing, the host assumes the
result length equals the input length.

### Data Format

All data passed through hooks is JSON-encoded bytes (UTF-8).

### Required Exports

Every plugin `.wasm` file must export:

- `memory` — Linear memory (auto-exported by most compilers).
- One or more `hook_*` functions from the [Available Hooks](#available-hooks) table.
- Optional: `__hook_result_len` (only when result length differs from input length).

## Development vs Production Mode

- **Production** (default): Plugins are loaded once at startup. To reload, restart the server.
- **Development** (`DEV_MODE=true`): Plugins are hot-reloaded when `.wasm` files change via filesystem watcher.

> **`DEV_MODE` is shared with the admin SPA.** Toggling the same env
> var to enable plugin hot-reload also enables admin-panel HMR. If
> you want production plugin loading but dev admin HMR, set
> `DEV_MODE=false`; if you want plugin hot-reload, the admin panel
> will also be in dev mode.

The watcher is non-recursive: subdirectories of `plugins/` are not
watched. It debounces filesystem events by 150 ms and reloads only
the affected `.wasm` files. On reload, the host unregisters the
plugin's old hooks and re-runs discovery.

For the full set of DEV_MODE caveats, see
[`references/dev-vs-prod.md`](dev-vs-prod.md).

## Go / TinyGo Guide

### Prerequisites

- Go 1.20+
- [TinyGo](https://tinygo.org/getting-started/install/) 0.28+

### Project Setup

```go
// go.mod
module example.com/my-plugin

go 1.21
```

```go
package main

import "unsafe"

var resultBuf [4096]byte

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
    input := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(offset))), length)
    var data map[string]any
    json.Unmarshal(input, &data)
    // ... modify data ...
    modified, _ := json.Marshal(data)
    copy(resultBuf[:], modified)
    return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

func main() {}
```

### Build

```bash
tinygo build -o plugin.wasm -target=wasi main.go
```

### TinyGo Notes

- Use `-target=wasi` (not `wasm32-wasi`).
- Export functions with `//export function_name` directive (no space after `//`).
- `unsafe` package is needed for WASM memory operations.
- No standard library networking or file I/O in WASI sandbox.

### Bundled Examples

This skill ships four Go/TinyGo example plugins under
`references/examples/`:

| Example | Hook | Use as starting point for |
|---------|------|---------------------------|
| `go-hello-world` | `on_plugin_loaded` (reserved) | Syntax reference only — the hook will not fire |
| `go-content-transform` | `before_save` | Simple in-place content modification |
| `go-system-fields` | `before_save` + `after_create` | Reading and writing system fields |
| `go-validation` | `before_save` | Validation that rejects the save |

Copy one of the working examples (any except `go-hello-world`) to
your source tree and modify it.

## Rust Guide

### Prerequisites

- Rust 1.70+
- `wasm32-wasip1` target

### Install Target

```bash
rustup target add wasm32-wasip1
```

### Project Setup

```toml
# Cargo.toml
[package]
name = "lesstruct-plugin"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]
```

```rust
// src/lib.rs
static mut RESULT_BUF: [u8; 4096] = [0; 4096];

#[no_mangle]
pub extern "C" fn hook_before_save(offset: u32, length: u32) -> u32 {
    let memory = unsafe {
        std::slice::from_raw_parts(offset as *const u8, length as usize)
    };
    // Process memory, write result to RESULT_BUF
    unsafe { RESULT_BUF.as_mut_ptr() as u32 }
}
```

### Build

```bash
cargo build --release --target wasm32-wasip1
# Output: target/wasm32-wasip1/release/lesstruct_plugin.wasm
```

> **No official Rust examples ship in the Lesstruct repo today.**
> The memory protocol above applies directly. Validate your `.wasm`
> with `wasm-tools validate plugin.wasm` before deploying.

## C/C++ Guide

### Prerequisites

- Clang or GCC with WASI support
- [WASI SDK](https://github.com/WebAssembly/wasi-sdk) (recommended)

### Project Setup

```c
// plugin.c
#include <stdint.h>
#include <string.h>

static uint8_t result_buf[4096];

__attribute__((export_name("hook_before_save")))
uint32_t hook_before_save(uint32_t offset, uint32_t length) {
    uint8_t *input = (uint8_t *)offset;

    const char *response = "{\"customFields\":{\"price\":0}}";
    memcpy(result_buf, response, strlen(response));

    return (uint32_t)result_buf;
}
```

### Build

```bash
# With WASI SDK
/opt/wasi-sdk/bin/clang --sysroot=/opt/wasi-sdk/share/wasi-sysroot \
    -O2 -o plugin.wasm plugin.c

# With clang and WASI target
clang --target=wasm32-wasi -o plugin.wasm plugin.c
```

> **No official C/C++ examples ship in the Lesstruct repo today.**
> The memory protocol above applies directly. Validate your `.wasm`
> with `wasm-tools validate plugin.wasm` before deploying.

## Testing Plugins

1. Compile the plugin to `.wasm`.
2. Copy the `.wasm` file to the `plugins/` directory.
3. Restart Lesstruct (or use `DEV_MODE=true` for hot-reload).
4. Observe behaviour in the application logs.

For a manual smoke test per hook, see
[`references/plugin-checklist.md`](plugin-checklist.md).

If you observe your hooks firing on create but not on delete, or
`on_plugin_loaded` not firing at all, you have hit a hook that is
defined but not currently invoked. See the
[Available Hooks](#available-hooks) section above.
