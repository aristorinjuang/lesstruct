---
name: lesstruct-module-development
description: Develop a Tier 1 Go module for a Lesstruct checkout. First-party Go features compiled into the binary (modules/<name>/, each its own Go module) against the kernel contract. Walks through intent triage, checkout acquire/verify, design checkpoint, tiered scaffold, inward-out build, custom-edition wiring, gates, verify, and audit/upgrade paths. Use when the user asks to write a Lesstruct Go module, add a Tier 1 feature, wire a module into an edition, add module routes/migrations/admin UI, or audit/upgrade a module.
---

# Lesstruct Module Development

## Overview

This skill guides a developer (and their AI agent) through building a
**Tier 1 Go module** for Lesstruct: a first-party feature compiled into
the binary (`modules/<name>/`, each its own Go module) against the
`kernel/` contract.

The skill works from the user's **cloned Lesstruct checkout** (the
source tree). It does not work from an installed binary's `themes/` or
`plugins/` directory — those belong to the theme and plugin skills.

**Output:** a scaffolded module under `modules/<name>/` wired through a
custom edition in `internal/editions/<custom>/` (plus a `cmd/` entry),
with gates green and tests passing both in-workspace and with
`GOWORK=off`.

Authoritative sources, in order: the checkout's `AGENTS.md`,
`docs/module-development.md`, and `kernel/module.go`. The bundled
`references/` are snapshots for offline work — when they disagree with
the checkout, the checkout wins.

## When to Use

Trigger this skill when the user asks to:

- Write a new Tier 1 Go module (a feature compiled into the binary).
- Add routes, migrations, admin UI, post types, capabilities, or themes
  contributed by a Go module.
- Wire an existing module into an edition or a custom binary.
- Audit a module (ship readiness, contract misuse, disabled-cascade risk).
- Upgrade a module across a Lesstruct/kernel version bump.

Do **not** use this skill when the user wants:

- A WASM runtime extension → the `lesstruct-plugin-development` skill.
- A theme override (`THEME_DIR`, CSS/templates/JS) → the
  `lesstruct-theme-development` skill.
- A kernel contract change or a new provider interface → stop and decide
  (kernel evolution is review-gated; see
  `references/upgrade-guide.md`).
- A custom admin SPA page → out of scope (one shell renders everything;
  modules contribute `AdminExtensionManifest` entries only).

## Activation

1. Load `references/module-development.md` (the adapted snapshot of the
   canonical doc) and `references/kernel-contracts.md` (the capability
   reality matrix). Treat them as working references; the checkout's
   `docs/module-development.md` and `kernel/module.go` stay
   authoritative.
2. Confirm the user's intent: **new private module**, **extend existing
   module**, **custom-edition wiring**, **audit**, or **upgrade**. Triage
   plugin/theme/kernel asks away before proceeding (Step 1 below).
3. Greet the user and state the chosen intent in one sentence before
   proceeding.
4. If the host offers a Plan Mode (or plan/act phases), run Steps 1–3
   there and stop for plan approval before any file is created or
   edited — the design checkpoint (Step 3) is the plan artifact. The
   plan lives in the session or workflow state (see
   `references/module-plan.md`); nothing extra is committed to the
   checkout.
5. Model routing is by role, switched manually at phase boundaries.
   Suggested lanes: Plan and Diff Review on the frontier model (e.g.
   Muse Wonder), Build on the cost-effective model (e.g. Muse Spark).
   Emit a switch prompt at each boundary (`Plan approved — switch to
   <build model> for implementation, or continue here`); never assume
   the host can switch models itself. On hosts without an explicit Plan
   Mode, run Steps 1–3 as the plan, present it, and wait for the user's
   explicit approval before touching files — the plan artifact and the
   stop-for-approval rule work the same with or without the mode.

## Procedure

This skill ships no executable `workflows/` scripts: its phase gates
need human approval (plan sign-off, escalation decisions), which agent
orchestration loops cannot grant. The manual procedure below is the
portable path on every host.

