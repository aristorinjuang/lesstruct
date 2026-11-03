<!--
  This is the CANONICAL agent-instructions file. `CLAUDE.md` is a symlink to it,
  so edit rules HERE — Claude Code reads it through `CLAUDE.md`, OpenCode reads
  `AGENTS.md` directly. Both see exactly this content.
-->

# Lesstruct — Agent Instructions

## Ramp-up — read in this order before your first task

1. **This file** — coding, testing, and doc-sync conventions.
2. **`docs/_index.md`** — the doc map ("for X, read Y").
3. **`docs/project-context.md`** — architecture, layers, and the **"Where does new code go?"** decision tree.
4. **`docs/api-reference.md`** — only if you are touching `/api/v1`.
5. **One exemplar of each layer**: a handler (`internal/api/handlers/`), a service (`internal/domain/<name>/service.go`), and a `*_test.go` under `internal/domain/`.

> The architecture, layer responsibilities, error/response conventions, and the
> decision tree live in `docs/project-context.md`. This file is the *style & process* contract; that file is the *architecture* contract. Read both.

## Code Style

- **File layout order (top to bottom):**
  1. `const (...)` — all file-level constants, grouped in one block. A lone file-level constant may be written bare (`const maxCreateAttempts = 3`); from the second one on, group them.
  2. `var (...)` — all file-level variables — exported test seams (`var Reader io.Reader = ...`) and sentinels included — grouped in one block. Compile-time interface witnesses (`var _ kernel.Module = (*Module)(nil)`) are the established exemption: they sit beside the type they assert and need not join the block.
  3. Private structs and functions. Package-level unexported helpers live here even when their only callers are exported methods. Private sentinel/error types precede the `var (...)` block that uses them.
  4. Public declarations — each struct followed by its methods (private methods first, then exported; the block is contiguous — no other declaration may interrupt it), then its constructor last, after all methods. Nothing of that type follows the constructor.
- The file-layout rules above are enforced mechanically by `tools/stylecheck` (`make stylecheck`, wired into `make lint`): methods stay with their struct, constructors close their type, and package-level helpers precede public declarations.
- Methods (functions with receivers) go immediately after the struct they belong to — including private structs.
- Constructors are the final declaration of their type: private helpers never follow them. This applies to **every** type that has a constructor — HTTP handlers, seam adapters, and drivers included — not just domain services and modules.
- Every exported symbol carries a godoc comment beginning with the symbol's name ("Create generates ...", never "This function generates ...").
- Do not ever use `interface{}`, use `any`!
- Always use constants instead of typed strings, especially to define HTTP methods! E.g., `http.MethodDelete` instead of `"DELETE"`.
- **Write modern Go (the toolchain is Go 1.27).** The `modernize` analyzer (`make modernize`) enforces this in CI; write the modern form directly:
  - `for i := 0; i < n; i++` → `for i := range n` (and `for y := 0; y < h; y++` → `for y := range h`).
  - Manual `contains` loops → `slices.Contains`; `sort.Slice`/`sort.Strings` → `slices.SortFunc`/`slices.Sort`; loop-built maps/keys → `maps.Copy`, `maps.Keys`, `slices.Sorted`.
  - Manual `min`/`max` `if`-blocks → the `min`/`max` builtins.
  - `if strings.HasPrefix(s, p) { strings.TrimPrefix(...) }` → `strings.CutPrefix`; same for `CutSuffix`; `strings.Split` + loop → `strings.SplitSeq`; `strings.IndexByte` + slice → `strings.Cut`.
  - `string += string` in a loop → `strings.Builder`.
  - `wg.Add(1); go func() { defer wg.Done() }()` → `wg.Go(func() {...})` (Go 1.25+).
  - `os.Setenv` in tests → `t.Setenv` (auto-restores; skip in `TestMain` and `t.Parallel` tests). `os.Unsetenv` stays as-is — there is no `t.Unsetenv`.
  - `sort`-unrelated legacy: never leave `omitempty` on nested struct fields (it has no effect); `new(expr)` (Go 1.26) over `func ptr(v T) *T { return &v }` helpers.

