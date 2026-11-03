# Module Plan

Fill this in during Plan Mode (SKILL.md Steps 1–3) and stop for
approval before any file is created or edited. The plan lives in the
session or workflow state — nothing extra is committed to the checkout.

## Goal

One or two sentences: what the feature does and who it serves.

## Intent and baseline

- Intent type: new private module / extend existing / custom-edition wiring / audit / upgrade.
- Checkout: absolute path, branch, tag or commit, derived kernel version.
- Build model for Steps 4–7 (recorded at approval).

## Module portfolio

One row per capability. Extend when the capability belongs to the same
bounded context and data owner; create when it needs an independent
lifecycle, disable cascade, or data ownership. Never a module per
endpoint or per table.

| Capability | Reuse / extend / new | Data owner | Contracts | Requires | Routes & auth | Migrations | Edition |
|---|---|---|---|---|---|---|---|
| | | | | | | | |

## Dependency graph

Implementation order following `Requires()` edges (leaves first).
Name the cascade each edge designs for the operator.

## Contracts per module

For each new/extended module: `RouteProvider` vs `PublicRouteProvider`
vs `DualAuthProvider`, `Migrator`, `ContentTypeProvider`,
`RoleProvider` (+ resolver), `ThemeProvider`/`ThemePageProvider`,
`AdminExtensionProvider`. Note the caveats (resolver enforcement,
`Init`-before-migrations, full `/api/v1/...` dual paths).

## Data and migrations

Tables owned, per-driver migration files, idempotency for pre-module
installs, no cross-module foreign keys. Backfill needs, if any.

## Routes and auth

Endpoints, realm (protected / public / dual), capability checks,
principal parsing, body limits.

## Edition shape

Custom edition name, `Wire`/`Mount` contents, `go.work` + `SUBMODULES`
entries. Platform-vs-edition placement for every cross-context seam.

## Test plan

Domain coverage target, handler/store/conformance tests, `GOWORK=off`
check, serve check, disabled-cascade expectation per surface.

## Risks and open questions

Kernel ambiguity, auth edge cases, destructive migrations, anything
that would trigger Escalation during Build.

## Approval

- [ ] Plan approved — switch to the build model for implementation (or continue here).
- [ ] Build model and edition target recorded above.
- [ ] Per-module `Version` (starts `0.1.0`) and `Optional()` soft deps recorded in Contracts.
