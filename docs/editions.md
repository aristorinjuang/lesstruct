# Editions

> **Scope.** This page is the canonical reference for the Lesstruct editions:
> which binary to install, what each one carries, and how to switch between
> them. For what each module owns, see [built-in-modules.md](built-in-modules.md);
> for how editions are wired in code, see [module-development.md](module-development.md).

Lesstruct ships as **one codebase, three binaries**. An edition is a fixed
module list compiled into one binary
([composition-root wiring](module-development.md#composition-root-wiring)).
All editions share the same `config.toml` schema and the same data directory,
so moving between editions is installing the other binary — no migration, no
config rewrite.

## The three editions

### Blog — the default CMS

The `lesstruct` binary. Every first-party module except the commerce stack:
identity (`users`, `auth`), the comment system, the admin panel, content
engine with custom post types, media library, SEO surfaces, the server-rendered
content site (`site-ssr`) with the embedded default blog theme
(`theme-default`), AI text generation (`textgen`), and the WordPress/Hugo
importers — plus the WASM plugin system every edition carries.

Choose this if you run a content site — blog, magazine, company site,
documentation — with the admin panel, the SSR frontend, and the static-site
exporter.

### Core — the headless engine

The `lesstruct-core` binary. The engine subset only: `users`, `auth`,
`adminpanel`, `posttypes-basic`, and `media`, plus the content engine and the
plugin system. No comment system, no SEO surfaces, no SSR theme engine, no
embedded theme, no AI text generation, no importers.

Choose this if you render your own frontend against `/api/v1` — a custom
storefront, a mobile app, a decoupled site — and want the smallest binary with
no public HTML surface to maintain.

### eCommerce — the blog platform plus commerce

The `lesstruct-ecommerce` binary. The full blog module list plus the commerce
stack — `catalog` (pricing, variant matrices, stock, SKUs), `cart`
(token-keyed guest carts), `orders` (checkout, lifecycle, expiry sweeper,
invoice PDFs), and `payments` (transfer-proof verification, Midtrans Snap
driver) — with `theme-storefront` in place of `theme-default`: product pages,
cart, checkout, and order tracking out of the box.

Choose this if you sell online. The demo `config.toml` works unchanged; the
commerce blocks (`[catalog]`, `[orders]`, `[payments]`, `[invoice]`) are
documented in [configuration.md](configuration.md).

## Comparison matrix

| Module | Blog (`lesstruct`) | Core (`lesstruct-core`) | eCommerce (`lesstruct-ecommerce`) |
|---|---|---|---|
| `users` | ✓ | ✓ | ✓ |
| `auth` | ✓ | ✓ | ✓ |
| `comments` | ✓ | — | ✓ |
| `adminpanel` | ✓ | ✓ | ✓ |
| `posttypes-basic` | ✓ | ✓ | ✓ |
| `media` | ✓ | ✓ | ✓ |
| `seo` | ✓ | — | ✓ |
| `site-ssr` | ✓ | — | ✓ |
| `theme-default` | ✓ | — | — |
| `theme-storefront` | — | — | ✓ |
| `textgen` | ✓ | — | ✓ |
| `importers` | ✓ | — | ✓ |
| `catalog` | — | — | ✓ |
| `cart` | — | — | ✓ |
| `orders` | — | — | ✓ |
| `payments` | — | — | ✓ |
| WASM plugin system | ✓ | ✓ | ✓ |

| | Blog | Core | eCommerce |
|---|---|---|---|
| Binary | `lesstruct` | `lesstruct-core` | `lesstruct-ecommerce` |
| Embedded theme | Default blog theme | None (headless) | Storefront theme |
| Public HTML site | Yes (SSR + SSG export) | No (`/api/v1` only) | Yes (storefront + SSG export) |
| Install | `go install github.com/aristorinjuang/lesstruct@latest` | `go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-core@latest` | `go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-ecommerce@latest` |

Module dependencies still apply inside every edition: disabling a module drops
everything that transitively requires it (so disabling `users` cascades away
`auth`, `comments`, and `adminpanel`; disabling `catalog` on eCommerce
cascades away `cart`, `orders`, and `payments`). The full cascade rules are in
[built-in-modules.md](built-in-modules.md) and
[configuration.md](configuration.md#modules).

## Installing

Release binaries ship for the blog edition (`lesstruct-<os>-<arch>`) and the
Core edition (`lesstruct-core-<os>-<arch>`) on `linux`, `darwin`, and `windows`
(`amd64`, `arm64`) from the
[releases page](https://github.com/aristorinjuang/lesstruct/releases).
The eCommerce edition is installed via `go install` or built from source —
no release asset is published for it yet:

```bash
# The default (blog) edition — every module except commerce
go install github.com/aristorinjuang/lesstruct@latest

# The Core edition — the headless engine
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-core@latest

# The eCommerce edition — blog plus the commerce stack
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-ecommerce@latest
```

From source:

```bash
make install          # blog edition + admin panel + CLI into $GOBIN
make build-core       # the Core edition
make build-ecommerce  # the eCommerce edition
make build-server     # blog edition without the embedded SQLite driver
                      # (~6 MB leaner, for PostgreSQL/MySQL deployments)
```

## Switching editions

Editions share one data directory and one `config.toml`. To switch, stop the
server, install the other binary, and start it against the same directory.
Within any edition, `[modules] disabled` still prunes further — e.g. a blog
install with `"comments"` disabled approximates Core-with-SSR, and an
eCommerce install with the commerce modules disabled behaves like the blog
edition with the storefront theme.
