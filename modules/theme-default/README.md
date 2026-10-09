# modules/theme-default

Official Lesstruct module: the default theme artifact — embedded layouts,
static assets, and template funcs consumed by the site-ssr engine.

## Layout (module template)

```
module.go            # kernel.Module + ThemeProvider (single registry-facing file)
files.go             # embedded pages/ + static/ artifact + Files() accessor
pages/               # embedded *.gohtml layouts (moved verbatim)
static/              # embedded CSS/JS assets (moved verbatim)
admin/               # admin-shell manifest (empty: themes ship no admin UI)
testutil/            # kernel fixture for module tests
```

## Files

The artifact layout mirrors the historical `internal/api/template` tree so
every `pages/<name>.gohtml` and `static/<name>` lookup keeps working
unchanged: core installs the root filesystem once at startup and resolves
`THEME_DIR` overrides over it. Third-party themes ship as new
`ThemeProvider` modules or as plain `THEME_DIR` directories.
