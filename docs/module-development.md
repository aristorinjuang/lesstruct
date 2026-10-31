# Module Development Guide

> **Audience.** This is the **developer-facing** reference for Lesstruct
> Tier 1 modules: first-party Go features compiled into the binary
> (`modules/<name>/`, each its own Go module).
>
> If you want sandboxed third-party extensions instead, write a **WASM
> plugin** — see [plugin-development.md](plugin-development.md). Tier 2
> plugins hook events at runtime; Tier 1 modules ship behavior at compile
> time. Both live on one event bus of concepts: modules own the kernel
> contracts, plugins consume hook payloads shaped like them.

The kernel (`kernel/`, Go module
`github.com/aristorinjuang/lesstruct/kernel`, currently `v1.0.0`) is the
contract. Read [kernel-versioning.md](kernel-versioning.md) first: semver
rules, stability classes, and the deprecation flow — especially the trap
that **adding a method to an exported interface is a breaking change**.

## Module anatomy

Every first-party module follows the same layout (see `modules/users` and
`modules/auth` for the full verticals, `modules/media` for the
single-source store pattern):

```
modules/<name>/
  go.mod            # module github.com/aristorinjuang/lesstruct/modules/<name>
                    # requires kernel (+ siblings) at the release tag; local
                    # checkouts resolve through the repo-root go.work instead
  module.go         # kernel.Module: options, Requires, handler accessors
  domain/           # services over narrow seam interfaces; 100% coverage bar
  api/              # HTTP handlers + seams.go; writes through kernel/httpx
  store/            # SQL adapter(s) for the domain's repository port
  infrastructure/   # external provider clients implementing a domain port
  admin/            # Manifest() + validation via kernel (see below)
  testutil/         # NewModule()/Kernel() fixture for module tests
  README.md         # what the module owns and its seams
```

The layers have a strict direction:

- `domain/` is the model: entities, value objects, sentinel errors, the
  domain service (application layer), and the ports it needs. It imports
  stdlib, `kernel/` published language, and — for a root-module context —
  sibling supporting modules' domains; never HTTP, SQL, or a remote provider
  client.
- `api/` decodes transport, reads the principal, and maps domain errors to
  the realm envelope. Business rules belong in the service.
- `store/` implements the repository port on `*sql.DB` + `kernel.Dialect`;
  it never leaks driver types upward.
- `infrastructure/` implements an outbound port against an external client
  (the textgen OpenAI client, the media image-generation clients). Cross-
  context adapters do **not** live here — they live in
  `internal/editions/<name>/`, the only layer that may import both sides.
- `payments/midtrans` shows the other shape: a self-contained driver package
  with its own protocol, kept out of both `domain/` and `infrastructure/`.

The full model vocabulary, bounded contexts, and context map are in
[ddd/domain-context.md](ddd/domain-context.md); read it before drawing a new
module boundary or moving code between layers.

`module.go` is options-built: the composition root supplies every
collaborator via `WithXxx` options, `New` builds the handlers **once**, and
typed accessors (`UserManagementHandler()`, …) expose them for explicit
mounting. `Requires()` declares hard dependencies — it is not documentation:
the registry topologically sorts on it, and the operator-facing
`[modules] disabled` cascade drops transitive dependents from it
(`auth` requires `users`; `comments` requires `users`; `adminpanel`
requires `auth`). Design `Requires()` as the disable cascade you want
operators to get. `Init` validates presence; real wiring happens in `New`.

Register new modules in the `SUBMODULES` list in the root `Makefile` so the
test/lint gates cover them alongside the root module.

## Kernel contracts reference

| Contract | Use it when… |
|---|---|
| `kernel.Module` / `kernel.Kernel` / `Registry` + `InitAll` | Always — the lifecycle every module implements. |
| `Capabilities[T]` | Discovering sibling modules instead of a service locator (theme providers, migrators, route providers, extension manifests). |
| `Principal` + `PrincipalFromContext` / `ContextWithPrincipal` | Reading the acting identity in handlers and services. Parse the decimal ID with the `principalID` pattern (fail closed on `kernel.ErrInvalidPrincipalID`); never invent a second identity shape. |
| `RouteProvider` / `PublicRouteProvider` | Mounting routes inside the existing protected/public groups. If your surface spans realms, body-limit groups, or per-route flags a single call cannot express, use a structural surface interface in `internal/api/routes` instead (the documented media/comments/auth/users deviation). |
| `ContentTypeProvider`, `RoleProvider`, `ThemeProvider`, `ThemePageProvider`, `AdminExtensionProvider`, `Migrator` | Contributing post types, capabilities, themes, theme-declared standalone pages, admin UI, or versioned migrations. |
| `Logger` (nil degrades to discard) | All module logging. |
| `Prune` | Never call directly — the composition root owns the disabled list. Know that it reads your `Requires()`. |