- If a function has many arguments, do not put those arguments into one line, but use multiple lines instead!
```go
// Good example

authHandler := handlers.NewAuthHandler(
  authService,
  jwtManager,
  logger,
  firstLoginService,
  registrationService,
  verificationService,
  loginService,
  userRepo,
  failedLoginRepo,
  notificationRepo,
  emailService,
)
```
```go
// Bad example

authHandler := handlers.NewAuthHandler(authService, jwtManager, logger, firstLoginService, registrationService, verificationService, loginService, userRepo, failedLoginRepo, notificationRepo, emailService)
```

### Module & adapter file layout

First-party `modules/<name>/` trees and edition adapter files follow this
organization:

- Adapters live one-seam-per-file in the edition package, named for the
  module they feed (`cart.go`, `orders.go`): fields, seam methods in the
  interface's declaration order, then the `New*` constructor. Wire steps
  (`wire<Module>` methods on the composition `state`) never mix into
  adapter files — they stay in the edition's main file beside their
  sibling wires.
- `embed.go` carries the migration FS with `//go:embed` written directly
  on the spec *inside* the `var (...)` block; outside the block the
  directive is misplaced and fails the build.
- Any handler decoding a request body on the anonymous public router
  caps it first with `r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)`
  — the public router rate-limits requests, not bytes.

### Domain package file grammar

A domain package under `internal/domain/<name>/` uses a fixed file layout:

- `types.go` — entity/DTO structs and ALL `Err*` sentinel errors (one grouped `var (...)` block).
- `repository.go` — the repository interface.
- `service.go` — the service "class": const → var → private helpers → struct → methods (private, then exported) → constructor last.
- `service_test.go` — table-driven tests in the `<name>_test` package.

## Structs & Constructors

- Treat `struct` like objects if they have function receivers (methods)! Put function receivers (methods) right after their `struct`, and constructors after all methods.
- **Constructor definition:** a constructor is any function whose FIRST return value is the struct that owns the methods — the name is irrelevant. `New*` is common, but `Open`, `Load`, `LoadRoles`, and `LoadManifest` are equally constructors. Every constructor rule applies to them: methods first, constructor last, nothing follows.
- **Receivers:** one style per type, never mixed. Services, handlers, adapters, and anything holding mutable state or dependencies use pointer receivers (`func (s *Service) ...`). Small immutable value types may use value receivers.

```go
// Good example

type Name struct {
    First string
    Last  string
}

func (n Name) Full() string {
    if n.Last == "" {
        return n.First
    }
    return fmt.Sprintf("%s %s", n.First, n.Last)
}

func NewName(first, last string) (Name, error) {
    if first == "" {
        return Name{}, errors.New("first name cannot be empty")
    }
    return Name{
        First: first,
        Last: last,
    }, nil
}
```

```go
// Bad example

type Name struct {
    First string
    Last  string
}

func NewName(first, last string) (Name, error) {
    if first == "" {
        return Name{}, errors.New("first name cannot be empty")
    }
    return Name{
        First: first,
        Last: last,
    }, nil
}

func (n Name) Full() string {
    if n.Last == "" {
        return n.First
    }
    return fmt.Sprintf("%s %s", n.First, n.Last)
}
```

## Logging & Error Handling

- Do not ever use `panic()`!
- Use `log.Fatalf()` or `log.Panicf()` only in the `main.go`!

## Mocks

- Always use `make mock` to generate mock files!
- Always use `github.com/stretchr/testify/mock` for writing mocks.

## Testing

- Do not test your works by `go build` or `go run`, but by `go test` instead!
- Always use packages that end with `_test` for all test files. Do not test private functions directly, but through the public ones.
- Always use `github.com/stretchr/testify` for writing unit tests.
- Make sure the domain layer ( @internal/domain/ ) has 100% test coverage! Remove unreachable code or skip errors using `_` variables if needed.
- Ensure your works pass `make lint`, `make test`, and `make vulncheck`! Run `make test-race` **only** when you touch concurrency code (`goroutine`, `sync`, channels, `http.Handler`, or code that shares mutable state across requests) — CI (`/.github/workflows/ci.yml`) already runs the race detector on every push, so a local run is slow and redundant for non-concurrent changes. For a fast scoped check, invoke the toolchain directly on the package you touched instead of the `./...` target: `go test -race ./internal/domain/<name>/...`.

