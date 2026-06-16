# Plugin Capabilities

> **Snapshot notice.** This is the **user-facing** snapshot of the Lesstruct
> plugin capabilities reference, bundled with the `lesstruct-plugin-development`
> skill so the skill is self-contained. The canonical, developer-facing
> version lives in the Lesstruct repository at `docs/plugin-capabilities.md`
> and references source-tree paths; this version is rewritten for someone
> with a binary Lesstruct install and a `plugins/` directory.
>
> If you have a newer Lesstruct release, re-export this file from the repo to
> refresh the snapshot.

Plugins can request access to host resources (HTTP, database, logging) by declaring them in a **capability manifest** file placed alongside the `.wasm` file.

## Quick Start

Create `<plugin-name>.manifest` next to `<plugin-name>.wasm` in the `plugins/` directory:

```toml
name = "my-enrichment-plugin"
version = "1.0.0"

[capabilities]
http = ["https://api.example.com/*"]
database = ["read:content", "write:content"]
```

If no `.manifest` file exists, the plugin gets **zero** host functions — same as before.

## Manifest Reference

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Plugin name (must be non-empty) |
| `version` | Yes | Semantic version of the plugin (must be non-empty) |
| `capabilities.http` | No | List of allowed URL patterns |
| `capabilities.database` | No | List of allowed database permissions |

### URL Patterns

URLs are matched using simple prefix matching. Use `*` as a suffix for wildcards:

```toml
http = [
    "https://api.example.com/*",         # Match all paths on api.example.com
    "https://jsonplaceholder.typicode.com/todos/1",  # Exact match only
]
```

- `https://api.example.com/*` matches `https://api.example.com/v1/data`, `https://api.example.com/foo/bar`, etc.
- Patterns are checked in order; the first match wins.
- The trailing `*` is the only wildcard supported. Regex patterns are not accepted.

### Database Permissions

Database permissions follow the format `<operation>:<table>`:

| Permission | Allows |
|-----------|--------|
| `read:content` | SELECT queries on the `content_items` table |
| `read:media` | SELECT queries on the `media_files` table |
| `read:users` | SELECT queries on the `users` table |
| `write:content` | INSERT/UPDATE/DELETE on the `content_items` table (including the `custom_fields` column) |
| `write:media` | INSERT/UPDATE/DELETE on the `media_files` table |
| `write:users` | INSERT/UPDATE/DELETE on the `users` table |

The host validates each SQL query to extract the target table name and checks it against the manifest.

#### Table Access Scope

Of the 11 tables in the Lesstruct database, only **three** are grantable
to plugins: `content_items` (normalised to `content`), `media_files`
(normalised to `media`), and `users`. The other eight tables —
`comments`, `blocked_emails`, `failed_login_attempts`,
`verification_tokens`, `password_reset_tokens`, `email_update_tokens`,
`soft_deleted_content`, and `api_keys` — are inaccessible to plugins by
design. A plugin cannot query or modify them via `db_query` or
`db_exec`.

## Available Host Functions

### HTTP

| Function | Signature | Description |
|----------|-----------|-------------|
| `lesstruct.http_get` | `(url_ptr, url_len, headers_json_ptr, headers_json_len) -> result_offset` | Perform an HTTP GET request |
| `lesstruct.http_post` | `(url_ptr, url_len, headers_json_ptr, headers_json_len, body_ptr, body_len) -> result_offset` | Perform an HTTP POST request |

### Database

| Function | Signature | Description |
|----------|-----------|-------------|
| `lesstruct.db_query` | `(sql_ptr, sql_len, params_json_ptr, params_json_len) -> result_offset` | Execute a SELECT query |
| `lesstruct.db_exec` | `(sql_ptr, sql_len, params_json_ptr, params_json_len) -> result_offset` | Execute an INSERT/UPDATE/DELETE |

### Logging

