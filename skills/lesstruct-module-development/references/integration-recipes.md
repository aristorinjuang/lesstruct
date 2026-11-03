# Integration Recipes — Custom-Edition Default

> The composition root (`internal/app/app.go` `Run`) is shared platform:
> config, database, identity, content engine, media stack, plugin system,
> boot. An **edition** is a fixed module list compiled into one binary
> (blog, core, commerce live in `internal/editions/<name>/` as separate
> packages so a binary links only its modules). Platform code never
> imports a theme or the engine directly — the edition owns that.

## Platform vs edition — where does the wiring go?

| Change | Goes in |
|---|---|
| Business rule, domain type, sentinel, repository port | `modules/<name>/domain/` |
| HTTP handler, seam interface | `modules/<name>/api/` (+ `agent/` for the Bearer realm) |
| SQL adapter | `modules/<name>/store/` (one impl over `*sql.DB + kernel.Dialect`) |
| External provider client | `modules/<name>/infrastructure/` (domain port) or `modules/<name>/<driver>/` (self-contained protocol driver, e.g. `payments/midtrans`); never in `domain/` |
| Cross-context adapter (module A needs module B's service) | `internal/editions/<name>/` — the **only** layer that may import both sides. One-seam-per-file, named for the module fed (`cart.go`, `orders.go`); `wire<Module>` steps stay in the edition's main file, never mixed into adapter files |
| Service construction, module `New` + candidates, server boot | `internal/app` (`Run`) + the edition's `Wire`/`Mount` hooks |
| Private module wiring | A **new custom edition** (`internal/editions/<custom>/` + `cmd/<custom>/main.go`). Never `internal/app` by default |

First-party upstreaming (editing `internal/app`, `SUBMODULES`, the
release EDGE map, `docs/built-in-modules.md`) is maintainer-gated and
out of scope for private modules.

## Recipe: new private module + custom edition

Assumes a clean checkout at a known tag (SKILL.md Step 2). Module key
`mymod`, edition `mycustom`.

### 1. Scaffold the module

Follow `scaffold/INSTRUCTIONS.txt`: copy the `core` tier + the overlays
the design needs into `modules/mymod/`. Resolve `__MODULE_NAME__` and
`__KERNEL_VERSION__` (derived from the checkout's pinned sibling
require, e.g. `awk '/lesstruct\/kernel / {print $2}' modules/cart/go.mod`).
No `replace` directives in `modules/mymod/go.mod`.

### 2. Register the workspace

```bash
# go.work — add the new module to the `use` block:
#   use (
#     .
#     ./kernel
#     ...
#     ./modules/mymod
#   )
# Never run a bare `go work sync` — it strips the intra-workspace
# requires the release process maintains (see
# references/troubleshooting.md).

# Makefile — add to SUBMODULES so gates cover it:
#   SUBMODULES := kernel modules/posttypes-basic ... modules/mymod
```

Private modules stay out of the `release` EDGE map and
`docs/built-in-modules.md` (first-party surfaces only).

### 3. Write the custom edition

Create `internal/editions/mycustom/mycustom.go`:

```go
// Package mycustom is the private custom edition: platform plus mymod.
package mycustom

import (
    "github.com/aristorinjuang/lesstruct/internal/app"
    "github.com/aristorinjuang/lesstruct/internal/modules"
    mymod "github.com/aristorinjuang/lesstruct/modules/mymod"
)

type state struct {
    mymod *mymod.Module
}

func (s *state) wire(core *app.Core) error {
    // Build the module's collaborators from core services, then New it.
    s.mymod = mymod.New(
        // mymod.WithService(...),
        mymod.WithLogger(modules.NewLogger(core.Logger)),
    )
    core.Candidates = append(core.Candidates, s.mymod)
    // Fill optional handler surfaces the edition owns:
    // hs := core.Handlers(); hs.X = s.mymod.Handler()
    return nil
}

func (s *state) mount(core *app.Core) error {
    // Post-InitAll assembly (templates, readers, generators). Most private
    // modules need nothing here — return nil.
    return nil
}

// Edition returns the custom edition.
func Edition() app.Edition {
    s := &state{}
    return app.Edition{
        Name:     "mycustom",
        Comments: true, // or false when the edition drops the comment system
        Wire:     s.wire,
        Mount:    s.mount,
    }
}
```

Theme swap (when the edition needs a different embedded theme instead
of the default): follow `blog.EditionWithTheme` — pass the alternate
`kernel.ThemeProvider` into the constructor rather than registering two
providers (first-provider-wins).

Cross-context seams (module A needs module B's service) become
small adapters in `internal/editions/mycustom/` (e.g. `mymod.go`):
fields + seam methods in the interface's declaration order + the `New*`
constructor. `wire` steps never live in adapter files.

### 4. Add the thin main

Create `cmd/mycustom/main.go` (mirrors `main.go` /
`cmd/lesstruct-core/main.go` — stays thin, no wiring):

```go
// Command mycustom is the private custom edition binary.
package main

import (
    "fmt"
    "os"

    "github.com/aristorinjuang/lesstruct/internal/app"
    mycustom "github.com/aristorinjuang/lesstruct/internal/editions/mycustom"
)

var version = "dev"

func main() {
    if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
        fmt.Println(version)
        return
    }
    app.Run(mycustom.Edition())
}
```

### 5. Wire order reference (what `Run` does with your edition)

1. Platform services built (config, DB, identity, content, media…).
2. `edition.Wire(core)` — your modules appended to `core.Candidates`
   (platform modules register around it; order only pins the init log
   and migration sequence — the registry sorts topologically anyway).
3. `ApplyDisabled` prunes `[modules] disabled` + cascade (warns); only
   kept modules register.
4. `InitAll` → capability discovery (`Capabilities[T]`) → first
   `ThemeProvider` installs → module migrations run → `edition.Mount`.

### 6. Verify both workspaces

```bash
go test ./modules/mymod/...
GOWORK=off go test ./modules/mymod/...   # proves go.mod resolves without the workspace
go build ./...                            # whole workspace still builds
go build -tags nosqlite ./...            # server-edition compile check
go build ./cmd/mycustom                   # custom binary links
```

`GOWORK=off` failing while in-workspace passes means a missing
`require` or a stray `replace` — fix `go.mod`, never add a filesystem
`replace` (it breaks `go install pkg@version`; local checkouts resolve
through `go.work`).

### 7. Serve + cascade check

```bash
go run ./cmd/mycustom                     # boot, note the "Modules initialized: …" line
# exercise the new surface (curl / browser / agent API as designed)
```

Then confirm the cascade: `[modules] disabled = ["mymod"]` prunes the
module (plus transitive dependents) with a warning, and its routes
unmount where flag-gated. A kept module whose hard dep is absent still
fails startup fast — that path is unchanged.