### Before Touching Any Test File
- **Read the entire test file first.** Never edit based on partial context.
- Identify the exact struct field names in the test table before writing anything.
- If the function under test uses factory functions, read their signatures before using them.

### Never Use These to Edit `.go` Files
- **Never use `sed`, `awk`, or any regex-based shell command to edit `.go` files.** Always use full-block rewrites or exact string replacement.

### Test File Structure
- All tests must be table-driven using a named struct slice.
- Always use `tt` as the loop variable and `t.Run(tt.name, ...)` for subtests.
- Always include a `name string` as the first field in the test struct.
- Always include `wantErr bool` if the function under test returns an error.

```go
// Good example

func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected ExpectedType
        wantErr  bool
    }{
        {
            name:     "success - valid input",
            input:    NewInputType(...),
            expected: NewExpectedType(...),
            wantErr:  false,
        },
        {
            name:    "error - invalid input",
            input:   NewInputType(...),
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionUnderTest(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Adding or Updating Test Cases
- **Always rewrite the entire `tests := []struct{...}{...}` block.** Never insert or patch individual lines inside it.
- Preserve all existing test cases unless explicitly told to remove one.
- Preserve the existing struct field order exactly — do not reorder fields.
- Always put a trailing comma after the last field value and after the last struct entry.

```go
// Good example — trailing commas

{
    name:     "success",
    input:    NewInputType(...),
    expected: NewExpectedType(...),
    wantErr:  false,   // ← trailing comma on last field
},                     // ← trailing comma on last entry
```

```go
// Bad example — missing trailing commas causes syntax errors

