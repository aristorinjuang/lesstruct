# AI Module Generation

> **Scope.** This page is the how-to for generating modules: the admin panel
> flow, the CLI, and the MCP tools. Endpoint semantics live in
> [api-reference.md](api-reference.md); module internals (the kernel
> lifecycle, layers, seams) live in
> [module-development.md](module-development.md).

Describe a module in plain language; Lesstruct builds it, validates it, and
stages it as an inert source project. Generation grounds every module on the
distilled kernel contract, so the result already speaks the host's module
language.

Tier 1 modules link at compile time — there is no runtime install. A staged
project is inert source: copy the tree into a source checkout, wire a custom
edition, and rebuild. The download ships an `INSTALL.md` with the exact
steps.

## Requirements

- An AI key: `AI_MODGEN_API_KEY` (or the shared `AI_TEXT_GENERATION_API_KEY`
  fallback) plus model/base-URL settings — see
  [configuration.md](configuration.md). Without a key the admin sidebar hides
  the Module Generator entry and the `/api/v1/modgen/*` routes stay
  unmounted (`404`); with a key but no generation service the endpoints
  answer `503 generation_unavailable`.
- An **admin** account. Every module endpoint requires admin on
  both the browser panel and the API — a non-admin key or session gets
  `403`.
- `/api/health` reports `"moduleGeneration": true` when the feature is live.

## Generate in the admin panel

Open **Module Generator** (`/admin/module-generator`) and write a brief —
"forum threads with moderation" is plenty. Pick the kernel surfaces the
module should implement (`routes`, `public_routes`, `migrations`,
`posttypes`, `roles`, `themes`, `admin`) — or leave them empty and let the
model infer them from the brief. Generation can take minutes: the request budget
is 5 minutes.

Every result passes the same gate the host trusts: the file contract
(`go.mod` + `module.go` + `README.md` plus the domain/api/store/admin
overlays), Go parse, the `kernel.Module` lifecycle shape (including the
`WithLogger` + `WithDB` adoption options), the layer import policy (no
`lesstruct/internal` anywhere, no transport or SQL inside `domain/`, every
`api/` handler through `kernel/httpx`, no `cgo`, `os/exec`, `unsafe`,
`syscall`, `plugin`, or `net/*` imports anywhere), `go.mod` hygiene (no
`replace`, kernel required at the host version, no unreviewed third-party
dependencies), and migration hygiene (allowlisted statements only —
idempotent creates, no cross-module foreign keys, no data writes) — plus a
generated-content scan (no `javascript:` URLs, no `go:linkname`). The gate
runs again when a staged project is applied, so only validated bytes reach
the checkout. A malformed model response is re-asked once; a failing
project goes back to the model with its findings (at most two repair
attempts); a still-failing project is refused and the panel toasts the
reason, e.g.
`Generation failed: api/ handlers must write through kernel/httpx`.

A generated project auto-stages as **source** under `modules-gen/`. It is
inert until installed — download the `.zip` and follow the `INSTALL.md`
inside:

```bash
unzip forum.zip
cp -r forum <lesstruct-checkout>/modules/
# go.work: add `use ./modules/forum`
# Makefile SUBMODULES: append `modules/forum`
# internal/editions/custom/: append the module to core.Candidates
cd <lesstruct-checkout> && go test ./modules/forum/... && go build .
```

## List, download, apply, delete

`/admin/modules` lists every staged project with its surfaces, file count,
and applied marker. **Download** streams the `.zip` with the `INSTALL.md`
steps. **Delete** removes the project directory. The browser form only: a brief
without surfaces lets the model choose them, and surfaces it adds beyond
the brief surface as warnings in the validation report.

**Apply to checkout** (visible only when `/api/health` reports
`moduleApply: true` — see [configuration.md](configuration.md)) stages the
project into the server's source checkout and verifies it: the tree lands in
`modules/<name>/`, `go.work` and `SUBMODULES` gain the registration, the
machine-owned custom edition is regenerated from its `applied.json`
registry, and module tests plus the custom-binary build run. Applying
executes the generated module's code on the host (its tests run as the
server user with the server's network access) and then links it into the
custom binary: enable applies only on machines you operate, for modules
whose generated source you have read. Verification failures keep the
applied tree and toast the failing steps; refusals (dirty shared files,
already-applied modules) toast the reason. Either way, rebuild
`./cmd/lesstruct-custom` and deploy the binary yourself — the server can
never hot-link new code.