Published-language libraries (no adapters needed, import directly):
`httpx` (the one response envelope — byte-identical to the legacy writers
by contract), `content`, `customfield`, `database` (Driver/Dialect),
`storage`, `seo`, `sanitize`, `tiptap`, `i18n`, `posttype`, `capability`,
`conformance`.

## Seams and integration patterns

- **Own your seams narrowly.** Each `api/` package declares exactly the
  collaborator subset its handlers touch (`UserStore`, `SoftDeleteStore`,
  `Mailer`, `TokenIssuer`, …) as local interfaces. The core repository or
  service satisfies them structurally wherever the method sets already
  match.
- **Core adapters live in `internal/modules`** — the only layer that may
  import both sides. Intent-named recorders (`AuthNotificationRecorder`),
  null-type conversion (`SoftDeleteStore`), policy translation
  (`UserCredentialPolicy`): formatting and translation live in the adapter
  so the module never imports repository or database types.
- **Sentinels translate at the seam.** Handlers match module-domain errors
  with `errors.Is`; when a seam bridges two error vocabularies, mirror the
  message and pin it with a test asserting byte-equality.
- **Never FK across module boundaries.** A module's schema may foreign-key
  its own tables; the moment a constraint references another module's
  table, migrations couple (ordering, drop order) and delete semantics
  diverge per driver. Index the reference column instead — a proof's
  history surviving an orders-table lifecycle is a feature, not an
  orphan (see `modules/payments/migrations/*`).
- **Webhook signatures compare in constant time.** Any endpoint
  authenticating a caller by a keyed checksum (`crypto/hmac`,
  `sha512(payload + key)`, …) compares digests with
  `crypto/subtle.ConstantTimeCompare` — never `==` — so a forged event
  cannot probe the expected signature byte by byte
  (`modules/payments/midtrans` is the pattern).
- **Envelopes are byte contracts.** All handlers write through
  `kernel/httpx`; the compat suite diffs both writers, so any drift fails a
  test before a client ever sees it.
- **Manifests validate centrally.** `Manifest()` returns your data;
  `AdminExtensionManifest.Validate()` (labels present, paths absolute, no
  duplicate paths) is the shared check — never copy it per module.

## Gating your module

- **Depguard.** Nested modules (including their tests) must not import
  `lesstruct/internal` — enforced mechanically by `make depguard` via the
  compiler's own dep graph. Generate testify mocks with `make mock`
  (mockery); prefer small hand-written recording stubs for narrow seams
  where a full mock is heavier than the contract.
- **Coverage.** The domain layer carries a 100% statement bar; empty
  function bodies do not count. Unreachable defensive branches are
  documented per-branch exceptions, not faked tests.
- **Test conventions.** Table-driven suites with `name` first and `wantErr`
  where errors return; `tt` loop variable; `require` for setup failures,
  `assert` for the rest; factory functions over struct literals where they
  exist. Handler tests attach identity with a test-local principal helper
  mirroring what the auth middleware populates.
- **Conformance fixtures.** `kernel/conformance` ships the author-facing
  checks: `CheckAuthorizer` (+ `AuthorizerFixture`) for capability
  resolvers, `CheckAdminExtension` (+ error-form `ValidateAdminExtension`
  for non-testify harnesses) for manifest providers. Run the ones your
  module's contracts touch.
- **Migrations.** Implement `kernel.Migrator` for versioned tables
  (`schema_migrations_<module>`); idempotent `CREATE TABLE IF NOT EXISTS`
  so pre-module installs upgrade cleanly. They run on every compiled-in
  driver — including `nosqlite` builds, which exclude sqlite entirely.

## Composition-root wiring

`internal/app` (`Run`) is the shared composition core: services are
built, modules adopt them via options, candidates accumulate,
`ApplyDisabled` prunes the `[modules] disabled` list (warn on drops and
unknown names), kept modules register **before `InitAll`**, migrations run,
handlers mount. An **edition** is a fixed module list compiled into one
binary: the concrete editions live in `internal/editions/<name>` as separate
packages — `blog` (the default root main: every first-party module) and
`core` (the headless engine, installed as `lesstruct-core`) — so a binary
links only the modules its edition wires. Edition packages implement two
hooks (`Wire` before the disabled cascade and `InitAll`, `Mount` after it)
and their mains stay thin (`main.go`, `cmd/lesstruct-core/main.go`).
(The blueprint calls a fixed module list compiled into one binary an
*edition* — that word lives here, in the editions packages, and in history.)
Product-level mains stay thin; build variants like
`-tags nosqlite` are advanced compile options, not separate products.

Headless stays a config of the default composition (`[headless]` suppresses
public-surface mounting only) — never the disabled list.

## Ship checklist

