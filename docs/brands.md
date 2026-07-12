---
title: "Brand Assets"
weight: 60
---

# Brand Assets

> **Scope.** This page is the canonical reference for the Lesstruct brand:
> the logo, the color palette, the type stack, and how they may be used.
> The colors are also the locked `--color-*` / `--brand-*` CSS custom
> properties in the default content-site theme
> ([theme-development.md](theme-development.md)) and the admin panel.

Lesstruct is MIT-licensed, and so are these assets. Use them freely; attribution
is appreciated but not required.

## The mark

The Lesstruct logo is a stylised, rounded **"L"** — a single mark with no
wordmark. It carries a cyan → blue → violet gradient that maps to the three
technology pillars the project is built on:

| Gradient stop | Color     | Pillar        |
|---|---|---|
| Top (cyan)    | `#22d3ee` | Go (primary)   |
| Middle (blue) | `#2563eb` | TypeScript (secondary) |
| Base (violet) | `#8b5cf6` | WebAssembly (accent) |

The mark reads on both light and dark backgrounds; it has a transparent
background and should always be placed on a solid or subtle surface — never on
busy imagery.

<img src="/brand/lesstruct.webp" width="160" height="160" alt="The Lesstruct logo">

## Logo variants

Only one mark exists. There is no separate "dark" or "mono" variant — the
gradient mark is used as-is on both light and dark surfaces.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/brand/lesstruct.webp">
  <img src="/brand/lesstruct.webp" width="96" height="96" alt="Lesstruct logo on a light surface">
</picture>

## Clear space & minimum size

- **Clear space.** Keep a margin around the mark equal to at least **25% of its
  height** free of other elements (text, UI chrome, imagery edges).
- **Minimum size.** Do not render the mark smaller than **24×24 px** in any
  interface. The favicon set (16–512 px) is the exception; it is pre-rasterised
  for those sizes.

## Color palette

The three brand colors are **locked**. Their values must never change; see the
"Brand colors (locked)" comment in `internal/api/template/static/style.src.css`
and `web/admin/src/assets/base.css`. Everything else (status colors, radii,
shadows) is derived design-system layering.

| Role | Token | Hex | RGB | Used for |
|---|---|---|---|---|
| Primary    | `--color-primary` / `--brand-primary`    | `#22d3ee` | `rgb(34, 211, 238)` | Go. Primary actions, links, focus rings, header accent. |
| Secondary  | `--color-secondary` / `--brand-secondary` | `#2563eb` | `rgb(37, 99, 235)` | TypeScript. Logo, headings, active nav, badges, secondary actions. |
| Accent     | `--color-accent` / `--brand-accent`     | `#8b5cf6` | `rgb(139, 92, 246)` | WebAssembly. Tags, accent highlights. |

Hover and light (alpha) variants are derived from these in the theme CSS:
primary hover `#06b6d4`, primary-light `rgba(34, 211, 238, 0.1)`,
secondary-light `rgba(37, 99, 235, 0.1)`, accent hover `#7c3aed`,
accent-light `rgba(139, 92, 246, 0.1)`.

### Neutrals

| Token | Hex | Used for |
|---|---|---|
| `--color-bg`        | `#ffffff` | Page background |
| `--color-text`      | `#1a1a2e` | Body text |
| `--color-text-muted`| `#6b7280` | Secondary text |
| `--color-border`    | `#e5e7eb` | Borders, dividers |
| `--color-card-bg`   | `#f9fafb` | Card surfaces |

### Status colors

| Token | Hex | Meaning |
|---|---|---|
| `--color-danger`  | `#dc2626` | Errors, destructive |
| `--color-success` | `#16a34a` | Success states |

## Typography

- **Primary typeface — Inter.** Loaded from Google Fonts
  (`family=Inter:wght@400;600;700`). Used by both the content site and the
  admin panel.
- **Monospace — JetBrains Mono**, falling back to Fira Code / Cascadia Code.
  Used for inline code and code blocks.
- **System fallback stack** — `-apple-system, BlinkMacSystemFont, "Segoe UI",
  Roboto, "Helvetica Neue", Arial, sans-serif` when Inter cannot be loaded.

The docs site (`lesstruct.dev`) renders via the Hugo Book theme and uses that
theme's font stack; Inter is the brand typeface referenced by the product UIs.

## Downloads

The logo ships in two formats, both 1024×1024 with a transparent background:

- [PNG](/brand/lesstruct.png) — `lesstruct.png` (1024×1024, RGBA)
- [WebP](/brand/lesstruct.webp) — `lesstruct.webp` (lossless)

In a source-tree checkout, both live under `docs/assets/brand/` (GitHub-facing
mirror) and `site/static/brand/` (served at `/brand/` on the docs site).

## Usage policy

**Do**
- Use the mark on solid light or dark backgrounds.
- Preserve the gradient and the rounded-"L" proportions.
- Keep clear space around the mark.

**Don't**
- Don't stretch, squash, or rotate the mark.
- Don't recolor the gradient or flatten it to a single color.
- Don't place the mark on busy imagery or low-contrast surfaces.
- Don't change the locked brand color values in code — they are part of the
  identity.

## Where the brand lives in code

| Surface | File |
|---|---|
| Content-site theme tokens (locked) | `internal/api/template/static/style.src.css` |
| Admin panel brand tokens | `web/admin/src/assets/base.css` |
| Favicon set | `site/static/favicon.*`, `web/admin/public/favicon.*` |
| Docs site logo (sidebar) | `BookLogo` in `site/hugo.yaml` → `/brand/lesstruct.webp` |
| README logo | `docs/assets/brand/lesstruct.webp` |