### Step 1: Triage the intent

Ask what the user wants the feature to contribute (routes, storage,
admin UI, post types, capabilities, migrations, themes). Route:

| Goal | Route |
|---|---|
| Runtime extension without recompiling (hook on save/publish, HTTP/DB host call) | WASM plugin — hand off to the `lesstruct-plugin-development` skill. |
| Re-skin or restructure the public site (CSS, templates, JS, second theme) | Theme skill — hand off to the `lesstruct-theme-development` skill. A second `ThemeProvider` registered as a "theme replacement" does **not** work: the first registered provider wins (`internal/app/app.go` ThemeInstaller loop). The only supported second-theme shape is `EditionWithTheme` (blog/commerce pattern). |
| New provider interface or wider kernel interface | Stop-and-decide. Adding a method to an exported interface is a **breaking change**; new contracts need a real consumer, narrow surface, semver class, conformance fixture, and same-change docs (`docs/kernel.md`). This skill does not evolve the kernel. |
| New admin SPA page or mini-SPA | Out of scope. Modules return `AdminExtensionManifest` data (menu, settings, editor extensions); the single shell renders it. |
| Tier 1 Go feature (compiled in, full reach) | Continue with this skill. |

Record the intent and the contribution surface before touching the tree.

### Step 2: Acquire and verify the checkout

Modules build against source, so the first job is a trustworthy
checkout. **Never invent a path.**

1. Ask the user for the absolute path to their Lesstruct clone
   (or confirm the current working directory is one).
2. Verify the markers: repo root has `go.work`, `kernel/go.mod`, and
   `docs/module-development.md`. If any is missing, stop — this is not
   a Lesstruct checkout.
3. Check the tree is clean: `git status --porcelain` must be empty.
   **Never scaffold or edit over a dirty tree** — ask the user to
   commit or stash first, then re-verify.
4. Record the branch and the baseline: `git rev-parse --abbrev-ref HEAD`
   and `git describe --tags --match "v*" --always --dirty`. Private
   modules build on a stable tag (or a known commit); note it.
5. Derive the versions from the checkout — **never hardcode**:
   Lesstruct from `git describe --tags`, kernel from the sibling
   require (read the `github.com/aristorinjuang/lesstruct/kernel`
   require line in any sibling `modules/<name>/go.mod`, e.g.
   `awk '/lesstruct\/kernel / {print $2}' modules/cart/go.mod` —
   workspace members report no version via `go list -m`, so read the
   pinned require instead) The scaffold's `__KERNEL_VERSION__` token
   resolves from this value. If the derived kernel version is empty,
   stop — never scaffold with an empty require.
6. Run the baseline gate once before coding so later failures are
   attributable: `go test ./internal/app/...` (or the touched package
   scope the change justifies). Do not proceed on a red baseline —
   report it.

See `references/troubleshooting.md` (dirty tree, version drift) when
any check fails.

### Step 3: Design checkpoint (before scaffolding)

Agree on the module portfolio first, then four decisions, and write
them down. Do not scaffold until all are answered. The plan template
is `references/module-plan.md`.

**Module portfolio.** For complex features, decide per capability
whether to reuse, extend, or create — one table row each:

| Capability | Reuse / extend / new | Data owner | Contracts | Requires | Routes & auth | Migrations | Edition |
|---|---|---|---|---|---|---|---|
| e.g. guest wishlist | New `wishlist` module | Wishlist items | Public routes, Migrator | Optional: `users` | Public + protected | Yes | New `internal/editions/shop` |
| e.g. product lookup | Reuse `catalog` | Catalog | Edition seam only | Requires: `catalog` | None | No | Same `shop` edition |

Rules: extend an existing module when the capability belongs to the
same bounded context and data owner; create a module when it needs an
independent lifecycle, disable cascade, or data ownership. Never create
a module per endpoint or per table. Implementation follows the
dependency graph. The `Requires` column names hard deps (`Requires()`)
or soft deps (`Optional()`) explicitly.

