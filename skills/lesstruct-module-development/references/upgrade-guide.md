# Upgrade Guide — Kernel-Versioning Digest for Module Authors

> Full policy: `docs/kernel-versioning.md`. Tags ride together —
> root `vX.Y.Z`, `kernel/vX.Y.Z`, every `modules/<name>/vX.Y.Z` on one
> commit; every `go.mod` requires the siblings at that tag. This page is
> the module-author digest for the **upgrade** intent.

## What a bump can do to you

- **Major** (any `vX` change, or any exported-interface method added):
  every implementer of the widened interface breaks. Adding a method to
  `kernel.Module`-style interfaces is breaking — prefer new narrow
  interfaces. Module code that implements the touched interface must
  change.
- **Minor** (new symbols, new capabilities, new struct-option methods):
  additive. Adopt what the feature needs; ignore the rest.
- **Patch** (docs, internal refactors, tests): no module change.

Stability classes: **contracts** (`Module`, `Kernel`, capability/provider
interfaces, `Principal`, registry/`Prune` semantics) are strict semver —
new sentinel errors are minor, changed meanings are major.
**Published-language** libraries (`httpx`, `content`, `customfield`,
`database`, `storage`, `seo`, `sanitize`, `tiptap`, `i18n`, `posttype`,
`capability`, `conformance`) evolve with deprecation notices —
behavior-preserving additions are minor.

The minimalism rule protects you: nothing enters the kernel without a
module needing it, and dead contracts are removed before a major tag
(the event bus + four unused providers died exactly this way at v1).
What you compile against is what some module uses.

## Upgrade procedure

1. **Re-derive versions** (SKILL.md Step 2): checkout the new tag,
   verify the tree is clean, read Lesstruct from
   `git describe --tags` and kernel from the pinned sibling require
   (`awk '/lesstruct\/kernel / {print $2}' modules/cart/go.mod`).
   Update the module's `go.mod` requires to the new tag together —
   lockstep, no `replace`.
2. **Diff the matrix**: re-read `references/kernel-contracts.md`
   against the new `kernel/` tree. Check each contract the module
   touches (routes, migrator, post types, roles, theme, admin) for
   widened interfaces, renamed symbols, and envelope changes.
3. **Handle deprecations**: `Deprecated:` comments name the
   replacement. Migrate in the minor; removal lands no earlier than
   the next major. The main module and all first-party modules migrate
   before the major tag — follow the same order.
4. **Re-run conformance**: `CheckAuthorizer` / `CheckAdminExtension`
   (or `ValidateAdminExtension`), envelope compat, manifest golden
   JSON where applicable.
5. **Re-verify both workspaces**: in-workspace `go test ./...` plus
   `GOWORK=off` module tests/build. Version drift shows up here first.
6. **Admin API note**: `GET /api/admin/extensions` carries
   `ExtensionsVersion` (currently 1) — bumped only on breaking shape
   change. Additive JSON fields never bump; empty collections stay
   `[]`, never null. If the upgrade bumped it, update manifest
   consumers; otherwise ignore.

## `go install` caveat

`go install pkg@version` needs the module tags — versions before
`v1.1.1` (whose `go.mod` files still carry filesystem `replace`
directives) cannot be installed that way. Use a release binary or build
from source for those.
