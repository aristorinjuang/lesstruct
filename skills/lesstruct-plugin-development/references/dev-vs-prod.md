# Development vs Production Mode

Lesstruct's plugin system supports two modes, toggled by the same
`DEV_MODE` env var used by the admin SPA.

## Production mode (default)

When `DEV_MODE` is unset, false, or any value other than the string
`"true"` (the runtime uses `getEnvBool`, which is permissive about the
exact spelling), the plugin system loads all `.wasm` files in
`plugins/` once at startup. To apply changes, restart the server.

This is the recommended mode for any non-development environment.

## Development mode (`DEV_MODE=true`)

When `DEV_MODE=true`, the plugin system starts a filesystem watcher
on the `plugins/` directory. When a `.wasm` file is created, modified,
or removed, the watcher:

1. Debounces filesystem events by 150 ms.
2. Reloads the affected `.wasm` file.
3. Unregisters the previous version's hooks.
4. Re-runs discovery on the new version.
5. Logs the reload.

The watcher is **non-recursive**: it only watches top-level files in
`plugins/`. Subdirectories are ignored. If you organise your plugins
into subdirectories (e.g. `plugins/team-a/foo.wasm`), they will not
be hot-reloaded; you will need to restart Lesstruct.

The watcher is also non-recursive for **deletions**: removing a
`.wasm` file unloads it, but the new state of `plugins/` is not
re-scanned for other changes.

## Shared with admin SPA HMR

`DEV_MODE=true` is the same env var the admin panel uses to enable
hot-module replacement (HMR) on the Vue dev server. Toggling it on for
plugin hot-reload also enables admin-panel HMR; toggling it off for
production plugin loading also disables admin HMR.

If you need plugin hot-reload but production admin (or vice versa),
you will need to either:

- Run a separate Lesstruct instance for dev and prod.
- Patch the runtime to read a separate env var (not currently supported).

## When to use DEV_MODE

- During plugin development: `DEV_MODE=true` to iterate without
  restarting the server.
- In CI / production: `DEV_MODE=false` (or unset) for stable plugin
  loading.

## Caveats

- Hot-reload is not atomic. If the new version of the plugin
  introduces a runtime error (e.g., a panicking hook), the watcher
  logs the error and the host's hook table is updated to skip the
  failed plugin. Subsequent content operations will not invoke the
  failed hook.
- The watcher does not reload manifest changes. If you change the
  `<name>.manifest` file, restart Lesstruct.
- The watcher does not reload changes to other files (e.g., adding a
  brand-new `.wasm` to `plugins/` while the server is running). It
  only watches the top-level entries of `plugins/`.

## Verifying the mode

To confirm which mode Lesstruct is running in, check the startup log:

- Production: `Production mode activated - startup-load enabled` (or
  similar; exact wording may vary).
- Development: `Dev mode activated - hot-reload enabled` (or similar).