1. **Module identity.** Registry key (`Name()`, e.g. `mymod`),
   `Requires()` (hard deps — this designs the operator's disable
   cascade), `Optional()` (soft deps). Keep `Requires()` minimal:
   every entry is a transitive-disable edge (`kernel/prune.go`).
   Cross-module foreign keys are forbidden — index the reference
   column instead. `Version` starts at `0.1.0` per the
   cart/media/posttypes-basic exemplars; `Optional()` lists soft deps
   that degrade when absent.
2. **Contract surface.** Which kernel contracts the module implements
   (see `references/kernel-contracts.md` for the reality matrix):
   `RouteProvider` vs `PublicRouteProvider` vs `DualAuthProvider`,
   `Migrator`, `ContentTypeProvider`, `RoleProvider` (+ resolver),
   `ThemeProvider`/`ThemePageProvider`, `AdminExtensionProvider`.
   Note the caveats: `RoleProvider` alone never authorizes (handlers
   enforce through the `CapabilityResolver` seam); `Init` tables do
   not exist yet (migrations run **after** `InitAll`); `DualAuthProvider`
   paths must be full `/api/v1/...` paths that collide with nothing.
   Migrations are tracked in `schema_migrations_<module>`.
3. **Platform vs edition.** All cross-context adaptation lives in
   `internal/editions/<name>/` — the only layer that may import both
   sides. The module itself imports stdlib, `kernel/` published
   language, and (for a root-module context) sibling supporting
   domains — never `lesstruct/internal`, HTTP, SQL, or remote clients
   outside its own `api/`/`store/`/`infrastructure/`.
4. **Tier + edition shape.** Default is a **private custom edition**:
   `internal/editions/<custom>/` plus a thin `cmd/<custom>/main.go`
   calling `app.Run(<custom>.Edition())`. Never wire a private module
   into `internal/app` by default. First-party upstreaming (editing
   `internal/app`, `SUBMODULES`, the release EDGE map,
   `docs/built-in-modules.md`) is a separate, maintainer-gated step.
   The module `README.md` documents the `[modules] disabled` cascade
   behavior (what drops vs what keeps serving).

### Step 4: Scaffold (tiered)

Templates live in `references/scaffold/`; resolution rules in
`references/scaffold/INSTRUCTIONS.txt`. Tiers:

- **core (always):** `go.mod` (requires kernel + siblings at the
  checkout's release tag, **no `replace` directives** — a filesystem
  `replace` in a published `go.mod` breaks `go install pkg@version`;
  local checkouts resolve through the repo-root `go.work`), `module.go`
  (options-built, handlers built once in `New`, typed accessors),
  `embed.go` (migration FS, `//go:embed` inside the `var` block),
  `README.md` (what the module owns and its seams).
- **domain overlay:** `domain/` — entities, sentinels (`types.go`),
  repository port (`repository.go`), service over narrow seams
  (`service.go`, 100% statement bar), table-driven tests.
- **api overlay:** `api/` — handlers + local `seams.go` (+ `agent/`
  for the Bearer realm), writes through
  `kernel/httpx`, principal from context (fail closed on
  `kernel.ErrInvalidPrincipalID`), errors mapped with `errors.Is`,
  public-router bodies capped with `http.MaxBytesReader`;
  `api/handler_test.go` covers the mapping.
- **store overlay:** `store/` — one `*sql.DB + kernel.Dialect`
  implementation covering all compiled-in drivers (placeholder
  rendering, `SupportsReturning` branches, duplicate mapping);
  idempotent `CREATE TABLE IF NOT EXISTS` migrations per driver under
  `migrations/{sqlite,postgresql,mysql}/`, tracked in
  `schema_migrations_<module>`.
- **admin overlay:** `admin/` — `Manifest()` data plus
  `AdminExtensionManifest.Validate()` (never a per-module copy of the
  check); conformance fixture adopted in the module test
  (`admin/manifest_test.go`).
- **testutil overlay:** `testutil/` — `NewModule()`/`Kernel()` fixture
  for module tests, mirroring the media module.

