# AI Theme Generation

> **Scope.** This page is the how-to for generating themes: the admin panel
> flow, the CLI, and the MCP tools. Endpoint semantics live in
> [api-reference.md](api-reference.md); theme internals (CSS tokens,
> template contract, static assets) live in
> [theme-development.md](theme-development.md).

Describe a theme in plain language; Lesstruct builds it, validates it, and
installs it. Generation grounds every theme on your edition's real sources,
so the result already speaks your site's design language.

## Requirements

- An AI key: `AI_THEMGEN_API_KEY` (or the shared `AI_TEXT_GENERATION_API_KEY`
  fallback) plus model/base-URL settings — see
  [configuration.md](configuration.md). Without a key the generator answers
  `503 generation_unavailable`.
- An **admin** account. Since v1.5 every theme endpoint requires admin on
  both the browser panel and the API — a non-admin key or session gets
  `403`.
- `/api/health` reports `"themeGeneration": true` when the feature is live.

## Generate in the admin panel

Open **Theme Generator** (`/admin/theme-generator`) and write a brief —
"a dark editorial theme with serif headings" is plenty. Pick a style preset
or leave `auto`; tick **Generate page templates** to also generate
`templates/*.html` overrides (stylesheet-only is the default). Generation
can take minutes: the request budget is 130 seconds, 190 with templates.

Every result passes the same gate the engine itself uses: file contract and
CSS parse always, plus template parse and smoke renders of every page with
sample data when templates are present. Generated output additionally passes
a content scan — no script elements, inline handlers, `javascript:` URLs, or
`root/` files; manual themes skip this scan. A failing theme goes back to
the model with its findings (at most two repair attempts); a still-failing
theme is refused and the panel toasts the reason, e.g.
`Generation failed: static/style.css has unbalanced braces`.

A generated theme auto-installs as **inactive**. Its card shows a snapshot
of the rendered homepage; download the `.zip` for a manual `THEME_DIR`
install, preview it, or activate it in place.

## Activate, deactivate, delete

`/admin/themes` lists every installed theme. **Activate** validates the
theme, writes `[theme] active`, and hot-applies it — no restart; a failed
reload reverts the config write. **Deactivate** falls back to the embedded
theme. The active theme and the live `THEME_DIR` target cannot be deleted
(the target lists as active instead). Resolution precedence: `THEME_DIR`
wins, then the activated theme, otherwise the embedded default.

## Preview and snapshots

**Preview** opens the installed theme on the real content site for your
browser only: a signed cookie (valid 15 minutes, `HttpOnly`) dispatches you
to a cached preview stack while visitors keep the active theme. Stop it from
the toolbar or wait it out; deleting the previewed theme stops it too.
Preview needs the production admin (same origin as the site); with the dev
server open `/` on the Go server instead.

**Snapshots** need no action: every theme card — and the just-generated
theme — frames a sandboxed, scaled snapshot of that theme's homepage,
rendered server-side with sample content. An unknown or broken theme shows
a placeholder instead of breaking the listing.

## From the terminal

The same surface is available to agents and scripts via `lesstruct-cli
theme` (admin API key):

```bash
lesstruct-cli theme generate --description "A dark editorial theme" --style dark
lesstruct-cli theme generate --description "Magazine layout" --template-overrides
lesstruct-cli theme list
lesstruct-cli theme download midnight-serif --output-dir /path/to/themes
lesstruct-cli theme activate midnight-serif
lesstruct-cli theme deactivate
lesstruct-cli theme delete paper-light
```

Generation failures print the first error finding; `--output json` emits the
raw payload including the full validation report and file map. See
[cli.md](cli.md) for flags and exit codes.

## From MCP

Five tools, all admin only: `theme_generate` (returns name, rationale,
validation, and file paths — contents stay out of the payload so hosts do
not truncate them), `theme_list`, `theme_activate`, `theme_deactivate`, and
`theme_delete` (requires `confirm: true`). Gate failures surface the first
error finding as the tool error. Preview and download stay out — preview is
a per-browser cookie, downloads are binary. See [mcp.md](mcp.md) for the
tool catalog.

## Troubleshooting

- `503 generation_unavailable` — no AI key configured; check
  [configuration.md](configuration.md).
- `429 generation_busy` — two generations already running server-wide; wait
  and retry.
- `500 generation_failed` — the gate still failed after two repairs; the
  response `details` (CLI `--output json`, MCP error text) carry the final
  findings.
- `403` — non-admin account or key; theme endpoints require admin.
- Snapshot frames show the login page — you are on the dev admin server
  (different origin); open the site on the Go server instead.

## See also

- [api-reference.md](api-reference.md) — the `/api/v1/themegen` contract.
- [cli.md](cli.md) — the `theme` command group.
- [mcp.md](mcp.md) — the `theme_*` tools.
- [theme-development.md](theme-development.md) — building and overriding
  themes by hand.