1. Layout matches the template; `go.mod` requires kernel (+ siblings) at the release tag and carries no `replace` directives (a filesystem `replace` in a published `go.mod` makes `go install pkg@version` fail — local checkouts resolve through `go.work` instead).
2. `Requires()` is the cascade you intend; listed in `SUBMODULES`.
3. Handlers write through `kernel/httpx`; envelope parity covered.
4. Manifest validates; conformance fixture adopted in the module test.
5. Domain coverage at bar; depguard clean; lint + modernize clean.
6. `README.md` names the seams; `docs/project-context.md` gains the entry.

## Releasing (first-party)

Releases are lockstep: the root module (`vX.Y.Z`), the kernel
(`kernel/vX.Y.Z`), and every first-party module (`modules/<name>/vX.Y.Z`)
are tagged at the same commit, and every `go.mod` requires the sibling
modules at that same version. Run `make release VERSION=vX.Y.Z`, which
rewrites the requires and verifies the workspace builds through a throwaway
file proxy (local checkouts resolve siblings through the committed `go.work`
workspace — never through `replace`; workspace builds keep `go.work.sum`
updated by themselves), then prints the commit + tag + verify sequence. `go
install` needs the module tags, so versions before `v1.1.1` (whose `go.mod`
files still carry filesystem `replace` directives) cannot be installed that
way — use a release binary or build from source instead.

## Building and installing third-party modules

Everything above applies to third parties too — the template, the seams,
the gates. The honest question is **integration**: how does your module get
into a running Lesstruct?

### What a module can contribute

A module is not limited to one thing. Through the kernel capability
interfaces it can contribute any combination of:

| Surface | Contract | Contributes |
|---|---|---|
| Protected routes | `RouteProvider` | Authenticated HTTP endpoints mounted in the admin/API group. |
| Public routes | `PublicRouteProvider` | Anonymous HTTP endpoints (public API, SSR pages). |
| Themes | `ThemeProvider` + `ThemePageProvider` | Embedded layouts, CSS, template funcs, and site-root files for the SSR engine; optional standalone pages served at fixed public paths (`cart`, `checkout`, `orders/track`). |
| Admin UI | `AdminExtensionProvider` | Manifest entries the admin shell renders: menu, settings, editor extensions. |
| Post/content types | `ContentTypeProvider` | Registered types beyond `post`/`page`. |
| Capabilities | `RoleProvider` | Capability strings for the permission registry. |
| Migrations | `Migrator` | Versioned `schema_migrations_<module>` tables on every compiled-in driver. |

Beyond the contracts, a module owns its `domain/` services and `api/`
handlers exactly like the first-party exemplars.

### Tier 1 vs Tier 2: do you need to recompile?

| | Tier 1 — Go module | Tier 2 — WASM plugin |
|---|---|---|
| Ships as | Source, compiled into the binary | A `.wasm` file dropped into `plugins/` |
| Recompile needed | **Yes** — it is part of the binary | **No** — loaded at startup, hot-reloaded in `DEV_MODE` |
| Reach | Full: routes, themes, admin UI, migrations, storage | Event hooks (`before_save`, `after_publish`, …) via host functions |
| Sandboxing | None — trusted code | Yes — wazero runtime, capability manifest |
| Right for | Product forks, internal distributions, deep integrations | Third-party runtime extensions, optional add-ons |

### Installing a Tier 1 module (the fork/build path)

There is no runtime installation for Go modules: **you compile your own
Lesstruct**. The composition root is deliberately a boring, machine-writable
list, so this is four steps in a fork of the repository:

1. `modules/<yourname>/` — write the module per this guide (or copy the
   closest first-party exemplar and rename). Because you are inside the same
   module tree, `internal/` (the composition root, adapters) stays
   importable to you.
2. Add it to `SUBMODULES` in the root `Makefile` so the gates cover it.
3. Register it in `internal/app/app.go`: build it from options after its
   dependencies exist and append it to the candidates list — `InitAll`,
   migrations, the disabled cascade, and the admin manifest pick it up
   unchanged.
4. Build your binary: `go build .` or your own main
   package calling `app.Run(edition)` (or one of the shipped editions under
   `internal/editions/`) under any build constraints.

Nothing else changes: routes mount through your declared providers, the
admin manifest lists your entries, `[modules] disabled = ["<yourname>"]`
prunes you like every other module, and `make test` runs your suite.

Shipping a module to *other* people's forks is the same mechanics with a
different diffusion: publish the source, they perform steps 1–3. A
`lesstruct mod get`-style installer that generates the composition
automatically is a known post-v1 direction — the boring-list design above is
what keeps that door open.

### The kernel is the stability contract

Everything your module compiles against lives in `kernel/`
(`kernel/vX.Y.Z`), versioned independently —
[kernel-versioning.md](kernel-versioning.md) is the promise your build can
rely on, and [the kernel](kernel.md) is the map of what you are building on.
