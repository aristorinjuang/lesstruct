# Plugin Pre-Flight Checklist

Use this checklist to audit a plugin before declaring it ready. Each
item is a hard pass / fail.

## File layout

- [ ] Plugin file is `<name>.wasm` in `plugins/`.
- [ ] Optional `<name>.manifest` is in the same directory as the `.wasm`.
- [ ] Plugin filename (without extension) and the manifest's `name` field
      match. The host does not enforce this today, but matching them is
      good hygiene.

## Hooks

- [ ] Plugin exports at least one of the three currently-invoked hooks:
      `hook_before_save`, `hook_after_create`, `hook_after_publish`.
- [ ] Plugin does **not** rely on `hook_on_plugin_loaded` or
      `hook_before_delete`. Both are defined in the host but not
      currently invoked. Plugins that only export one of these will
      load but never fire.
- [ ] Each hook's result is a JSON object containing (at most)
      `customFields` for mutation. The host reads only `customFields`
      back; other fields in the result are ignored.
- [ ] `__hook_result_len` is exported only if the result length differs
      from the input length. The export is optional; the host uses
      `inputLen` as the result length when the export is missing.

## Manifest

- [ ] `name` is set and non-empty.
- [ ] `version` is set and non-empty.
- [ ] `capabilities.http` is set to a list of URL patterns if the
      plugin uses `http_get` or `http_post`.
- [ ] `capabilities.database` is set to a list of
      `read|write:content|media|users` permissions if the plugin uses
      `db_query` or `db_exec`.
- [ ] URL patterns use trailing `*` only (no regex, no `?`).
- [ ] URL patterns are ordered so the most specific match comes first
      (the first match wins).

## Host function calls

- [ ] If the plugin uses `http_get` or `http_post`, the manifest's
      `http` patterns cover the actual URLs the plugin will request.
- [ ] If the plugin uses `db_query` or `db_exec`, the manifest's
      `database` permissions cover the actual table names in the
      queries (and the actual operation: `db_query` is always
      `read`, `db_exec` is always `write`).
- [ ] Every SQL query is a single explicit `SELECT` / `INSERT` /
      `UPDATE` / `DELETE` statement. No CTEs, no subqueries, no
      comments, no quoted identifiers, no multi-statement queries.
- [ ] The plugin reads each host function's result from offset `4096`
      before invoking the next host function. All four data host
      functions share the same result offset.

## Build

- [ ] Compiled with `tinygo build -target=wasi` (Go),
      `cargo build --target wasm32-wasip1` (Rust), or
      `clang --target=wasm32-wasi` (C/C++).
- [ ] Compiled `.wasm` is well under 64 MB (typical TinyGo plugin:
      < 1 MB).
- [ ] For Go plugins that use host functions, the
      `//go:wasmimport` block from
      `references/host-function-imports.go.txt` is copied into the
      plugin's `main.go`.

## Deployment

- [ ] Lesstruct was restarted after the last `plugins/` change
      (unless `DEV_MODE=true` is set; in that case, the watcher
      picks up new `.wasm` files automatically).
- [ ] The `.wasm` is the top-level file in `plugins/`. Subdirectories
      are not watched.

## Smoke test

- [ ] Run the smoke test from
      [`SKILL.md` Step 8](../../SKILL.md#step-8-verify-manual) and
      confirm the relevant hook fires and any `customFields`
      modification is reflected in the saved item.
- [ ] For host function calls, check the Lesstruct log for
      "blocked" messages. If the call was permitted, no log line is
      produced (audit logging is partial); use `log_info` from the
      plugin to record what it did.

## Documentation

- [ ] A `plugins/<name>.README` records the Lesstruct version the
      plugin was authored against, the hooks it uses, and the
      capabilities declared in the manifest.
