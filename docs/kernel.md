# The Kernel

> **Audience.** Lesstruct developers and advanced module authors who want to
> understand, evolve, or extend `kernel/` — the Go module every Lesstruct
> module is written against.

The kernel (`github.com/aristorinjuang/lesstruct/kernel`, tagged
`kernel/vX.Y.Z`) is the public contract of Lesstruct v1: the lifecycle every
module implements, the identity every handler reads, the envelope every
response uses, and the discovery mechanism that lets modules cooperate
without knowing each other. It is deliberately small and deliberately
one-way: **the kernel imports nothing from Lesstruct modules or internals.**
Modules import the kernel; the composition root imports everything.

Two rules made it survive five phases of reviews, and they are the rules to
keep:

1. **Minimalism.** Nothing enters the kernel without a module needing it.
   Dead contracts are removed before a release tag, never deprecated in
   place (the event bus and four unused provider interfaces were removed
   exactly this way in the v1 freeze).
2. **Narrow over wide.** Adding a method to an exported interface is a
   breaking change — every implementer breaks. Grow through new narrow
   interfaces, never by widening existing ones.

## Package map

| Package | Class | What it is |
|---|---|---|
| (root) | Contract | `Module`, `Kernel`, `Registry`, `Prune`, capability/provider interfaces, `Principal`, `Logger`, admin-manifest types |
| `capability` | Contract | Capability-string vocabulary (`Split`, `ManageContent`). |
| `conformance` | Fixture | Embeddable test suites third parties run against their implementations (`CheckAuthorizer`, `CheckAdminExtension`). |
| `httpx` | Published language | The one JSON response envelope; byte-identical to the legacy writers by pinned test. |
| `content`, `customfield` | Published language | Content/field types and validation shared by every content-touching module. |
| `database` | Contract | `Driver`/`Dialect` abstraction (placeholders, `RETURNING`, upsert, duplicate mapping) — how single-source stores run on three databases. |
| `storage` | Contract | `Save`/`Delete`/`GetURL` file-storage seam. |
| `seo`, `sanitize`, `tiptap`, `i18n`, `posttype` | Published language | SEO metadata/feeds, HTML policies, TipTap rendering, translations, the post-type index. |
| `posttype` | Contract | Registry index of content types (`Capabilities[ContentTypeProvider]` feeds it). |

"Contract" means strict semver and adversarial review of every change;
"published language" means semver with deprecation notices — see
[kernel-versioning.md](kernel-versioning.md).

## Lifecycle

1. The composition root (`internal/app`) constructs modules from options and
   appends them as candidates.
2. `kernel.Prune(candidates, disabled)` removes everything the operator
   disabled via `[modules]` plus every transitive `Requires()` dependent
   (cascade), with reasons.
3. Kept modules register on the `Registry`; `InitAll` resolves the
   dependency graph topologically and calls `Init(host)`. A kept module
   whose hard dependency is absent fails startup fast.
4. Capability discovery: `kernel.Capabilities[T](registry)` collects every
   registered module implementing contract `T` — themes, migrators, route
   providers, admin extensions — in name order. This is the only discovery
   mechanism; the kernel is never a service locator.
5. `ShutdownAll` runs the reverse order.

When you design a module's `Requires()`, you are designing the operator's
disable cascade. When you implement a new contract, you are designing what
`Capabilities[T]` finds.

## Evolving the kernel

Adding a contract — the checklist, in order:

1. **The need is real.** At least one module (usually first-party) requires
   it today, or a frozen API forced a documented deviation that this
   contract removes. "Might be useful" is the event-bus trap: it shipped in
   the kernel, gathered zero consumers, and was deleted at the freeze.
2. **Narrowest surface.** One or two methods, named for the capability
   (`AdminExtensionProvider`, `Migrator`), carrying only what discovery
   needs. Data belongs in small structs next to the interface.
3. **Semver classify.** New interface = minor. Anything that widens an
   existing interface = major. Say it out loud before writing code.
4. **Conformance fixture.** If third parties will implement it, ship a
   `kernel/conformance` check (`CheckAuthorizer` and `CheckAdminExtension`
   are the patterns) and adopt it in a first-party module test so it cannot
   rot.
5. **Docs in the same change.** The contract table in
   [module-development.md](module-development.md), this page's map, and the
   sync-table row in `AGENTS.md`.

Evolving published-language libraries is the same discipline with a softer
tag: behavior-preserving additions are minors, deprecations name the
replacement and survive until the next major.

Hard rules, enforced and reviewed:

- The kernel imports only stdlib + vendored-independent libs — never
  `lesstruct/internal`, never `modules/*`.
- No `panic`; errors are values, wrapped with `%w`.
- `Capabilities[T]` is the only discovery path.
- Nothing enters without a module needing it (review gate: kernel
  minimalism).
