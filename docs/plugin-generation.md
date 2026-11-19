# AI Plugin Generation

> **Scope.** This page is the how-to for generating plugins: the admin panel
> flow, the CLI, and the MCP tools. Endpoint semantics live in
> [api-reference.md](api-reference.md); plugin internals (hooks, memory
> protocol, host functions) live in
> [plugin-development.md](plugin-development.md) and
> [plugin-capabilities.md](plugin-capabilities.md).

Describe a plugin in plain language; Lesstruct builds it, validates it, and
installs it as a source project. Generation grounds every plugin on the
distilled SDK contract, so the result already speaks the host's hook
language.

## Requirements

- An AI key: `AI_PLUGGEN_API_KEY` (or the shared `AI_TEXT_GENERATION_API_KEY`
  fallback) plus model/base-URL settings — see
  [configuration.md](configuration.md). Without a key the generator answers
  `503 generation_unavailable`.
- An **admin** account. Every plugin endpoint requires admin on
  both the browser panel and the API — a non-admin key or session gets
  `403`.
- `/api/health` reports `"pluginGeneration": true` when the feature is live,
  plus `"pluginBuild": true` when server-side builds are enabled (set
  `PLUGIN_BUILD_ENABLED=true` and install TinyGo on the host — see
  [configuration.md](configuration.md)).

## Generate in the admin panel

Open **Plugin Generator** (`/admin/plugin-generator`) and write a brief —
"uppercase post titles on save" is plenty. Pick the hooks to implement
(`before_save`, `after_create`, …) and the host functions the plugin needs
(`http_get`, `log_info`, …). Generation can take minutes: the request budget
is 130 seconds.

Every result passes the same gate the host trusts: file contract and Go
parse always, plus hook-export and host-function policy, plus a
generated-content scan (no `javascript:` URLs, no `go:linkname`). Host calls
must be covered by grants: HTTP calls need URL grants, database calls need
table grants — uncovered calls fail validation. `go.mod` must stay
dependency-free (builds run offline). A malformed model response is re-asked
once; a failing project goes back to the model with its findings (at most
two repair attempts); a still-failing project is refused and the panel toasts
the reason, e.g.
`Generation failed: main.go must export at least one hook function`.

A generated project auto-installs as **source** under `plugins/src/`. It is
inert until installed — download the `.zip` and compile it yourself (the
archive ships the synthesized capability manifest alongside the sources):

```bash
cd my-plugin && tinygo build -o my-plugin.wasm -target=wasi main.go
cp my-plugin.wasm my-plugin.manifest <lesstruct-project>/plugins/
```

The `.manifest` sidecar is load-bearing: it declares the capability
grants, and without it the loader provides no host functions. Copy both
files, then restart the server (`DEV_MODE` hot-reloads instead).

Or, on a build-enabled server, press **Build & install** on the project
card: the server compiles with TinyGo, load-checks the built `.wasm`, and
installs it plus the synthesized capability manifest into `plugins/`.
`DEV_MODE` hot-reloads the new plugin; otherwise restart the server to
apply it.

## List, download, build, delete, status

`/admin/plugins` lists every generated project; cards for installed plugins
carry an **installed** marker. **Build & install** compiles a project on the
server when builds are enabled (hidden otherwise). **Delete** removes the
project directory (no active-state guard — projects are inert source until
compiled). **Status** lists the loaded `.wasm` plugins with their hook
registrations — the read-only view of what the host actually runs.

## From the terminal

The same surface is available to agents and scripts via `lesstruct-cli
plugin` (admin API key):

```bash
lesstruct-cli plugin generate --description "Uppercase titles" --hooks before_save --capabilities log_info
lesstruct-cli plugin list
lesstruct-cli plugin download my-plugin --output-dir /path/to/projects
lesstruct-cli plugin build my-plugin
lesstruct-cli plugin status
lesstruct-cli plugin delete my-plugin
```

Generation failures print the first error finding; `--output json` emits the
raw payload including the full validation report and file map. See
[cli.md](cli.md) for flags and exit codes.

## From MCP

Five tools, all admin only: `plugin_generate` (returns name, rationale,
validation, hooks, capabilities, and file paths — contents stay out of the
payload so hosts do not truncate them), `plugin_build` (needs a
build-enabled server), `plugin_projects`,
`plugin_status`, and `plugin_delete` (requires `confirm: true`). Gate
failures surface the first error finding as the tool error. Download stays
out — downloads are binary. See [mcp.md](mcp.md) for the tool catalog.

## Troubleshooting

- `503 generation_unavailable` — no AI key configured; check
  [configuration.md](configuration.md).
- `503 build_unavailable` — server-side builds are not enabled: set
  `PLUGIN_BUILD_ENABLED=true` and install TinyGo on the host.
- `429 generation_busy` — two generations already running server-wide; wait
  and retry.
- `429 build_busy` — a build is already running server-wide; wait and retry.
- `409 plugin_exists` — the plugin is already installed; delete the `.wasm`
  from `plugins/` (or run the build again after deleting it) to replace it.
- `500 generation_failed` — the gate still failed after two repairs (a
  malformed model response is re-asked once before that); the response
  `details` (CLI `--output json`, MCP error text) carry the final
  findings.
- `500 build_failed: module[lesstruct] not instantiated` at startup-load —
  a manually copied `.wasm` whose `.manifest` sidecar was left behind (or
  predates the sidecar shipping in downloads): copy the
  `<name>.manifest` next to the `.wasm` and restart.
- `403` — non-admin account or key; plugin endpoints require admin.
- `400 invalid_hooks` / `400 invalid_capabilities` — the hook suffix or
  host function is outside the known vocabulary; see
  [plugin-development.md](plugin-development.md) and
  [plugin-capabilities.md](plugin-capabilities.md).

## See also

- [api-reference.md](api-reference.md) — the `/api/v1/plugingen` contract.
- [cli.md](cli.md) — the `plugin` command group.
- [mcp.md](mcp.md) — the `plugin_*` tools.
- [plugin-development.md](plugin-development.md) — writing and building
  plugins by hand.
- [plugin-capabilities.md](plugin-capabilities.md) — the capability
  manifest reference.
