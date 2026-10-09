# modules/site-ssr

Official Lesstruct module: the public-site SSR engine — contentpage data
assembly, template resolution, and the HTTP surfaces that render the content
site (plus its auth pages).

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
contentref.go        # ContentRef projection + ContentReader seam (3.5c)
template.go          # template engine, view models, SetEmbeddedFS root
theme.go             # THEME_DIR resolution chain, RootFiles serving
format.go            # localized date formatting helpers
assemble.go          # DataAssembler (3.5d)
handler.go           # content-site + auth-page HTTP handler (3.5d)
user_provider.go     # UserProvider seam + adapter
admin/               # admin-shell manifest (empty: the engine ships no admin UI)
testutil/            # kernel fixture for module tests
```

## Content seam

The engine renders `ContentRef` projections, never content entities: every
field the assembler reads, nothing it does not. Core adapts the content
service in `internal/modules` (see the site adapter). Comments flow through
the comments module's own domain. Any theme satisfies the engine via
`kernel.ThemeProvider` (`Optional: ["theme-default"]`); without one only
`THEME_DIR` files resolve.