Resolve tokens (`__MODULE_NAME__`, `__KERNEL_VERSION__`) per
`INSTRUCTIONS.txt`, copy the tiers the design needs, and keep the file
layout contract (`AGENTS.md`: const → var → private helpers → public
declarations; methods with their struct; constructor last).

### Step 5: Build inward-out

1. **Domain first.** Types, sentinels, ports, service. No HTTP, SQL, or
   provider clients in `domain/`.
2. **Store next** (if any). Single-source against the dialect; no
   driver types leak upward. Migrations idempotent for pre-module
   installs.
3. **API next.** Narrow seams declared where handlers live; the core
   repository or service satisfies them structurally. Sentinels
   translate at the seam (mirror the message, pin byte-equality with a
   test). Webhook signatures compare with
   `crypto/subtle.ConstantTimeCompare`, never `==`.
4. **`module.go`, `admin/`, `testutil/`.** Options adopt every
   collaborator; `New` builds handlers once; `Requires()` is the
   intended cascade; `Init` validates presence only (no tables yet);
   `Manifest()` validates centrally.
5. Keep `README.md` current: what the module owns and its seams.

### Step 6: Wire the custom edition (default)

Private modules never land in `internal/app` by default. The recipe
(full steps in `references/integration-recipes.md`):

1. Create `internal/editions/<custom>/` (one-seam-per-file adapters
   named for the module they feed, `wire<Module>` steps beside their
   siblings, never mixed into adapter files).
2. Implement `Wire` (construct services/stores, `New` the module with
   options, append to `core.Candidates`, fill optional handler
   surfaces) and `Mount` (post-`InitAll` handler assembly) per the
   blog/commerce exemplars (`EditionWithTheme` for theme swaps).
3. Add a thin `cmd/<custom>/main.go` calling
   `app.Run(<custom>.Edition())` (stays thin like `main.go` and
   `cmd/lesstruct-core/main.go`).
4. Register the workspace: add the module to `go.work` (`use`) and to
   `SUBMODULES` in the root `Makefile` so gates cover it. Private
   modules stay out of the release EDGE map and
   `docs/built-in-modules.md`.
5. Confirm the disabled cascade: `[modules] disabled` prunes the
   module plus transitive dependents with warnings; a kept module
   whose hard dep is absent still fails fast at `InitAll`.

### Step 7: Gates

Run, in order, and fix before proceeding:

1. `go test ./...` inside the module (domain at 100% statement bar;
   empty bodies do not count; unreachable branches are documented
   per-branch exceptions).
2. `make depguard` — nested modules (tests included) must not import
   `lesstruct/internal`.
3. `make lint` (`modernize` + `stylecheck` + `golangci-lint`) and
   `make vulncheck`.
4. Conformance fixtures the module's contracts touch:
   `conformance.CheckAuthorizer` for resolvers,
   `conformance.CheckAdminExtension` (or `ValidateAdminExtension`
   outside testify) for manifest providers.
5. Envelope parity: every handler writes through `kernel/httpx` (the
   compat suite diffs both writers — drift fails a test before a
   client sees it).

### Escalation (during Build)

Escalation is a conditional loop inside Steps 4–7, not a separate
final phase. The build model stops coding, preserves the current state,
and hands control to the frontier model or the human when the work
leaves the approved plan's safe lane:

- A kernel contract is missing or ambiguous; a kernel change may be needed.
- Auth, capability, or principal semantics are unclear.
- A migration is destructive, needs a backfill, or diverges across drivers.
- Route collision, dual-realm mounting, or body-limit complexity.
- Concurrency or background-job lifecycle.
- Tests fail twice without a clear cause.
- The diff starts diverging from the approved plan.

Escalation output is short:

```text
Problem:
Evidence (failing command, log, or code path):
Options:
Recommendation:
Plan change required: yes/no
```