| Function | Signature | Description |
|----------|-----------|-------------|
| `lesstruct.log_info` | `(message_ptr, message_len) -> void` | Log an informational message to the host |
| `lesstruct.log_error` | `(message_ptr, message_len) -> void` | Log an error message to the host |

Logging functions are always available when any manifest exists — no capability declaration needed.

## Result Format

All host functions return results as JSON at offset `4096` in WASM memory.

### Success

```json
{
    "status": 200,
    "body": "...",
    "headers": {"Content-Type": "application/json"}
}
```

### Error

```json
{
    "error": "url_not_allowed",
    "message": "URL \"https://evil.com\" not in capability manifest http allowlist"
}
```

> **Offset collision.** All four data host functions (`http_get`,
> `http_post`, `db_query`, `db_exec`) write their result to the same
> fixed offset (`4096`). If your plugin calls `http_get` and then
> `db_query` before reading the first result, the second overwrites
> the first. Read each result before invoking the next host function.

## Security Model

| Concern | Mechanism |
|---------|-----------|
| URL allowlisting | Each HTTP call is checked against the manifest's `http` patterns before the request is made |
| Table-level DB access | SQL queries are parsed to extract the target table, then checked against `database` permissions |
| Response size limit | HTTP response bodies are capped at 1MB |
| Request timeout | HTTP requests have a 10-second timeout (set once at startup); DB queries use the parent request context |
| Memory isolation | Plugins remain in the WASM sandbox; host functions only write results to controlled offsets |
| Audit logging | Only **denied** host function calls are logged at the host level. Successful calls produce no log line. Use `log_info` from your plugin to record what you did. |

### SQL Parser Limitations

The host uses a hand-rolled substring parser to extract the target
table from each query. The parser looks for the first occurrence of
`FROM`, `INSERT INTO`, `UPDATE`, or `DELETE FROM` and reads the first
identifier after it.

The parser does **not** handle:

- Common Table Expressions (`WITH ... SELECT`).
- Subqueries.
- SQL comments (`--` or `/* */`).
- Quoted identifiers.
- Multi-statement queries.

Write a single explicit `SELECT`/`INSERT`/`UPDATE`/`DELETE`
statement per host function call.

## Example: HTTP Enrichment Plugin

A plugin that calls `lesstruct.http_get` to enrich a content item
during `before_save`:

```go
//go:wasmimport lesstruct http_get
func httpGet(urlPtr, urlLen, headersPtr, headersLen uint32) uint32

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
    url := "https://api.example.com/items/lookup"
    // Write url to WASM memory, call httpGet, read result from offset 4096...
    // Parse JSON, extract a value, set it in customFields, write back to resultBuf.
    return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}
```

Manifest:

```toml
name = "http-enrich"
version = "1.0.0"

[capabilities]
http = ["https://api.example.com/*"]
```

> **Note.** The Lesstruct SDK does not currently ship a
> `//go:wasmimport` declaration for `http_get` (or any other host
> function). The declarations are bundled with this skill at
> `references/host-function-imports.go.txt` — copy that block into
> your `main.go` to use the host functions.

## Example: Database Plugin

A plugin that reads content metadata during a `before_save` hook:

```go
//go:wasmimport lesstruct db_query
func dbQuery(
    sqlPtr, sqlLen uint32,
    paramsPtr, paramsLen uint32,
) uint32

//export hook_before_save
func hookBeforeSave(offset uint32, length uint32) uint32 {
    sql := "SELECT COUNT(*) FROM content_items WHERE post_type = ?"
    params := `["product"]`

    // Write SQL and params to WASM memory, call dbQuery, read result from offset 4096...
    resultOffset := dbQuery(sqlPtr, sqlLen, paramsPtr, paramsLen)
    // Parse JSON result from resultOffset...
}
```

Manifest:

```toml
name = "db-plugin"
version = "1.0.0"

[capabilities]
database = ["read:content"]
```
