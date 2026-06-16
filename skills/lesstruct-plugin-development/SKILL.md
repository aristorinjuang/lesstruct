---
name: lesstruct-plugin-development
description: Develop a WASM plugin for a Lesstruct installation. Plugins extend Lesstruct through three currently-invoked hooks (before_save, after_create, after_publish) and optional host functions (http_get, http_post, db_query, db_exec, log_info, log_error). Walks through the hook contract, the memory protocol, the capability manifest, and the build-and-deploy cycle. Use when the user asks to write a Lesstruct plugin, add a hook, query the database from a plugin, or call an external API from a plugin.
---

# Lesstruct Plugin Development

## Overview

This skill guides a Lesstruct user (and their AI agent) through writing a
WASM plugin for a Lesstruct installation. The skill works from the user's
`plugins/` directory and the running Lesstruct install; it does not require
access to the Lesstruct source tree.

**Output:** none mandatory. The skill typically produces a `<name>.wasm`
file and an optional `<name>.manifest` in the user's `plugins/` directory.

## When to Use

Trigger this skill when the user asks to:

- Write a new WASM plugin for their Lesstruct installation.
- Add a hook (before_save, after_create, after_publish) to a plugin.
- Call an external API from a plugin (http_get, http_post).
- Read or write to the database from a plugin (db_query, db_exec).
- Diagnose why a hook is or is not firing.
- Audit a plugin's manifest for unsupported features.

## Activation

1. Load `references/plugin-development.md` and
   `references/plugin-capabilities.md` (the user-facing snapshots of the
   contract). Treat them as authoritative for the rest of the workflow.
2. Confirm the user's intent: **new plugin**, **add a host-function call**,
   **debug a plugin**, **audit a plugin**, or **prepare for an upgrade**.
3. Greet the user and state the chosen intent in one sentence before
   proceeding.

## Procedure

### Step 1: Locate the plugins directory

The `plugins/` directory is the relative path `plugins/` from the Lesstruct
working directory. It is hard-coded; it is not configurable via env or flag.
Ask the user for the absolute path to their `plugins/`.

If they do not have one, give them the boilerplate and **wait**:

```bash
mkdir -p plugins
```

…then restart Lesstruct (or rely on `DEV_MODE=true` hot-reload).

**Never invent a path.** If the user is unsure, ask them to run
`ls plugins/` (or the equivalent) and paste the result.

### Step 2: Inventory existing plugins

Read the contents of `plugins/`. For each `.wasm` file, check whether a
matching `<name>.manifest` exists. Pair them up. Note any `.wasm` files
that have no manifest (they get zero host functions — hooks only).

### Step 3: Plan the plugin

Ask the user three questions:

1. **What should the plugin do?** (enrich content with an external API,
   validate fields, log activity, etc.)
2. **When should it run?** This determines the hook. The three currently
   invoked hooks are:
   - `before_save` — runs before content is saved (create or update).
     Result's `customFields` is applied to the saved item.
   - `after_create` — runs after content is created. Result is discarded
     (notification-style).
   - `after_publish` — runs after content is published. Result is
     discarded (notification-style).
3. **What host functions does it need?** Skip this if hooks-only.
   - `http_get`, `http_post` for external API calls.
   - `db_query`, `db_exec` for database access.
   - `log_info`, `log_error` for logging (always available if a manifest
     exists; no capability declaration needed).

> **Reserved hooks.** Two hooks (`on_plugin_loaded`, `before_delete`) are
> defined in the host but **not currently invoked**. If the user's plan
> relies on one of these, warn them: "This hook is registered if exported,
> but the host does not invoke it today. It will not run. Pick
> `before_save`, `after_create`, or `after_publish` instead."

### Step 4: Pick a starting point

The skill bundles four Go/TinyGo example plugins under
`references/examples/`. Three of them work today; one uses a reserved hook.

| Example | Hook | Fires today? |
|---------|------|--------------|
| `go-content-transform` | `before_save` | ✓ |
| `go-system-fields` | `before_save` + `after_create` | ✓ |
| `go-validation` | `before_save` | ✓ |
| `go-hello-world` | `on_plugin_loaded` | ✗ (reserved) |

Recommend the user copy one of the three working examples as a starting
point. The `go-system-fields` example is the most complete; the
`go-content-transform` example is the simplest.

For Rust or C/C++ plugins, no official examples ship. Tell the user the
Go memory protocol applies directly and they should validate the
`.wasm` with `wasm-tools validate plugin.wasm` before deploying.

### Step 5: Implement

For a Go/TinyGo plugin:

1. Copy the chosen example to a new directory under `plugins-src/`
   (or wherever the user keeps source code).
2. Edit `main.go`:
   - Update the `//export hook_*` function to do what the user wants.
   - If using host functions, copy the `//go:wasmimport` block from
     `references/host-function-imports.go.txt` into `main.go`.
3. Edit `go.mod` to set the module path.
4. Build:
   ```bash
   tinygo build -o my-plugin.wasm -target=wasi .
   ```

For Rust:

1. `cargo init --lib`.
2. Set `crate-type = ["cdylib"]` in `Cargo.toml`.
3. Implement the hook(s) in `src/lib.rs` using the memory protocol from
   the bundled reference.
