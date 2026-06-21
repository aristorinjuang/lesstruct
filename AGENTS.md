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

- Put all private structs or functions before all public structs or functions!
- Do not ever use `interface{}`, use `any`!
- Always use constants instead of typed strings, especially to define HTTP methods! E.g., `http.MethodDelete` instead of `"DELETE"`.

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

## Structs & Constructors

- Treat `struct` like objects if they have function receivers (methods)! So, put constructors (functions that start with `New` in general) after all methods. Put function receivers (methods) right after their `struct`.

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
- Ensure your works pass `make lint`, `make test`, and `make vulncheck`!

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
| `internal/config/`, `config.toml`, `.env.example` | `docs/configuration.md` |
| `internal/api/template/`, `internal/api/contentpage/`, theme CSS/JS contracts | `docs/theme-development.md` AND `skills/lesstruct-theme-development/references/theme-development.md` (user-facing snapshot) |
| `internal/plugin/` (SDK/hooks/capabilities), `pkg/sdk/` | `docs/plugin-development.md`, `docs/plugin-capabilities.md` AND the matching snapshots under `skills/lesstruct-plugin-development/references/` |
| `internal/api/handlers/agent/`, `internal/api/middleware/apikey.go`, `/api/v1` route shape, response envelope | `docs/api-reference.md` |
| `site/`, `site/themes/hugo-book/`, `.github/workflows/docs.yml` | the `site/` build (the docs site renders from `docs/` and `skills/*/references/` via Hugo mounts — see "Docs site" below) |

Rules:
- If you cannot tell whether a doc applies, read its first section — each doc states its scope at the top.
- For theme/plugin docs, the `docs/` copy is developer-facing (source-tree paths) and the `skills/.../references/` copy is user-facing (binary install paths). Keep both in sync.
- If the change is large enough to need its own commit, the doc update goes in the SAME commit as the code change.

### Docs site

The `site/` directory is the Hugo project that publishes the docs at `lesstruct.dev` (GitHub Pages). It does **not** duplicate the source — `site/hugo.yaml` mounts `docs/*.md` and `skills/<name>/references/*.md` directly, so editing a file in `docs/` is the only edit needed for the rendered site.

Before pushing changes that affect `docs/`, `skills/`, or `site/`, run `make docs-serve` locally to spot-check rendering. CI (`/.github/workflows/docs.yml`) builds and deploys the site on every push to `main` that touches the relevant paths.

Do not edit files inside `site/themes/hugo-book/` — they are a vendored copy of [`alex-shpak/hugo-book`](https://github.com/alex-shpak/hugo-book) at tag `v12.0.0`. To upgrade the theme, replace the directory contents from a newer release tag.

## Git

- Never add a `Co-Authored-By` trailer — or any AI/assistant attribution — to Git commit messages. Write commit messages plainly, as if authored solely by the user.

## Language

- Always give the output in English.
