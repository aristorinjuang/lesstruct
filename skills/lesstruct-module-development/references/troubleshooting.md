# Troubleshooting — Module Development

## Dirty tree — HALT

**Symptom:** `git status --porcelain` prints anything.

**Fix:** commit or stash first, then re-verify. The skill never
scaffolds or edits over a dirty tree — a dirty baseline makes later
failures unattributable and risks overwriting uncommitted work.

```bash
git status --porcelain   # must be empty before Step 4
git stash push -m "pre-module baseline"  # or: git commit
```

## Not a checkout / missing markers

**Symptom:** no `go.work`, `kernel/go.mod`, or
`docs/module-development.md` at the supposed root.

**Fix:** ask the user for the absolute path to their clone (never
invent one). If they have none:

```bash
git clone https://github.com/aristorinjuang/lesstruct.git
cd lesstruct
git checkout <stable-tag>   # private modules build on a tag, not main
```

## Version drift (kernel / Lesstruct)

**Symptom:** scaffold's `__KERNEL_VERSION__` does not match the
checkout; `GOWORK=off` resolves a different kernel than in-workspace;
upgrade changed a contract shape.

**Fix:** re-derive both versions (SKILL.md Step 2):

```bash
git describe --tags --match "v*" --always --dirty
awk '/lesstruct\/kernel / {print $2}' modules/cart/go.mod
```

Then re-read `references/kernel-contracts.md` against the checkout's
`kernel/` tree and run the upgrade path (`references/upgrade-guide.md`).
Published `go.mod` files carry no `replace` — drift is fixed by
re-requiring the tag, never by a filesystem override. Versions before
`v1.1.1` cannot be `go install`ed (their `go.mod` files still carry
filesystem `replace` directives) — build from source instead.

## Mount failures (routes 404 / realm wrong)

| Symptom | Likely cause | Fix |
|---|---|---|
| New route 404s everywhere | Provider not registered (not in `core.Candidates`), or pruned by `[modules] disabled` cascade | Check the boot log's `Modules initialized:` line; check `[modules] disabled` + cascade warnings |
| Protected route reachable anonymously | Mounted via `PublicRouteProvider` instead of `RouteProvider` | Move to `RegisterRoutes` (protected group) |
| Anonymous route demands login | Mounted via `RouteProvider` instead of `PublicRouteProvider` | Move to `RegisterPublicRoutes` (public group) |
| Dual-realm path works on one realm only | Registered twice by path instead of once through `DualAuthRegistrar` (Chi routes by path — the second registration shadows) | Implement `DualAuthProvider.RegisterDualRoutes` with full `/api/v1/...` patterns; verify with both a Bearer `lesstruct_…` key and a JWT cookie |
| Public `POST` rejects large bodies | Route-level `MaxBytesReader` missing on the public router | Cap first: `r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)` — the public router rate-limits requests, not bytes |
| Module drop leaves endpoints mounted | Expected for non-`comments` modules | Disable is lifecycle-level (no `Init`, no migrations, no manifest) — only `comments` unmounts routes. Provider-discovered routes unmount when dropped; explicit `HandlerSet` surfaces may keep serving. Do not rely on disable as an HTTP kill-switch |

## Disabled-cascade surprises

**Symptom:** disabling one module drops others (`disabled = ["users"]`
drops `auth`, `comments`, `adminpanel`; `catalog` drops `cart` drops
`orders` drops `payments`).

**Fix:** this is the designed cascade (`Requires()` edges in
`kernel/prune.go`). Read the boot warnings — each names its cause
(`module "auth" disabled: requires disabled module "users"`). Unknown
names warn and are ignored (configs stay portable across editions).
Under cascade, services built pre-prune keep the pre-cascade flag value
while routes unmount — direct `disabled = ["comments"]` is the only
fully coherent route-mount parity (covered by tests). Design
`Requires()` as the cascade you want operators to get.

## Init vs migrations ordering

**Symptom:** `Init` fails on a missing table.

**Fix:** by design — migrations run **after** `InitAll`. `Init` must
only validate presence ( collaborators non-nil, config resolvable);
table creation belongs in idempotent per-driver migrations
(`CREATE TABLE IF NOT EXISTS`, tracked in
`schema_migrations_<module>`). Pre-module installs already carry the
tables from the core sequence, so non-idempotent migrations break
upgrades.

## Authorization bypass (`RoleProvider` alone)

**Symptom:** declared capabilities do not gate anything.

**Fix:** `RoleProvider.Capabilities()` only declares strings. Gates live
in handlers through the `CapabilityResolver` seam (adopted via
`WithRoles` / `WithMediaRoleService` / …) with fail-closed unknown
handling. Add the resolver adoption + the in-handler check, and prove
it with `conformance.CheckAuthorizer`.

## Second theme does nothing

**Symptom:** a newly registered `ThemeProvider` never renders.

**Fix:** first-provider-wins (`internal/app/app.go` installs
`providers[0]`). A second registration is not a theme replacement.
Swap via `EditionWithTheme` (blog/commerce pattern) or `THEME_DIR`
overrides — the supported child-theme path.

## `GOWORK=off` fails, in-workspace passes

**Symptom:** `go test ./...` green, `GOWORK=off go test ./...` red
(missing require, wrong version, stray `replace`).

**Fix:** the module's `go.mod` is incomplete. Add the missing
`require` at the checkout's release tag; drop any filesystem `replace`
(local checkouts resolve siblings through the committed `go.work`).
Never run a bare `go work sync` to fix it — it strips the
intra-workspace requires the release process maintains.