4. `cargo build --release --target wasm32-wasip1`.
5. Validate: `wasm-tools validate target/wasm32-wasip1/release/my_plugin.wasm`.

For C/C++:

1. Set up a WASI sysroot (WASI SDK recommended).
2. Implement the hook(s) using `__attribute__((export_name(...)))`.
3. Build with `clang --target=wasm32-wasi -O2 -o my-plugin.wasm plugin.c`.
4. Validate: `wasm-tools validate my-plugin.wasm`.

### Step 6: Write the manifest (if using host functions)

If the plugin uses any host function except `log_info` / `log_error`,
create `<name>.manifest` next to `<name>.wasm` in `plugins/`:

```toml
name = "my-plugin"
version = "0.1.0"

[capabilities]
http = ["https://api.example.com/*"]
database = ["read:content"]
```

The `name` field is required and must be non-empty. The runtime does not
cross-check it against the filename, but matching them is good hygiene.

For URL patterns: trailing `*` is the only wildcard. Patterns are checked
in declaration order; the first match wins.

For database permissions: only `read|write : content|media|users` are
accepted. Eight other tables exist in the database but are inaccessible
to plugins by design.

### Step 7: Deploy

```bash
cp my-plugin.wasm plugins/
cp my-plugin.manifest plugins/   # if applicable
```

Restart Lesstruct (or set `DEV_MODE=true` for hot-reload of `.wasm`
files in `plugins/`). The watcher is non-recursive; subdirectories of
`plugins/` are not watched.

### Step 8: Verify (manual)

Give the user a smoke test per hook:

| Hook | Smoke test |
|------|------------|
| `before_save` | Create or update a content item. Confirm the plugin's `log_info` line appears in the Lesstruct log (if the plugin calls it) and that any `customFields` modification is reflected in the saved item (check via the API or the admin panel). |
| `after_create` | Create a content item. Confirm a `log_info` line appears if the plugin calls it. The plugin's returned data is **not** applied. |
| `after_publish` | Create then publish a content item. Confirm a `log_info` line appears if the plugin calls it. |
| `http_get` / `http_post` | Trigger a save that exercises the plugin. Check the external service's log to confirm the call landed. |
| `db_query` / `db_exec` | Trigger a save that exercises the plugin. Check the host's denied-call log; if the call was permitted, no log line is produced (audit logging is partial — see the user-facing doc). |

If the smoke test fails:

1. Confirm the `.wasm` is in `plugins/` (case-sensitive).
2. Confirm the manifest is in `plugins/` next to the `.wasm` (case-sensitive).
3. Confirm the hook name in the manifest / `//export` matches one of
   `before_save`, `after_create`, `after_publish` (case-sensitive).
4. For host function calls, confirm the manifest declares the matching
   capability (`http` for `http_get` / `http_post`, `database` for
   `db_query` / `db_exec`).
5. Confirm the URL pattern matches the actual URL (trailing `*` only).
6. Confirm the SQL query is a single explicit statement (no CTEs, no
   subqueries, no comments — see the user-facing capabilities doc).
7. If using `http_get` / `http_post` then `db_query` in the same hook
   call, read the first result before invoking the second (the result
   offset `4096` is shared).

### Step 9: Document and hand off

Recommend the user create a `plugins/<name>.README` recording:

- The Lesstruct version the plugin was authored against.
- The hook(s) and host functions it uses.
- The capabilities declared in the manifest.

On future Lesstruct upgrades, the user re-runs this skill with intent
**prepare for upgrade**. The workflow then:

1. Asks the user to compare the bundled `hook-data-example.json` against
   any new shape shipped in the upgrade.
2. Prompts the user to re-read the bundled user-facing docs for any
   newly-invoked hooks (e.g., if `on_plugin_loaded` becomes invoked,
   the user can switch from `before_save` to it).
3. Surfaces any newly-supported capabilities (none today; the
   `references/dev-vs-prod.md` and `plugin-capabilities.md` files in
   the skill are the source of truth for the current set).

### Step 10: Optional — pre-flight audit

If the user asks for an audit of an existing plugin, run the checklist
from `references/plugin-checklist.md` and report each item as pass / fail
/ not applicable.

## Out of scope

- The admin panel, themes, API responses, and email templates. Plugins
  do not affect any of these.
- Internal Lesstruct source changes. The skill is for plugin authors,
  not Lesstruct contributors.

## Reference Index

- `references/plugin-development.md` — user-facing snapshot of the
  plugin contract (hooks, memory protocol, build instructions).
- `references/plugin-capabilities.md` — user-facing snapshot of the
  manifest schema and host functions.
- `references/host-function-imports.go.txt` — copy-pasteable
  `//go:wasmimport` block for Go/TinyGo plugins.
- `references/hook-data-example.json` — the actual JSON shape the
  host sends to a `before_save` hook.
- `references/dev-vs-prod.md` — DEV_MODE caveats and watcher
  behaviour.
- `references/plugin-checklist.md` — pre-flight audit checklist.
- `references/install-paths.md` — per-agent install snippets.
- `references/examples/` — four bundled Go/TinyGo example plugins.