**Remove from checkout** (applied cards only, behind a confirm dialog)
reverses an apply: the registry entry and custom-edition wiring are
regenerated without the module, its `go.work`/`SUBMODULES` registrations
are dropped, the module tree is deleted, and the custom binary rebuilds to
verify. Tracked operator edits inside the tree refuse the removal (`409
tree_dirty` — commit or revert them first); the staged copy is untouched
and re-apply restores the module. Verification failures keep the removal and
report the build output. Deleting a staged copy whose module is still
applied warns that the applied tree survives in the checkout.

**Replace** an applied module by applying with `replace` confirmed: the
owned tree is overwritten with the staged files and the full verification
runs. Hand-placed trees (present without a registry entry) are never
overwritten — `409 module_applied` either way.

A hook-shaped brief with no surfaces ("uppercase titles on save") earns a
tier-guidance warning: a Tier 2 WASM plugin may fit better than a Tier 1
module — see [plugin-generation.md](plugin-generation.md).

## From the terminal

The same surface is available to agents and scripts via `lesstruct-cli
module` (admin API key):

```bash
lesstruct-cli module generate --description "Forum threads" --surfaces routes,migrations
lesstruct-cli module list
lesstruct-cli module download forum --output-dir /path/to/projects
lesstruct-cli module apply forum
lesstruct-cli module unapply forum --confirm
lesstruct-cli module delete forum
```

Generation failures print the first error finding; `--output json` emits the
raw payload including the full validation report and file map. See
[cli.md](cli.md) for flags and exit codes.

## From MCP

Five tools, all admin only: `module_generate` (returns name, rationale,
validation, surfaces, and file paths — contents stay out of the payload so
hosts do not truncate them), `module_projects`, `module_apply` (minutes-long;
needs an apply-enabled server — stages the project and reports
verification), `module_unapply` (requires `confirm: true`), and
`module_delete` (requires `confirm: true`). Gate failures surface the first
error finding as the tool error. Download stays out — downloads are binary.
See [mcp.md](mcp.md) for the tool catalog.

## Troubleshooting

- `503 generation_unavailable` — the surface is mounted but generation has
  no AI key configured; check [configuration.md](configuration.md). (With
  no key at all the routes stay unmounted and answer `404`.)
- `429 generation_busy` — two generations already running server-wide; wait
  and retry.
- `409 module_exists` — a project with the same name is already staged;
  delete it (or pick another brief) to regenerate.
- `500 generation_failed` — the gate still failed after two repairs (a
  malformed model response is re-asked once before that); the response
  `details` (CLI `--output json`, MCP error text) carry the final
  findings.
- `400 invalid_surfaces` — a surface is outside the known vocabulary
  (`routes`, `public_routes`, `migrations`, `posttypes`, `roles`,
  `themes`, `admin`).
- `503 apply_unavailable` — applies are not enabled (`MODULE_APPLY_ENABLED`)
  or the host cannot honor them (no Go toolchain, no source checkout).
- `503 apply_not_checkout` — the server does not run from a source checkout
  (binary deployments can never apply).
- `409 module_applied` — the module is already in the checkout's modules
  tree. Apply with `replace` confirmed to overwrite a registry-owned tree;
  hand-placed trees are never overwritten.
- `404 module_not_applied` — unapply for a module the checkout registry
  does not hold.
- `400 confirmation_required` — unapply (or any destructive call) without
  its explicit confirmation.
- `409 tree_dirty` — shared registry files carry uncommitted operator
  changes (or a machine-owned path holds hand-written files).
- `500 apply_failed` — a write or unexpected failure mid-apply; the tree was
  rolled back and the error text says which step failed. (Verification
  failures are *not* this: they answer `200` with `verified: false` and
  keep the applied tree.)
- `403` — non-admin account or key; module endpoints require admin.

## See also

- [api-reference.md](api-reference.md) — the `/api/v1/modgen` contract.
- [cli.md](cli.md) — the `module` command group.
- [mcp.md](mcp.md) — the `module_*` tools.
- [module-development.md](module-development.md) — writing and wiring
  modules by hand.
