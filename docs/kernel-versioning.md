# Kernel versioning

The kernel (`kernel/`, Go module `github.com/aristorinjuang/lesstruct/kernel`) is versioned independently from the main module. Tags follow the subdirectory-module convention: `kernel/vX.Y.Z`.

## Semver rules

- **Major**: removing or changing the meaning of any exported symbol; adding a method to an exported interface (every implementer breaks); changing a capability contract or envelope shape.
- **Minor**: new exported symbols, new capabilities, new optional interface methods on structs (never on interfaces consumed by third parties).
- **Patch**: documentation, internal refactors, test changes.

The interface-method trap is the reason this file exists: Go interfaces are
implemented structurally, so **adding a method to `kernel.Module`-style
interfaces is a breaking change**. Prefer new narrow interfaces over
widening existing ones.

## Stability classes

- **Contracts** (`Module`, `Kernel`, capability/provider interfaces, `Principal`, registry semantics, `Prune`): strict semver. Errors are values — new sentinel errors are minor; changing an existing error's meaning is major.
- **Published-language libraries** (`httpx`, `content`, `customfield`, `database` Driver/Dialect, `storage`, `seo`, `sanitize`, `tiptap`, `i18n`, `posttype`, `capability`, `conformance`): semver with deprecation notices. Behavior-preserving additions are minor.
- **Minimalism rule**: nothing enters the kernel without a module needing it. Dead contracts are removed before a major tag (as in v1.0.0: the event bus and four unused provider interfaces), never deprecated in place.

## Deprecation flow

1. Mark deprecated in a minor release with a `Deprecated:` doc comment naming the replacement.
2. Remove no earlier than the next major release.
3. The main module and all first-party modules migrate off deprecated symbols before the major tag.

## Tag scheme

- `kernel/vX.Y.Z` tags the kernel module. First-party modules and the main module pin it via `require` + `replace` for local development; external consumers resolve the tag. The kernel ships exactly one tag per release, **aligned with the main module's tag** (`v1.0.0` + `kernel/v1.0.0` land together).
- The main module tags separately (`vX.Y.Z` at the repo root) once the phase gate passes. Never tag mid-phase.

## Admin API versioning

`GET /api/admin/extensions` serves the versioned manifest aggregation
(`ExtensionsVersion`, currently 1):

- Bump the version **only on a breaking shape change** (renamed/removed
  fields, changed types). A future shell must not render unknown majors.
- Additive JSON fields never bump — decoders ignore unknown fields.
- Empty collections serialize as `[]`, never null; the golden JSON test in
  the adminpanel module pins the exact wire shape.
- Module authors validate manifests with
  `AdminExtensionManifest.Validate()` and prove the provider contract with
  the `kernel/conformance` fixture (`CheckAdminExtension` for testify
  harnesses, `ValidateAdminExtension` in error form otherwise).
