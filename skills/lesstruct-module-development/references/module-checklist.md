# Module Checklist — Ship + Audit

Run each item and report pass / fail / not applicable. The module ships
when every applicable item passes.

## Layout

- [ ] `modules/<name>/` matches the template: `go.mod`, `module.go`,
  `domain/`, `api/`, `store/` (when persistent), `infrastructure/`
  (when calling external providers — never in `domain/`),
  `admin/`, `testutil/`, `README.md`.
- [ ] `go.mod` requires kernel (+ siblings) at the checkout's release
  tag and carries **no** `replace` directives.
- [ ] `embed.go` carries the migration FS with `//go:embed` on the spec
  **inside** the `var (...)` block.
- [ ] File layout contract holds (const → var → private helpers →
  public declarations; methods with their struct; constructor last).
  `make stylecheck` green.
- [ ] Registered in `go.work` (`use`) and root `Makefile` `SUBMODULES`.
  Private modules stay out of the release EDGE map and
  `docs/built-in-modules.md`.

## Contracts

- [ ] `Name()` stable registry key; `Requires()` is the intended
  disable cascade; `Optional()` degrades (never fails) when absent.
- [ ] Handlers write through `kernel/httpx` only (envelope parity).
- [ ] Principal read from context; decimal ID parsed fail-closed on
  `kernel.ErrInvalidPrincipalID`; no second identity shape.
- [ ] Correct route surface: `RouteProvider` (protected),
  `PublicRouteProvider` (anonymous), or `DualAuthProvider` (both, full
  `/api/v1/...` patterns, no collisions). No multi-group surface
  forced through a single call.
- [ ] Public-router bodies capped with `http.MaxBytesReader` before decode.
- [ ] Capabilities declared (`RoleProvider`) **and** enforced (resolver
  seam + in-handler checks, fail closed on unknown).
- [ ] `Manifest()` validates via `AdminExtensionManifest.Validate()`
  (no per-module copy); conformance adopted
  (`CheckAdminExtension` / `ValidateAdminExtension`; `CheckAuthorizer`
  for resolvers); manifest deterministic across calls.
- [ ] Migrations per driver (`{sqlite,postgresql,mysql}/`), idempotent
  (`CREATE TABLE IF NOT EXISTS`), tracked in
  `schema_migrations_<module>`; no cross-module FKs (indexed reference
  columns instead); `nosqlite` build considered.
- [ ] Webhook signatures compare with
  `crypto/subtle.ConstantTimeCompare`, never `==`.
- [ ] Sentinels translate at the seam (`errors.Is`); mirrored messages
  pinned byte-equal by test.
- [ ] `Init` validates presence only (no tables — migrations run after
  `InitAll`); `Shutdown` releases what `Init` acquired.
- [ ] Logging via injected `kernel.Logger` (nil degrades to discard);
  no `panic`, no `fmt.Println`/`log.*` outside `internal/app`.

## Gating

- [ ] Domain coverage at 100% statements (empty bodies do not count;
  unreachable branches are documented per-branch exceptions).
- [ ] Table-driven suites (`name` first, `wantErr` where errors return,
  `tt` loop variable, `require` for setup / `assert` for the rest,
  factory constructors over struct literals).
- [ ] `make depguard` clean (no `lesstruct/internal` imports, tests
  included). Mocks via `make mock` (mockery) or narrow hand-written
  stubs.
- [ ] `make lint` (modernize + stylecheck + golangci-lint) clean.
- [ ] `make vulncheck` clean.

## Wiring (custom edition)

- [ ] Private module wired via `internal/editions/<custom>/` + thin
  `cmd/<custom>/main.go` — never `internal/app` by default.
- [ ] `Wire` constructs services/stores, `New`s the module with options,
  appends to `core.Candidates`, fills optional handler surfaces;
  `Mount` assembles post-`InitAll` state. Adapters one-seam-per-file;
  `wire<Module>` steps beside siblings, never in adapter files.
- [ ] Disabled cascade verified (`[modules] disabled = ["<name>"]`
  prunes + dependents with warnings; routes unmount where flag-gated).
- [ ] Second-theme goals routed to `EditionWithTheme`, never a second
  `ThemeProvider` registration.

## Verification

- [ ] `go test ./...` in-workspace green.
- [ ] `GOWORK=off go test ./...` (or `go build ./...`) in the module green.
- [ ] `go build ./...` whole workspace green; `go build -tags nosqlite
  ./...` considered; custom binary links and serves the surface.
- [ ] `README.md` names the seams; `docs/project-context.md` gains the
  entry when the change is architecture-level (same-change doc rule).

## Upgrade (when bumping Lesstruct/kernel)

- [ ] Versions re-derived from the checkout; `go.mod` requires moved
  together (lockstep); no `replace` reintroduced.
- [ ] Capability matrix re-checked against the new `kernel/` tree.
- [ ] Conformance fixtures re-run; envelope compat still pinned.
- [ ] Both workspaces re-verified (in-workspace + `GOWORK=off`).