Before escalating, preserve the state: `git status --porcelain` and
`git diff` (or stash) so the resumed session starts from the exact
tree. The twice-failed-tests trigger points at
`references/troubleshooting.md` first; only genuinely new ambiguity
goes up. The human (or the frontier model, with the human informed)
owns the `Plan change required` decision.

If the plan materially changes, return to Plan Mode for approval
before implementation resumes.

### Step 8: Verify (acceptance probe)

1. In-workspace: `go test ./...` from the repo root scope (or the
   touched module scope) — green.
2. Out-of-workspace: `GOWORK=off go test ./...` (or `go build ./...`)
   inside the module — proves the `go.mod` requires resolve without
   the workspace (catches missing requires and stray `replace`s).
3. Serve check (when routes were added): build the custom binary and
   hit the new surface (`curl` or the agent API path); confirm
   disabled-cascade behavior per surface (`[modules] disabled = ["<name>"]`):
   provider-discovered routes unmount when dropped, while explicit
   `HandlerSet` surfaces may keep serving (lifecycle-level disable is
   not an HTTP kill-switch — only `comments` unmounts routes).
4. File sweep: every file `SKILL.md` and the loaded references point
   at exists (the skill is markdown — this sweep is the real test).

### Step 9: Diff Review

Diff Review answers "is this the right change, built the right way?"
— the gates already answered "did tests, lint, coverage, and builds
pass?" Run it after Step 8, ideally in a fresh session (or subagent)
on the frontier model, never on the model that wrote the code. Check
the diff against the approved plan:

- `Requires()` cascade and provider mounting match the design.
- Auth, capability checks, principal parsing, envelope usage.
- Migrations on all drivers; no cross-module foreign keys.
- Domain coverage and meaningful handler/store tests.
- `GOWORK=off`, depguard, no `replace` directives.
- No accidental `internal/app` or unrelated edits.
- Module README and required doc sync.

Report findings with file/line references. Trivial findings get
patched; design flaws escalate back to planning. Carry into the review
session: the approved plan (`references/module-plan.md` contents), the
diff range under review, and the checkout baseline (branch, tag,
derived kernel version). One finding per item, each naming the file,
the line, what the plan required, and what the diff does instead.

### Step 10: Audit and upgrade paths

- **Audit** (on request): run `references/module-checklist.md` and
  report each item pass/fail/N-A.
- **Upgrade** (on request): follow `references/upgrade-guide.md`
  (kernel-versioning digest): re-derive versions, diff the capability
  matrix, re-run conformance, re-verify both workspaces.

## Out of scope

- The admin panel SPA (`web/admin/`), API response reshaping, and email
  templates. Modules contribute manifest data only.
- Plugin/theme authoring beyond triage (Steps 1). Route to the sibling
  skill.
- Kernel evolution (new/widened contracts). Stop-and-decide only.
- First-party release mechanics for private modules (`make release`
  lockstep tags, proxy verification, `go install @version`). Private
  modules ship as source the operator forks and builds.

## Reference Index

- `references/module-development.md` — adapted user-facing snapshot of
  the module contract (layout, seams, gating, wiring).
- `references/kernel-contracts.md` — capability reality matrix
  (discovery, mounting, caveats, verification per contract).
- `references/integration-recipes.md` — custom-edition recipe,
  `go.work`/`SUBMODULES` steps, `GOWORK=off` check, platform-vs-edition
  decision.
- `references/troubleshooting.md` — dirty tree, version drift, mount
  failures, disabled-cascade surprises.
- `references/module-plan.md` — the plan artifact template (module
  portfolio, dependency graph, contracts, data, routes/auth, edition,
  tests, risks). Lives in session/workflow state, not the checkout.
- `references/module-checklist.md` — ship + audit checklist.
- `references/upgrade-guide.md` — kernel-versioning digest for the
  upgrade intent.
- `references/scaffold/` — tiered templates (core always;
  domain/api(+agent)/store/admin/testutil/migrations overlays) with
  `INSTRUCTIONS.txt` token resolution.
- `references/install-paths.md` — per-agent install snippets.
