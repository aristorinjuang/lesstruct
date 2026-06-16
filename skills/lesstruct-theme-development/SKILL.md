---
name: lesstruct-theme-development
description: Develop a custom theme for a Lesstruct installation. Overrides the default CSS, JavaScript, and HTML templates served at the public site. Walks through the override mechanics, the layout/body block contract, CSS custom properties, the JS DOM contract, and the CDN assets the default layout pulls in. Use when the user asks to build a theme, customise the public site, override CSS, or override templates in a Lesstruct installation.
---

# Lesstruct Theme Development

## Overview

This skill guides a user (and their AI agent) through building, modifying, or
repairing a custom theme for a Lesstruct installation. The skill works entirely
from the user's `themes/<name>/` directory and the running public site; it does
not require access to the Lesstruct source tree.

**Output:** none mandatory. The skill may produce files inside the user's
`themes/<name>/` directory (CSS, JS, HTML templates) and optionally a
`CHANGELOG.md` recording the Lesstruct version the theme was authored against.

## When to Use

Trigger this skill when the user asks to:

- Build a new theme for their Lesstruct installation.
- Re-skin the public site (change colours, fonts, layout).
- Override an HTML template, a JavaScript file, or the stylesheet.
- Audit or repair a theme that broke after a Lesstruct upgrade.
- Diagnose why a theme override is not taking effect.

## Activation

1. Load `references/theme-development.md` (the user-facing snapshot of the
   theme contract). Treat it as the authoritative reference for the rest of
   the workflow.
2. Confirm the user's intent: **new theme**, **modify existing**, **audit**,
   or **repair after upgrade**. Each path follows the same workflow but
   changes the entry point.
3. Greet the user and state the chosen intent in one sentence before
   proceeding.

## Procedure

### Step 1: Locate the theme directory

Ask the user for the absolute path to their `themes/<name>/` directory.

If they do not have one, give them the boilerplate and **wait**:

```bash
mkdir -p themes/mytheme/{static,templates}
```

…then add `THEME_DIR=themes/mytheme` to their `.env` (or the environment
their Lesstruct process runs in) and restart Lesstruct.

**Never invent a path.** If the user is unsure, ask them to run
`ls themes/` (or the equivalent on their install) and paste the result.

### Step 2: Inventory existing files

Read the contents of `themes/<name>/` (if it exists). For each file present,
note which embedded default it overrides. Cross-reference against the
[Static File Overrides](references/theme-development.md#static-file-overrides)
and [Template Data Fields](references/theme-development.md#template-data-fields)
sections of the loaded reference.

The result of this step is a short list:

- Files that exist and what they override.
- Files that fall back to embedded defaults (and which default).

### Step 3: Plan changes

Depending on the user's intent, identify the smallest set of files to create
or edit:

- **Re-skin (CSS only)** — `themes/<name>/static/style.css` only.
- **Restructure layout** — `themes/<name>/templates/layout.html`, possibly
  with matching JS overrides if DOM contracts change.
- **Rebuild a single page** — the corresponding `templates/<page>.html` only.
- **Repair after upgrade** — diff the new embedded defaults (the
  [Reference URLs](references/theme-development.md#reference-urls) section
  lists where to fetch them) against the user's existing override.

Flag any conflict up front:

- Brand tokens that the user is overriding in CSS.
- CDN assets the default layout pulls in that the user is not loading.
- JS files the user is overriding that have DOM contracts with the templates.

### Step 4: Fetch the default CSS (when CSS work is involved)

If the work involves `style.css`, give the user two options:

- Use the bundled `references/default-style-reference.css` as the starting
  point (offline, readable, section-organised).
- Fetch the live default: `curl -s http://localhost:8080/static/style.css > /tmp/default.css`
  and start from that (matches what browsers actually receive).

Note that `references/default-style-reference.css` is a verbatim copy of the
upstream readable source, **not** the minified file browsers receive. Theme
authors do not need to minify their override; Lesstruct serves it verbatim.

### Step 5: Implement

For each file to create or edit:

- **CSS** — preserve the custom-property contract (every variable the default
  uses should still be defined; new ones are fine). Override only the values
  the user wants to change.
- **Templates** — preserve the `{{define "layout"}}` / `{{define "body"}}`
  block contract. `layout.html` must call `{{template "body" .}}` inside
  `<main>`. Every other template defines `body` only.
- **JavaScript** — preserve the DOM contract (ids, classes, data-attributes)
  that the default templates expect. If the user's new layout uses different
  markup, update the JS to match.
- **CDN assets** — if the user is dropping katex and/or highlight.js, the
  layout must drop the matching `<link>` / `<script>` tags and the matching
  static files (`math.js`, `highlight.min.js`).

After each file change, remind the user that **a server restart is required**
to pick up the change — `THEME_DIR` and theme files are read at startup.

### Step 6: Verify (manual)

Hand the user the smoke test plan from
[references/page-render-smoke-test.md](references/page-render-smoke-test.md).
It lists 10 page URLs and what to look for on each. The skill cannot run Go
tests because it has no access to the Lesstruct source; the user must verify
in a browser or with `curl` against their running install.

If the user reports a failure, the most common causes are in
[Troubleshooting](references/theme-development.md#troubleshooting) of the
loaded reference.

### Step 7: Document and hand off

Recommend the user create or update `themes/<name>/CHANGELOG.md` recording:

- The Lesstruct version the theme was authored against.
- The files the theme overrides.
- Any non-default CDN or JS choices the theme makes.

When the user upgrades Lesstruct in the future, they re-run this skill with
intent **repair after upgrade**. The workflow then:

1. Asks the user to fetch the new default CSS (`curl`).
2. Diffs the new default against the user's override.
3. Asks the user to inspect the embedded JavaScript of the new release (the
   [Reference URLs](references/theme-development.md#reference-urls) section
   in the loaded reference lists which endpoints changed) for new DOM
   contracts.
4. Surfaces any structural changes from the loaded reference's
   [Template Data Fields](references/theme-development.md#template-data-fields)
   section.

### Step 8: Optional — pre-flight audit

If the user asks for an audit of an existing theme, run the checklist from
[references/theme-audit-checklist.md](references/theme-audit-checklist.md) and
report each item as pass / fail / not applicable.

## Out of scope

- The admin panel (Vue SPA). Themes do not affect it; rebrand by editing
  `web/admin/` source and rebuilding.
- API responses (`/api/*`). Themes do not affect them.
- Plugins, hooks, and capabilities. Themes do not affect them.
- Email templates and other server-rendered channels.

## Reference Index

- `references/theme-development.md` — user-facing snapshot of the theme
  contract (CSS variables, template data fields, JS DOM contract, CDN
  assets, troubleshooting, reference URLs).
- `references/default-style-reference.css` — verbatim copy of the readable
  default stylesheet, ready to use as a CSS-only starting point.
- `references/theme-audit-checklist.md` — pre-flight checklist for
  auditing an existing theme.
- `references/page-render-smoke-test.md` — manual page-by-page smoke test
  plan.
- `references/install-paths.md` — per-agent install snippets.