{
    name:     "success",
    input:    NewInputType(...),
    expected: NewExpectedType(...),
    wantErr:  false    // ← missing comma = syntax error
}                      // ← missing comma = syntax error
```

### Constructing Objects in Tests
- **Always use factory functions (`New...`), never raw struct literals.**
- If no suitable factory function exists, ask before inventing a struct literal.
- Apply the same multi-line argument rule — one argument per line if there are many.

### Testify Usage
- Use `require.*` when the test cannot meaningfully continue on failure (e.g., after checking `err`).
- Use `assert.*` for all other checks.
- Always put `expected` before `actual`: `assert.Equal(t, tt.expected, result)`.
- Use `assert.ErrorIs(t, err, ErrSomething)` for specific error type checks, not plain `assert.Error`.
- Never use `assert.True(t, a == b)` — always use `assert.Equal`.

### After Every Test Edit
- Run `go test ./path/to/package/... -v -run TestFunctionName` to verify the specific test.
- If it fails, fix it before moving to the next task.
- If it fails twice with no clear cause, stop and explain what you observe — do not keep guessing.

## Documentation Sync

The docs under `docs/` are contracts, not afterthoughts. When you change code that a doc describes, update the doc in the same change — never leave it for "later".

| If you touch... | Update... |
|---|---|
| `go.mod`, `internal/domain/`, `internal/api/`, `internal/repository/`, `internal/content/`, `internal/plugin/` (architecture-level), `web/admin/src/` (architecture-level), `cmd/lesstruct-cli/` (architecture-level) | `docs/project-context.md` |
| A bounded context, domain-package layer boundary, aggregate, or value object | `docs/ddd/domain-context.md` (+ `docs/ddd/class-diagram.puml` for commerce) |
| A domain term used in code, an API field, or UI copy | `docs/ddd/ubiquitous-language.md` |
| `modules/orders/` (checkout, lifecycle, stock settlement, invoice) | `docs/ddd/orders-bounded-context.md` |
| `internal/config/`, `config.toml`, `.env.example` | `docs/configuration.md` |
| `modules/site-ssr/`, `modules/theme-default/`, `modules/theme-storefront/`, theme CSS/JS contracts | `docs/theme-development.md` AND `skills/lesstruct-theme-development/references/theme-development.md` (user-facing snapshot) |
| `modules/theme-default/pages/*.gohtml` (formatting/whitespace) | `docs/coding-standards/html.md` |
| `internal/plugin/` (SDK/hooks/capabilities), `pkg/sdk/` | `docs/plugin-development.md`, `docs/plugin-capabilities.md` AND the matching snapshots under `skills/lesstruct-plugin-development/references/` |
| `modules/<name>/` (new/changed module layout, seams, gating, wiring) | `docs/module-development.md` AND the matching snapshots under `skills/lesstruct-module-development/references/` |
| `modules/<name>/` (module added/removed, `Requires()` changed, admin manifest entries changed) | `docs/built-in-modules.md` |
| `cmd/lesstruct-cli/` (command, subcommand, or flag surface — not architecture-level wiring) | `docs/cli.md` |
| `internal/api/handlers/agent/`, `internal/api/middleware/apikey.go`, `/api/v1` route shape, response envelope | `docs/api-reference.md` |
| `internal/api/routes/routes.go`, `internal/api/handlers/` (new/removed/renamed endpoints, method changes, query/body param changes, auth-realm changes) | `docs/bruno/` — one `.yml` request per route; add new requests in the matching resource folder and update existing requests' body/query/path/auth in lockstep |
| A user-facing feature added, removed, or behavior-changed (UI, CLI, or config) | `docs/features.md` (canonical catalog) AND, if it is in the homepage's curated subset, the card copy in `site/layouts/landing.html` |
| `site/`, `site/themes/hugo-book/`, `.github/workflows/docs.yml` | the `site/` build (the docs site renders from `docs/` and `skills/*/references/` via Hugo mounts — see "Docs site" below) |

Rules:
- If you cannot tell whether a doc applies, read its first section — each doc states its scope at the top.
- For theme/plugin docs, the `docs/` copy is developer-facing (source-tree paths) and the `skills/.../references/` copy is user-facing (binary install paths). Keep both in sync.
- `docs/features.md` is the canonical feature catalog. The homepage grid (`site/layouts/landing.html`) and `README.md` curate subsets from it — keep the three in sync when feature wording changes.
- `docs/bruno/` is the executable mirror of `internal/api/routes/routes.go` — one request file per HTTP route. When you add or change an endpoint, add or update the matching `.yml` in the same change; if a request references a new variable, also add it to `docs/bruno/environments/Local.example.yml`. Two auth realms coexist: browser (JWT/CSRF, tested in Bruno with `{{TOKEN}}` bearer) and agent (Bearer `{{API_KEY}}`, whose requests live under `Agent (API Key)/`).
- If the change is large enough to need its own commit, the doc update goes in the SAME commit as the code change.

### Docs site

The `site/` directory is the Hugo project that publishes the docs at `lesstruct.dev` (GitHub Pages). It does **not** duplicate the source — `site/hugo.yaml` mounts `docs/*.md` and `skills/<name>/references/*.md` directly, so editing a file in `docs/` is the only edit needed for the rendered site.

Before pushing changes that affect `docs/`, `skills/`, or `site/`, run `make docs-serve` locally to spot-check rendering. CI (`/.github/workflows/docs.yml`) builds and deploys the site on every push to `main` that touches the relevant paths.

Screenshots in docs are rendered with the `screenshot` shortcode (`{{< screenshot src="name" alt="..." caption="..." >}}`), which emits a `<picture>` with light + dark variants. The WebP screenshots live in `site/static/screenshots/` (served at `/screenshots/`), with a `docs/assets/screenshots/` mirror for the GitHub `README.md`. `make screenshots` captures PNGs with Playwright and converts them to WebP (the social `og-card.png` stays PNG). Regenerate them with `make screenshots` against a running instance (see `scripts/screenshots/README.md`). The shortcode fails the docs build if a referenced WebP is missing — so after any admin-UI or default-theme change, re-run `make screenshots` and commit the new files in the same change.

Do not edit files inside `site/themes/hugo-book/` — they are a vendored copy of [`alex-shpak/hugo-book`](https://github.com/alex-shpak/hugo-book) at tag `v12.0.0`. To upgrade the theme, replace the directory contents from a newer release tag.

## Git

- **Use Git Conventional Commits** for every commit: `type: summary` or `type(scope): summary`, where type is one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, or `ci`. The summary is lowercase, imperative, no trailing period. The body (blank line after the summary) explains the what and the why.
- Never add a `Co-Authored-By` trailer — or any AI/assistant attribution — to Git commit messages. Write commit messages plainly, as if authored solely by the user.

## Language

- Always give the output in English.
