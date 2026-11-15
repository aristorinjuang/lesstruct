# themegen

AI theme generator: validated themes (`static/base.css` + `static/style.css`, plus opt-in `templates/*.html` overrides) from a natural-language brief, powered by an OpenAI-compatible API. Generated themes pass the same file/CSS/template-render gate the engine uses at boot and activation, plus a generated-content scan (no scripts, handlers, `javascript:` URLs, or `root/` files — manual themes stay trusted); failures feed a bounded two-attempt repair loop over the original readable output before the theme is refused with its reason, and CSS is minified at install time.

- `domain/` — brief/theme types, validation (slug grammar, allowlist, size caps), envelope parser, orchestration service with the repair loop, validator.
- `infrastructure/` — OpenAI-compatible client + per-call grounded prompts (edition stylesheet and template sources + distilled contracts, including the style preset on repairs).
- `store/` — filesystem store under `THEMES_DIR` with `theme.json` manifests and stdlib zips.
- `api/` — browser handler: generate, list, download, activate, deactivate, delete, preview.
- `admin/` — admin-shell manifest (Theme Generator + Themes nav).

Activation writes `[theme] active`, validates the full theme (CSS plus template parse and smoke renders), and hot-applies it through the edition's reloader; a failed reload reverts the config write. Precedence: `THEME_DIR` > preview cookie > `[theme] active` > embedded. Startup never bricks: a broken active theme falls back to embedded with `themeValidationFailed` on `/api/health`.
