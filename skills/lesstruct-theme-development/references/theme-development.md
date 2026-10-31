# Theme Development Guide

> **Snapshot notice.** This is the **user-facing** snapshot of the Lesstruct
> theme development guide, bundled with the `lesstruct-theme-development` skill
> so the skill is self-contained. The canonical, developer-facing version lives
> in the Lesstruct repository at `docs/theme-development.md` and references
> source-tree paths; this version is rewritten for someone with a binary
> Lesstruct install and a `themes/<name>/` directory.
>
> If you have a newer Lesstruct release, re-export this file from the repo to
> refresh the snapshot.

Lesstruct supports custom themes for the public-facing content site. Themes override the
default CSS, JavaScript, and (optionally) HTML templates without modifying the core source.

## How Themes Work

1. Create a theme directory with your custom files.
2. Set the `THEME_DIR` environment variable to point to it.
3. At startup, Lesstruct resolves each template and static file through a
   composite filesystem: if the file exists under `THEME_DIR`, that copy is
   used; otherwise the embedded default is used.

The embedded default is the theme of the edition you run: the blog edition
ships the default blog theme, and the eCommerce edition ships the storefront
theme (see [Standalone Pages](#standalone-pages)). Your `THEME_DIR` overrides
either one the same way.

The storefront theme is commerce-focused: content posts still render the
comment section and load `comments.js` when comments are enabled, but the
theme does not ship the public search box, KaTeX math rendering, or
highlight.js syntax highlighting. Those remain features of the default blog
theme; a child theme can add them back through `THEME_DIR` overrides.

This means you can ship a **partial** theme — a single `style.css`, `base.css` + `style.css`, or a full
`layout.html`, or anything in between — and the rest stays on the embedded
defaults.

## Theme Directory Structure

```
themes/
  mytheme/
    root/            # Optional; copied to the site ROOT (see "Root-Level Files")
      webpushr-sw.js
    static/          # Served at /static/*
      base.css
      style.css
      auth.js
      comments.js
      nav-auth.js
      search.js
      math.js
      verify-email.js
      reset-password.js
      highlight.min.js
      favicon.svg
      favicon.ico
      favicon-96x96.png
      apple-touch-icon.png
    templates/       # Go html/template files
      layout.html
      homepage.html
      index.html
      post.html
      author.html
      tag.html
      not_found.html
      login.html
      register.html
      forgot_password.html
      verify_email.html
      reset_password.html
      cart.html            # Only when the active theme declares these pages
      checkout.html        # (the eCommerce storefront theme does; see
      orders-track.html    # "Standalone Pages")
```

The theme can override **any subset** of these files. Any file not present
falls back to the embedded default. The embedded defaults are not visible
files on your install — they live inside the Lesstruct binary — but you can
inspect the served versions at the URLs listed below in
[Reference URLs](#reference-urls). You can also add per-post-type templates
(e.g. `page.html`, `event.html`) — see
[Per-post-type templates](#per-post-type-templates).

## Root-Level Files

Some files must live at the **site root** to work: push service workers (the
Webpushr SDK registers `/webpushr-sw.js`), `manifest.json` for some PWA setups,
or `.well-known/` verification files. Place them in your theme's optional
`root/` directory:

```
themes/
  mytheme/
    root/
      webpushr-sw.js     # → https://example.com/webpushr-sw.js (both surfaces)
```

- **Dynamic server**: a request is answered from `root/` only when it names an
  existing regular file there; every other path falls through to alias
  redirects and content pages, so `root/` files never shadow posts or pages.
- **Static export**: the whole `root/` tree lands at the archive root (not
  under `/static/`).

`static/` remains the right home for everything that can live under `/static/*`;
use `root/` only when an external system demands a fixed root URL.

## Quick Start: CSS-Only Theme

The simplest theme overrides only the CSS.

### 1. Create the theme directory

```bash
mkdir -p themes/mytheme/static
```

### 2. Start from the readable source

The file browsers receive is `/static/base.css` and `/static/style.css` (the
minified defaults). The readable, documented sources are bundled with this skill
at `references/default-base-reference.css` and
`references/default-style-reference.css` — copy them as your starting point:

```bash
cp references/default-base-reference.css themes/mytheme/static/base.css
cp references/default-style-reference.css themes/mytheme/static/style.css
```

You do not need to minify your override; Lesstruct serves your file verbatim.

> **Your theme CSS doubles as the AI's brand brief.** When a user generates
> HTML/CSS with AI, the active theme's `base.css` and `style.css` are injected
> into the generation prompt. The AI reuses your design tokens
> (`var(--color-primary)`, `var(--space-*)`, etc.) and component classes
> (`.btn`, `.form-control`) instead of inventing off-brand styles. Keep your
> tokens well-named and your component classes semantic — the AI follows your
> lead.
(If you maintain a `.src.css` for your own authoring convenience and want to
ship a minified version, use any CSS minifier — but the theme override is happy
with any valid CSS.)

### 3. Override the design tokens

The default theme exposes every visual decision as a CSS custom property under
`:root`. Override the ones you want to change:

```css
:root {
  --color-bg: #ffffff;
  --color-text: #1a1a2e;
  --color-primary: #22d3ee;
  --max-width: 1200px;
}
```

> **Note on the brand tokens.** The default stylesheet marks the `--color-*`
> brand tokens as LOCKED in its own source — that means the embedded default
> will not change those values, not that themes cannot override them. Your
> theme is free to redefine any token. The lock exists so the upstream visual
> identity stays stable across releases.

### 4. Configure the theme

Set `THEME_DIR` in your `.env`:

```bash
THEME_DIR=themes/mytheme
```

### 5. Restart Lesstruct

Themes are loaded at startup. Restart the server to apply changes.

## CSS Variable Reference

The full set of custom properties the default theme defines.

### Brand colors

| Variable | Default | Description |
|----------|---------|-------------|
| `--color-bg` | `#ffffff` | Page background color |
| `--color-text` | `#1a1a2e` | Main text color |
| `--color-text-muted` | `#6b7280` | Secondary / muted text |
| `--color-primary` | `#22d3ee` | Primary brand color (links, buttons, focus rings) |
| `--color-primary-hover` | `#06b6d4` | Primary color on hover |
| `--color-secondary` | `#2563eb` | Secondary brand color (logo, active nav, headings) |
| `--color-accent` | `#8b5cf6` | Accent color (tags, highlights) |
| `--color-border` | `#e5e7eb` | Border and divider color |
| `--color-card-bg` | `#f9fafb` | Card and elevated surface background |

### Status colors

| Variable | Default | Description |
|----------|---------|-------------|
| `--color-danger` | `#dc2626` | Error and validation messages |
| `--color-success` | `#16a34a` | Success messages |
| `--color-link` | `#0e7490` | Body-text link ink (darker cyan; `--color-primary` is too light for text) |
| `--color-link-hover` | `#155e75` | Body-text link ink on hover |

### Layout

| Variable | Default | Description |
|----------|---------|-------------|
| `--max-width` | `1200px` | Outer container max width |
| `--content-width` | `768px` | Single-article reading width |
| `--header-height` | `80px` | Sticky header height (used for anchor offset) |

### Radii

| Variable | Default | Description |
|----------|---------|-------------|
| `--radius-sm` | `4px` | Small elements (badges) |
| `--radius-md` | `6px` | Buttons, inputs, alerts |
| `--radius-lg` | `8px` | Cards, modals |

### Elevation

| Variable | Default | Description |
|----------|---------|-------------|
| `--shadow-sm` | `0 1px 2px rgba(0, 0, 0, 0.05)` | Subtle lift |
| `--shadow-md` | `0 4px 16px rgba(0, 0, 0, 0.10)` | Cards, hovered inputs |
| `--shadow-lg` | `0 8px 24px rgba(0, 0, 0, 0.12)` | Modals, popovers |

### Spacing

| Variable | Default | Description |
|----------|---------|-------------|
| `--space-1` | `0.25rem` | Tightest gap |
| `--space-2` | `0.5rem` | |
| `--space-3` | `0.75rem` | |
| `--space-4` | `1rem` | Standard gap |
| `--space-5` | `1.5rem` | Container padding, card padding |
| `--space-6` | `2rem` | Section spacing |
| `--space-8` | `3rem` | Page-top spacing |

### Motion and focus

| Variable | Default | Description |
|----------|---------|-------------|
| `--transition-fast` | `0.2s ease` | Default transition timing |
| `--ring` | `0 0 0 3px color-mix(in srgb, var(--color-primary) 22%, transparent)` | Focus ring for all text fields |

### Typography

| Variable | Default | Description |
|----------|---------|-------------|
| `--font-sans` | `'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif` | Body and headings |
| `--font-mono` | `"JetBrains Mono", "Fira Code", "Cascadia Code", monospace` | Code blocks and inline `<code>` |

## Font Customization

The default theme loads Inter from Google Fonts through a plain `<link>`
stylesheet in the `<head>` of `layout.html` (kept parallel with the theme
stylesheets — never `@import` it from CSS, which would serialize discovery
and delay first paint). To switch:

1. Point the font `<link>` in your `templates/layout.html` at the new family
   (or self-host and `@font-face` it, keeping the `<link>` parallel).
2. Override `--font-sans` on `:root`.
3. Override `--font-mono` if you want a different monospace stack.

```html
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Fira+Sans:wght@400;600;700&display=swap">

:root {
  --font-sans: 'Fira Sans', -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --font-mono: 'Fira Code', monospace;
}
```

## Dark Theme Example

Invert the light/dark variables, then re-tune the shadows and surfaces for a
dark backdrop:

```css
:root {
  --color-bg: #0f172a;
  --color-text: #e2e8f0;
  --color-text-muted: #94a3b8;
  --color-primary: #f59e0b;
  --color-primary-hover: #d97706;
  --color-secondary: #14b8a6;
  --color-accent: #8b5cf6;
  --color-border: #334155;
  --color-card-bg: #1e293b;
  --color-danger: #f87171;
  --color-success: #4ade80;
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.40);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.45);
  --shadow-lg: 0 8px 24px rgba(0, 0, 0, 0.55);
}
```

Also re-style form inputs (`background`, `color`, `caret-color`) and the
`<pre><code>` blocks if you want them distinct from the page background.

## Environment Variable

| Variable | Default | Description |
|----------|---------|-------------|
| `THEME_DIR` | `""` (empty) | Path to custom theme directory. Relative or absolute. Read at startup; restart required. |

When `THEME_DIR` is empty or unset, no disk access happens for the content
site; all files come from the embedded defaults.

## Fallback Behavior

For every request, Lesstruct checks the theme directory first and falls back to
the embedded default if a file is missing. This is independent for static
files and for each named template. You can override `style.css` only and keep
the embedded `search.js`, `auth.js`, `layout.html`, and every other file. The
fallbacks compose — partial themes are the normal case.

The `root/` directory does **not** participate in this fallback — it is additive
only (files there are never overridden by, or fall back to, anything embedded).
See [Root-Level Files](#root-level-files).

## Template Overrides

Themes can override any of the 10 page templates. Place your overrides in
`themes/<name>/templates/`.

### Per-post-type templates

Every content row is rendered through a per-post-type template. The lookup chain
is:

1. **Theme** `<theme>/templates/<postType>-<slug>.html` — per-slug override (see [below](#per-slug-template-overrides))
2. **Theme** `<theme>/templates/<postType>.html` — e.g. `templates/page.html`
3. **Theme** `<theme>/templates/post.html`
4. **Embedded** `pages/<postType>.gohtml` (reserved for future built-ins)
5. **Embedded** `pages/post.gohtml` — the universal default

This means you can ship a `page.html` that omits the related-posts section,
author box, and date metadata, while `post.gohtml` keeps the full blog chrome.
If a theme only ships `post.html`, every post type (including `page`) renders
through that one template. If a theme ships neither, the embedded `post.gohtml`
is used for everything.

### Per-slug template overrides

A theme can ship a distinct template for **one specific content row** by naming
the file `<postType>-<slug>.html`. The file applies only when both the post
type and the slug match the request. This mirrors the WordPress
`page-{slug}.php` convention but generalizes to every post type.

Example: a theme that wants `/about` (a Page with slug `about`) to render
through a dedicated layout ships `themes/<name>/templates/page-about.html`.
The Page with slug `contact` (also `postType=page`) is unaffected and
continues to render through `page.html`.

The lookup chain is the one above; per-slug is checked first, then the
per-post-type fallbacks apply.

#### Naming rules

- Filename pattern: `<postType>-<slug>.html` (e.g. `page-about.html`,
  `article-spotlight.html`, `menu-item-special.html`).
- `<postType>` must be a registered post-type slug (`post`, `page`, or any
  custom type declared in `config.toml`). Files whose prefix is not a known
  post type are ignored.
- When the post-type slug itself contains a hyphen (e.g. `menu-item`), the
  **longest** matching registered post type wins. So `menu-item-special.html`
  is interpreted as `menu-item` + `special`, not `menu` + `item-special`
  (assuming both `menu` and `menu-item` are registered).
- A per-post-type template file itself (e.g. `page.html`) is never mistaken
  for a per-slug override — it has no hyphen and is therefore skipped by the
  per-slug scan.
- Files are discovered at startup by scanning `<theme>/templates/`. Add or
  remove a per-slug file, then restart Lesstruct to pick up the change.

#### What per-slug templates do NOT solve on their own

`html/template` cannot run database queries, so a per-slug template alone
cannot render dynamic regions such as "recent posts of type X" or "top
contributors ranked by a custom field". Those regions are added at the theme
layer via client-side fetches against the public API — see
[Dynamic regions via CSR](#dynamic-regions-via-csr) below. The per-slug
template is the layout shell where those region hooks live.

#### Dynamic regions via CSR

A per-slug template defines DOM hooks (elements with `data-*` attributes);
a theme JS file fetches `/api/v1/public/*` and renders into them. The page
body stays server-rendered; only the dynamic regions are client-rendered.

```html
<!-- themes/<name>/templates/page-chefs.html -->
{{define "body"}}
<article class="page-chefs">
	<div class="page-body">{{.Body}}</div>

	<aside class="page-sidebar">
		<h2>Top Chefs</h2>
		<ul id="top-chefs" data-limit="5"></ul>
	</aside>

	<section class="recent-recipes">
		<h2>Latest Recipes</h2>
		<div id="recent-recipes"
		     data-post-type="recipe"
		     data-limit="8"></div>
		<a href="/recipe">See all recipes.</a>
	</section>
</article>
<script src="/static/page-chefs.js" defer></script>
{{end}}
```

```js
// themes/<name>/static/page-chefs.js
async function renderTopChefs() {
	const ul = document.getElementById('top-chefs');
	const limit = ul.dataset.limit;
	const res = await fetch(`/api/v1/public/authors?sort_by=cf:rating&order=desc&limit=${limit}`);
	const authors = (await res.json()).data;
	ul.innerHTML = authors.map(a =>
		`<li><a href="/authors/${a.username}">${a.displayName}</a> (${a.contentCount})</li>`
	).join('');
}

async function renderRecentRecipes() {
	const el = document.getElementById('recent-recipes');
	const { postType, limit } = el.dataset;
	const res = await fetch(`/api/v1/public/content_items?post_type=${postType}&limit=${limit}`);
	const items = (await res.json()).data;
	el.innerHTML = items.map(p =>
		`<article><a href="/${p.slug}">${p.title}</a></article>`
	).join('');
}

renderTopChefs();
renderRecentRecipes();
```

The corresponding `config.toml` entries — these are mandatory; without them the cf queries above return `400 field_not_queryable`:

```toml
[[public_field]]
resource   = "user"
field      = "rating"
operations = ["sort"]
```

See the **Public custom-field query** section of the API reference for the full parameter catalog and the numeric-safety rules.

### Per-post scripts

For post-specific scripts (e.g., Chart.js canvases), declare a `post_script` custom field (`textarea`) on the post type — declaration is the operator's opt-in:

```toml
[[post_type]]
slug = "post"
  [[post_type.fields]]
  name = "Post Script"
  slug = "post_script"
  type = "textarea"
```

When declared and non-empty, its raw HTML is emitted verbatim at the end of the post via `{{.PostScripts}}` (excluded from the visible custom-fields section and stripped from AMP). Raw emission means the admin-authored value is **executable** — only declare it on types whose editors are fully trusted (like Ghost's per-post code injection). Prefer `<script src="/static/...">` references (same-origin, `script-src 'self'`-clean); inline `<script>…</script>` requires `'unsafe-inline'`. The embedded `post.html` already emits `{{.PostScripts}}`; custom themes add the same one line.

### Block contract

Templates use Go's `html/template` with two `{{define}}` blocks:

- **`layout.html`** must define `{{define "layout"}}…{{end}}` — the outer
  page shell (DOCTYPE, `<head>`, header, footer). It must call
  `{{template "body" .}}` inside a `<main>` element.
- **All other templates** must define `{{define "body"}}…{{end}}` —
  page-specific content that is rendered inside the layout's `<main>` element.

> If you override `layout.html`, the default `body` block from the embedded
> page templates still works. If you override only page templates, they
> continue to use the embedded `layout.html`. Either is supported; mix as you
> wish. The embedded layout is parsed first and the theme override second, so
> partials defined by the embedded layout (such as `product-card` in the
> storefront theme) remain available when you replace only the outer shell;
> a definition in your `layout.html` wins on a name collision.

### GOHTML Formatting

All `.gohtml` files must follow the Lesstruct GOHTML coding standard. Key
rules:

- **Tabs** for indentation (one per nesting level).
- **Strategic `{{- -}}` trimming** around block-control actions (`define`,
  `range`, `if`, `with`, `end`) so source is indented without injecting
  blank lines into rendered output.
- **Inline runs on one line** — `{{if}}…{{end}}` around inline elements
  (nav links, author bylines, language links) must not be broken across
  lines, as newlines between inline elements create visible spaces.
- `style="display:none"` must be kept verbatim — test assertions check it.

### Template Data Fields

Every page embeds `LayoutData`, so the layout's `.` is always populated with
the fields below.

**`LayoutData`** — available to every page:

| Field | Type | Description |
|-------|------|-------------|
| `.PageTitle` | `string` | HTML `<title>` content |
| `.Title` | `string` | Page heading |
| `.Description` | `string` | Meta description |
| `.OGTitle` | `string` | Open Graph title |
| `.OGDesc` | `string` | Open Graph description |
| `.OGImage` | `string` | Open Graph image URL (may be empty). Always absolutized against `SITE_URL`, so it is safe to drop into `og:image`/`twitter:image` verbatim — unlike body/card image URLs, which keep their storage-native (possibly root-relative) shape. |
| `.NavigationItems` | `[]NavigationItem` | Header nav items, each with `.Title`, `.URL`, `.IsActive`. Titles localize: Home renders `ui.home`, post-type entries prefer `name_id` when the negotiated language is Indonesian (base `name` otherwise). Conventional legal pages join this list only when `[legal] legal_nav` is `header` or `both`. |
| `.FooterItems` | `[]NavigationItem` | The full site map the footer renders: every published page (conventional legal pages included) plus every listing post type. Always complete, regardless of `legal_nav`; same item shape as `.NavigationItems`. |
| `.Legal` | `LegalData` | Legal projections for this request: `.CookiePolicyURL`, `.PrivacyPolicyURL`, `.TermsOfServiceURL`, `.ReturnsPolicyURL` (resolved to the negotiated language, an explicit `[legal]` override, or empty when the document is absent); `.CookieNotice` (`.Before`, `.LinkText`, `.After`, `.URL` — the notice already split around its policy link); `.CheckoutConsent` / `.RegistrationConsent` (whether the server enforces the gate). Empty URLs mean no link and no gate from that document. |
| `.CurrentPath` | `string` | Current request path |
| `.Lang` | `string` | Current language code (e.g. `"en"`, `"fr"`); **required** by `<html lang="…">` and `{{t}}` calls. Negotiated per request (`?lang` → cookie → `Accept-Language` → primary). |
| `.LanguageLinks` | `[]LanguageLink` | Alternate-language links (`.Code`, `.Name`, `.URL`); empty if no translations exist |
| `.Languages` | `[]LanguageLink` | Site-wide language switcher (`.Code`, `.Name`, `.URL` with `?lang=`); empty when a single language is configured. |
| `.SiteConfig` | `SiteConfig` | Site-wide identity (see below); same on every page |

**`SiteConfig`** — site-wide identity (from the optional `[site_config]` block in `config.toml`; see the configuration docs):

| Field | Type | Description |
|-------|------|-------------|
| `.Name` | `string` | Site name. Always populated (defaults to `"Lesstruct"` when unconfigured). Drives `og:site_name` and the `PageTitle` suffix; use it as the logo text and the logo `alt`. |
| `.Logo` | `string` | Optional logo image URL/path (e.g. `/uploads/logo.png`). Empty means no image — render `.Name` as text. The storefront header renders logo **plus** name together when set. |
| `.Favicon` | `string` | Optional favicon image URL/path (e.g. `/uploads/icon.png`). Empty means use the theme's bundled set; when set, render it as both `rel="icon"` and `rel="apple-touch-icon"`. |
| `.ProductImageRatio` | `template.CSS` | Optional storefront product media ratio (canonical `"W / H"` from `[site_config] product_image_ratio`). Empty means the hard-coded 4/5 CSS applies; when set, the storefront layout emits an inline `<style>` overriding `.product-card__media,.product-gallery__main,.cart-line__media`. |
| `.HeroImage` | `string` | Optional storefront hero photo (from `[site_config] hero_image`). Empty keeps the monogram panel; set replaces it with the photo. |
| `.HeroTitle` | `string` | Optional storefront hero title override. Empty keeps the bilingual default. |
| `.HeroSubtitle` | `string` | Optional storefront hero subtitle override. Empty keeps the bilingual default. |

**`IndexData`** — homepage (rendered via `homepage.html`) and post-type listing pages (rendered via `index.html`). Both use the same data struct; override `homepage.html` to give the homepage a distinct layout from listing pages:

| Field | Type | Description |
|-------|------|-------------|
| `.Posts` | `[]PostItem` | Latest posts (homepage) or posts of the listed type. Paginated via `?page=N`; filterable by `?year=` and `?month=` on homepage, post-type listings, and tag pages |
| `.Tags` | `[]string` | Distinct tags across all published content; populated on the homepage for tag clouds (empty on post-type listings) |
| `.Sections` | `[]HomeSection` | Per-post-type homepage sections; **only populated when `[[homepage_section]]` blocks are configured** in `config.toml`. Empty otherwise — fall back to `.Posts` |
| `.HasPrev` / `.HasNext` | `bool` | Pagination state (see `PaginationData` below) |
| `.PrevURL` / `.NextURL` | `string` | Prev/next page URLs; empty when the corresponding flag is false |
| `.CurrentPage` | `int` | 1-based current page number |

**`HomeSection`** — one per `[[homepage_section]]` block (homepage only):

| Field | Type | Description |
|-------|------|-------------|
| `.PostTypeSlug` | `string` | The post-type slug (e.g. `"article"`) |
| `.Title` | `string` | Display title (from the block's `title`, else the post type's name) |
| `.Description` | `string` | Post-type description (may be empty) |
| `.URL` | `string` | Link to the post-type listing (e.g. `/article`) |
| `.Posts` | `[]PostItem` | Items in this section |

**`PaginationData`** — embedded in `IndexData`, `AuthorData`, and `TagData`:

| Field | Type | Description |
|-------|------|-------------|
| `.CurrentPage` | `int` | 1-based page number from `?page=N` (defaults to 1) |
| `.HasPrev` | `bool` | True when not on page 1 |
| `.HasNext` | `bool` | True when another page exists |
| `.PrevURL` | `string` | URL of the previous page; page 1 links to the bare path |
| `.NextURL` | `string` | URL of the next page |

Page size is set by the `POSTS_PER_PAGE` env var (default 50, max 100). Render `<nav class="pagination">` with `{{if or .HasPrev .HasNext}}…{{end}}`.

**`PostItem`** — a card in the post grid:

| Field | Type | Description |
|-------|------|-------------|
| `.Slug` | `string` | URL slug |
| `.Title` | `string` | Post title |
| `.MetaDescription` | `string` | Short description |
| `.ImageURL` | `string` | Cover image URL (may be empty). Storage-native shape: root-relative (`/uploads/media/<file>`) with the default local driver, absolute with S3/MinIO — render as-is in `<img src>`; only SEO meta tags need absolutizing. Cards resolve the `_medium` rendition when the pipeline generated one, `_thumb` otherwise. |
| `.ImageSrcset` | `string` | Responsive image `srcset` (may be empty) |
| `.ImageSizes` | `string` | Responsive image `sizes` (may be empty) |
| `.ImageVariants` | `map[string]string` | Map of thumbnail variant URLs keyed by configured suffix (e.g. `"_large"`, `"_medium"`, `"_thumb"`). Guard with `{{with (index .ImageVariants "_large")}}` — keys depend on `[[thumbnail]]` config. Populated when a media record is resolved; empty when no media. |
| `.OriginalURL` | `string` | The unscaled original image URL (from `Media.URL`). Falls back to the raw content image URL when no media record is found. Useful for hero backgrounds, high-DPI displays, and download links. |
| `.Author` | `string` | Author display name |
| `.Username` | `string` | Author username (for `/authors/<username>` links) |
| `.AuthorAvatarURL` | `string` | Avatar URL (may be empty) |
| `.CreatedAt` | `time.Time` | Creation timestamp; format with `{{formatDate .Lang .CreatedAt}}` for a localized date, `{{.CreatedAt.Format "2006-01-02"}}` for a custom layout, or call any `time.Time` method |
| `.PostType` | `string` | Post type (e.g. `post`, `article`, `event`); use it to branch card layouts, render type badges, or build type-aware links. Emitted as `data-post-type="…"` on the default cards |
| `.Tags` | `[]string` | Post tags; render with `{{range .Tags}}<a href="/tags/{{. \| urlpath}}" class="tag">{{.}}</a>{{end}}` (empty when none). Not rendered on the default card — override `index.html` to show them |
| `.CustomFields` | `map[string]any` | Raw custom fields of the post (empty when none). Same data as `ContentData.CustomFields` on the detail page. Use `{{index .CustomFields "slug"}}` to read a value — e.g. `{{if index .CustomFields "link"}}…{{end}}` to branch card links by field presence. Not rendered on the default card — override `index.html` or `homepage.html` to use them |

**`ContentData`** — single post page:

| Field | Type | Description |
|-------|------|-------------|
| `.Slug` | `string` | Post URL slug |
| `.Body` | `template.HTML` | Rendered post body (safe HTML; do not re-escape) |
| `.Tags` | `[]string` | Post tags |
| `.Author` | `string` | Author display name |
| `.Username` | `string` | Author username |
| `.AuthorAvatarURL` | `string` | Avatar URL |
| `.CreatedAt` | `time.Time` | Creation timestamp; format with `{{formatDate .Lang .CreatedAt}}` for a localized date, `{{.CreatedAt.Format "2006-01-02"}}` for a custom layout, or call any `time.Time` method |
| `.AllowComments` | `bool` | Whether comments are enabled |
| `.CustomFields` | `map[string]any` | Raw custom-field values keyed by name |
| `.CustomFieldsFormatted` | `[]FormattedField` | Display-formatted custom fields (`.Label`, `.Value`) |
| `.CustomFieldGalleries` | `map[string][]GalleryImage` | Resolved gallery-typed custom fields keyed by field slug (`.URL`, `.Alt`, `.Thumb`, `.Medium`, `.Large`, `.Srcset`, `.Original`). Surfaces match by suffix name (`_thumb` for thumbs, `_large` for the main image) with original-URL fallbacks, so they adapt to any `[[thumbnail]]` config; prefer this over the raw `lines` fallback for gallery fields. |
| `.Related` | `[]PostItem` | Related posts (same post type & language, ranked by shared tags), rendered above the comments section; empty slice when none |
| `.Comments` | `[]CommentItem` | Comments (`.Author`, `.Text`, `.CreatedAt` — timestamp as `time.Time`) |
| `.PostType` | `string` | Post type; branch the single-page template to show type-specific metadata (e.g. event start/end vs. article link). Emitted as `data-post-type="…"` on the default `<article>` |
| `.LanguageLinks` | `[]LanguageLink` | Inherited via `LayoutData`; also rendered inside the article for translated posts |

**`AuthorData`** — author page:

| Field | Type | Description |
|-------|------|-------------|
| `.AuthorName` | `string` | Author display name |
| `.Username` | `string` | Author username |
| `.AuthorAvatarURL` | `string` | Avatar URL |
| `.Posts` | `[]PostItem` | Author's posts (paginated via `?page=N`) |
| `.CustomFieldsFormatted` | `[]FormattedField` | Author "About" custom fields plus any system fields allowlisted with `"expose"` in `[[public_field]]`; merged into one list |
| `.HasPrev` / `.HasNext` / `.PrevURL` / `.NextURL` / `.CurrentPage` | | Embedded `PaginationData` |

**`TagData`** — tag page:

| Field | Type | Description |
|-------|------|-------------|
| `.TagName` | `string` | Tag display name |
| `.Posts` | `[]PostItem` | Posts with this tag (paginated via `?page=N`) |
| `.HasPrev` / `.HasNext` / `.PrevURL` / `.NextURL` / `.CurrentPage` | | Embedded `PaginationData` |

**`AuthPageData`** (`login.html`, `register.html`, `forgot_password.html`),
**`NotFoundData`** (`not_found.html`),
**`VerifyEmailData`** (`verify_email.html`),
**`ResetPasswordData`** (`reset_password.html`) — each embeds `LayoutData`
only. The dedicated structs exist so future per-page fields can be added
without breaking the layout contract.

### Example: Custom Layout

```html
{{define "layout"}}<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.PageTitle}}</title>
<meta name="description" content="{{.Description}}">
<meta property="og:title" content="{{.OGTitle}}">
<meta property="og:description" content="{{.OGDesc}}">
<meta property="og:site_name" content="{{.SiteConfig.Name}}">
{{if .OGImage}}<meta property="og:image" content="{{.OGImage}}">{{end}}
<link rel="stylesheet" href="/static/base.css">
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header class="site-header">
<div class="container">
<a href="/" class="site-logo">{{if .SiteConfig.Logo}}<img src="{{.SiteConfig.Logo}}" alt="{{.SiteConfig.Name}}">{{else}}{{.SiteConfig.Name}}{{end}}</a>
</div>
</header>
<main class="container">{{template "body" .}}</main>
<footer class="site-footer">
<div class="container">
<p>&copy; {{.SiteConfig.Name}}</p>
</div>
</footer>
</body>
</html>{{end}}
```

> **Reminder:** if you change the math or syntax-highlighting libraries, your
> layout must load their CSS and JS instead of the katex / highlight.js ones
> pulled by the default. See
> [CDN Assets Pulled by the Default Layout](#cdn-assets-pulled-by-the-default-layout).

### Template Helper Functions

The engine registers these helpers for use in any template:

- `{{urlpath "string"}}` — URL-encodes a string. Used in tag links so
  non-ASCII tag names resolve correctly:
  `<a href="/tags/{{.TagName | urlpath}}">`.
- `{{upper "string"}}` — Uppercases a string (e.g. `{{upper .Code}}` → `ID`).
  Both embedded layouts use it for the `#language-select` dropdown options.
- `{{t .Lang "ui.key"}}` — translates a UI string for the given language.
  Falls back through the configured languages and finally English; returns
  the key itself if no translation is found. Common keys:

  | Key | Default |
  |-----|---------|
  | `ui.login` | `Login` |
  | `ui.logout` | `Logout` |
  | `ui.register` | `Register` |
  | `ui.search` | `Search` |
  | `ui.search_posts` | `Search posts...` |
  | `ui.toggle_navigation` | `Toggle navigation` |
  | `ui.no_posts` | `No posts yet.` |
  | `ui.no_comments` | `No comments yet. Be the first to comment!` |
  | `ui.login_to_comment` | `Login to comment` |
  | `ui.by_author` | `by` |
  | `ui.comments` | `Comments` |
  | `ui.submit_comment` | `Submit Comment` |
  | `ui.back_to_home` | `Back to home` |
  | `ui.not_found_404` | `404` |
  | `ui.page_not_found` | `Page not found.` |
  | `ui.forgot_password` | `Forgot Password` |
  | `ui.reset_password` | `Reset Password` |
  | `ui.verify_email_title` | `Verify Email` |
  | `ui.browse_posts` | `Browse %s.` |
  | `ui.posts_by` | `Posts by %s.` |
  | `ui.posts_tagged` | `Posts tagged %q.` |
  | `ui.cookie_notice` | `We use cookies to remember your preferences…` (neutral plain notice, used when no policy page resolves) |
  | `ui.cookie_notice_policy` | Neutral notice with `{cookie_policy}` where the policy link belongs (used only when a policy resolves) |
  | `ui.cookie_notice_store` | Storefront plain notice (`…language and currency preferences… cart…`) — the commerce edition's wording |
  | `ui.cookie_notice_store_policy` | Storefront notice with `{cookie_policy}` where the policy link belongs |
  | `ui.cookie_policy_link` | `cookie policy` (the link label) |
  | `ui.cookie_accept` | `Accept` |
  | `ui.cookie_decline` | `Decline` |
  | `ui.legal_terms` | `Terms of Service` |
  | `ui.legal_privacy` | `Privacy Policy` |

- `{{assetURL "string"}}` — `assetURL "base.css"` → `/static/base.<hash>.css`,
  `assetURL "style.css"` → `/static/style.<hash>.css` —
  Returns a versioned URL for the named static asset. Currently `base.css` and
  `style.css` are supported. The hash is computed from both files at startup;
  cache headers are `immutable` for versioned URLs.

**Additional helpers from the active theme.** The embedded theme may contribute
extra funcs (the storefront theme adds `{{lines .CustomFields.images}}` to
split a gallery field (legacy newline-separated or canonical gallery array) into a URL list). The engine's own helpers always
win on a name collision. A `THEME_DIR` theme cannot add new funcs — only the
compiled-in theme can.

## Standalone Pages

Some pages are applications rather than content: the cart, checkout, and order
tracking. A theme can ship them as **standalone pages** — server-rendered
shells at fixed public paths whose data loads client-side from the public JSON
APIs. The shell keeps the shared layout (header, footer, site name,
navigation); only the data regions are dynamic.

The available pages depend on the active theme. The eCommerce storefront theme
ships all three:

| Page key | Public path | Template file |
|---|---|---|
| `cart` | `/cart` | `cart.html` |
| `checkout` | `/checkout` | `checkout.html` |
| `orders/track` | `/orders/track` | `orders-track.html` |

A theme only serves the pages it declares, and the declarations live in the
compiled-in theme. Override a declared page by placing the matching template
under `themes/<name>/templates/`; a page the active theme does not declare is
not served, even if the file exists. If a screenshot or link returns 404, the
active theme does not declare that page.

Each shell receives `StandalonePageData`, which embeds the layout fields
(`.PageTitle`, `.Title`, `.Description`, `.Lang`, `.SiteConfig`,
`.NavigationItems`, `.CurrentPath`, `.NoIndex`, …) plus `.Page`, the page key.
The commerce handlers populate the layout identity and navigation fields;
`.Description` is empty for these application pages, and `.NoIndex` is true so
the layout can emit `<meta name="robots" content="noindex">`.

Example (`themes/<name>/templates/cart.html`):

```html
{{define "body"}}
<section class="cart-page" data-cart-page>
	<h1>{{.Title}}</h1>
	<div id="cart-lines"></div>
</section>
{{end}}
```

The shared storefront layout already loads `storefront.js`; a child template
does not need to add it. The bundled shells also include a `<noscript>` notice
so visitors without JavaScript see why the page cannot load.

The shipped `storefront.js` hydrates product cards and product pages from
`GET /api/v1/public/catalog/products/{slug}`, the cart from
`/api/v1/public/cart*`, checkout from `POST /api/v1/public/checkout`, order
tracking from `GET /api/v1/public/orders/{orderNumber}?email=`, payment methods
from `/api/v1/public/payments/methods`, and proofs/sessions from
`/api/v1/public/orders/{orderNumber}/payments` (`/payments/session` for
Midtrans). It formats prices with zero decimals for IDR/JPY/KRW/VND and two
decimals for other currencies; three-decimal currencies are unsupported.
Product cards and the product page convert through `[[catalog.display_currency]]`
when the visitor picks a currency from `#currency-select` (persisted in
browser storage only when `lesstruct_consent=accepted`; on decline the picker is view-scoped with nothing persisted; cart, checkout, and orders stay in the base
currency). The header runs Cart → Currency (`#currency-select`) → Language
(`#language-select`, options uppercased via `{{upper .Code}}` with the active
code selected; `storefront.js:initLanguageSelect` unhides it and navigates,
the blog theme uses an inline script) → Log out. Both themes render a GDPR banner (`#cookie-banner` with `#cookie-accept`/`#cookie-decline`): the blog theme toggles it with an inline script, the storefront theme with `storefront.js:initCookieBanner`. Preference cookies carry `Secure` exactly when the page is HTTPS (`HttpOnly` intentionally absent so the JS decline flow can delete the language cookie).

The banner renders `.Legal.CookieNotice` exactly as supplied: `{{.Legal.CookieNotice.Before}}{{if .Legal.CookieNotice.LinkText}}{{if .Legal.CookieNotice.URL}}<a href="{{.Legal.CookieNotice.URL}}">{{.Legal.CookieNotice.LinkText}}</a>{{else}}{{.Legal.CookieNotice.LinkText}}{{end}}{{end}}{{.Legal.CookieNotice.After}}` — every piece is a plain string the layout escapes, so a translation can never inject markup, and an unresolved policy degrades to the plain `ui.cookie_notice` sentence. The footer should range `.FooterItems` (the full sitemap), not `.NavigationItems`. When `.Legal.CheckoutConsent` or `.Legal.RegistrationConsent` is true, render a required checkbox (`#checkout-consent` / `#register-consent`, `name="consentAccepted"`, `value="true"`) linking `.Legal.TermsOfServiceURL` / `.Legal.PrivacyPolicyURL`; the client script must gate the submit and send `consentAccepted`, but the server rejects an unchecked submission with `400 CONSENT_REQUIRED` regardless. The
product page renders a multi-axis matrix with combined price/stock/image/SKU
updates and a payload-driven SKU row hidden when empty.

## Static File Overrides

Any file in the embedded `static/` can be replaced by a same-named file under
`themes/<name>/static/`. The files are served at `/static/<filename>`.
Minified theme assets keep their readable sources beside them:
`theme-storefront`'s `storefront.js` is built from its `storefront.src.js`
source; the `.src.*` dev sources are never exported to static-site archives.

| File | Used by | DOM contract |
|------|---------|--------------|
| `base.css` | All pages (linked from `layout.html`) | Imports Google Fonts, defines CSS custom properties (`:root` tokens), resets, base element styles, and `.content-body` typography. Override freely. |
| `style.css` | All pages (linked from `layout.html`) | Defines layout primitives, components, header/nav, grid, article wrappers, tags, comments, auth, footer, and responsive rules. Override freely. |
| `storefront.js` | `theme-storefront` layout (all pages); hydrates cart, checkout, order tracking, product pages, and product cards | Expects `[data-cart-page]`, `[data-checkout-page]`, `[data-track-page]`, `[data-product-page]`, `[data-variant-matrix]`, `[data-variant-axes]`, `[data-price]`, `[data-stock]`, `#product-sku-row`, `#currency-select`, `#language-select`, `#cookie-banner`, `#cookie-accept`, `#cookie-decline`, `#checkout-consent`, `#cart-count`, `#checkout-link`; reads the cart token always and the display currency only when `lesstruct_consent=accepted`; reads `document.cookie lesstruct_consent` for banner + currency gating; prefills checkout name/email from `/api/profile/` with cookie credentials; POSTs checkout with `consentAccepted` derived from `#checkout-consent` (when present) and calls `/api/v1/config`, `/api/v1/public/cart*`, `/checkout`, `/orders/*`, and `/payments/*`. Checkout success stores `order.trackingToken` (the order's guest key, delivered once) and redirects to `/orders/track?order=…&t=…`; the track page expects `#track-token` (prefilled from `?t=` or the stored order) and sends all three factors — `?email=&token=` on tracking reads, `token` in the payment-proof and hosted-session bodies. Query params are decoded one extra layer when escapes remain, so a gateway finish redirect that arrives double-encoded (Midtrans re-encodes the callback URL) still auto-loads instead of 404ing on a literal `%40`. |
| `nav-auth.js` | `layout.html` | Expects `#nav-login`, `#nav-logout`; probes `/api/profile` with the session cookie (shows `#nav-logout` when authenticated) and logs out via `POST /api/auth/logout`; handles `.nav-toggle` / `.site-nav` for mobile. |
| `search.js` | `layout.html` | Expects `.search-toggle`, `.search-box`, `#search-input`, `#search-dropdown`; fetches `/api/v1/public/search?q=…`. |
| `auth.js` | `login.html`, `register.html`, `forgot_password.html` | Expects `#login-form`/`#mfa-form`/`#register-form`/`#forgot-form`, inputs named `username`/`name`/`email`/`password`/`code`, `#mfa-code`, `#mfa-email-hint`, `#mfa-resend`, `#mfa-submit`, `#mfa-back`, `#login-links`, `#register-consent`, and `#auth-error` / `#auth-success`. POSTs to `/api/auth/login` (returns an MFA challenge), `/api/auth/mfa/verify` (sets the `lesstruct_jwt` HttpOnly session cookie and returns only the user — no token), `/api/auth/mfa/resend`, `/api/auth/register` (with `consentAccepted` from `#register-consent` when present; any `data` payload is a success — duplicates never return an error or an account ID, the existing holder gets a notice email), `/api/auth/forgot-password`. A `firstLoginRequired` response (seeded admin before setup) redirects to `/admin/first-login`. |
| `comments.js` | `post.html` in both embedded themes (only when comments are enabled) | Expects `#comment-form[data-slug]`, `#comment-error`, `#comment-success`, `#comment-login-link`; probes `/api/profile` with the session cookie to decide form vs login link; POSTs to `/api/v1/content_items/<slug>/comments` with cookie credentials (no `Authorization` header). |
| `math.js` | `layout.html` | KaTeX auto-render; depends on katex from CDN (see below). |
| `verify-email.js` | `verify_email.html` | Reads `?token=` from the URL; calls `/api/auth/verify-email?token=…`; toggles `#auth-error` / `#auth-success`. |
| `reset-password.js` | `reset_password.html` | Reads `?token=` from the URL; POSTs to `/api/auth/reset-password`; expects `#new-password` input. |
| `highlight.min.js` | `layout.html` | Provides the global `hljs`. The default layout also runs `hljs.highlightAll()` on `DOMContentLoaded`. |

The default layout uses `{{assetURL "base.css"}}` and `{{assetURL "style.css"}}` to emit content-hashed filenames with immutable cache headers. Plain `/static/base.css` and `/static/style.css` still work for backward compatibility.

### Favicons

Both embedded themes ship the canonical brand favicon set:

| File | Link emitted by the layouts |
|------|-----------------------------|
| `favicon-96x96.png` | `<link rel="icon" type="image/png" href="/static/favicon-96x96.png" sizes="96x96">` |
| `favicon.svg` | `<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">` |
| `favicon.ico` | `<link rel="shortcut icon" href="/static/favicon.ico">` |
| `apple-touch-icon.png` | `<link rel="apple-touch-icon" sizes="180x180" href="/static/apple-touch-icon.png">` |

Override any of them by shipping a same-named file in `themes/<name>/static/`
(ship the whole set for a consistent rebrand), or replace the set with a single
image from `config.toml`:

```toml
[site_config]
favicon = "/uploads/icon.png"
```

When `favicon` is set, the layouts emit that URL as both `rel="icon"` and
`rel="apple-touch-icon"` and skip the bundled links — the theme's favicon files
remain served but unused. The value is available to full `layout.html`
overrides as `.SiteConfig.Favicon`.

If you override any JS file, keep the DOM contract above — the default page
templates look for those exact ids and classes. If you change them, you must
also change the corresponding page template.

> **v1.1 login contract.** `POST /api/auth/login` no longer returns a session
> token by itself: it returns `{mfaRequired, challengeToken, emailHint,
> expiresIn}` (`expiresIn` is in seconds), and the client must exchange the
> emailed code at
> `POST /api/auth/mfa/verify`. Verify sets the `lesstruct_jwt` HttpOnly
> session cookie (lifetime matches the token expiry) and returns only the
> user — no token in any login JSON, nothing in `localStorage`. A theme that
> bundles its own `auth.js` must implement both steps and handle resend token
> rotation. `POST /api/auth/logout` revokes the session server-side and
> clears the cookie.
> The seeded admin's pre-setup login returns `{firstLoginRequired: true}` with
> no session instead of an MFA challenge.
>
> Cookie-authenticated `fetch` calls need no `Authorization` header: the browser attaches the session cookie automatically on same-origin requests (use `credentials: 'include'` when the theme is served cross-origin from the API). State-changing calls (`mfa/verify`, `logout`, comment POSTs) pass the server's CSRF origin check — same-origin `fetch` sends `Origin` automatically; cross-origin themes must send `Origin` and be allowlisted via `CORS_ALLOWED_ORIGINS`.

## CDN Assets Pulled by the Default Layout

The default `layout.html` loads the following from `cdn.jsdelivr.net`:

- `katex@0.16.11/dist/katex.min.css` and `katex.min.js` — math rendering.
- `highlight.js@11.11.1/styles/github-dark.min.css` — code-block theme.

Both stylesheets load with `media="print" onload="this.media='all'"` (plus a
`<noscript>` fallback) so they never block first paint, and the scripts load
with `defer` (execution order is document order: `katex.min.js` before the
deferred `math.js`, `highlight.min.js` before the `DOMContentLoaded`
`highlightAll()` call). Keep this async pattern when you swap libraries.

If your theme drops katex and/or highlight.js (for example, you use a
different math library or a different syntax highlighter), update
`layout.html` to drop the matching `<link>` / `<script>` tags and override
`math.js` and `highlight.min.js` accordingly. Otherwise, the assets will be
requested and unused.

## What Does NOT Theme

`THEME_DIR` only affects the public content site. It does not change:

- The admin panel (the Vue-based dashboard at `/admin/`).
- Any `/api/*` JSON response.
- Plugin behaviour, hooks, or capabilities.
- Email templates or other server-rendered channels.

## Reference URLs

When developing a theme, these URLs are useful for inspecting the embedded
defaults that your theme falls back to:

| Default | URL |
|---------|-----|
| Default stylesheet (minified) | `GET /static/base.css` |
| Default stylesheet (minified) | `GET /static/style.css` |
| Default index page (with navigation) | `GET /` |
| Default content page | `GET /<existing-slug>` |
| Default 404 page | `GET /this-does-not-exist` |
| Default author page | `GET /authors/<username>` |
| Default tag page | `GET /tags/<tag-name>` |
| Login form | `GET /login` |
| Register form | `GET /register` |
| Forgot password form | `GET /forgot-password` |
| Verify email page | `GET /verify-email?token=<valid>` |
| Reset password page | `GET /reset-password?token=<valid>` |
| Cart page (storefront theme) | `GET /cart` |
| Checkout page (storefront theme) | `GET /checkout` |
| Order tracking page (storefront theme) | `GET /orders/track` |
| Public search endpoint (used by `search.js`) | `GET /api/v1/public/search?q=<query>` |
| Login endpoint (used by `auth.js`; returns an MFA challenge) | `POST /api/auth/login` |
| MFA verify endpoint (used by `auth.js`; returns the JWT) | `POST /api/auth/mfa/verify` |
| MFA resend endpoint (used by `auth.js`) | `POST /api/auth/mfa/resend` |
| Register endpoint (used by `auth.js`) | `POST /api/auth/register` |
| Forgot-password endpoint (used by `auth.js`) | `POST /api/auth/forgot-password` |
| Verify email endpoint (used by `verify-email.js`) | `GET /api/auth/verify-email?token=<token>` |
| Reset password endpoint (used by `reset-password.js`) | `POST /api/auth/reset-password` |
| Comment submit endpoint (used by `comments.js`) | `POST /api/v1/content_items/<slug>/comments` |

## Theme Authoring Workflow

Recommended sequence for a new theme:

1. **Pick a base.** Decide whether you are re-skinning (CSS only),
   rearranging the layout (`layout.html` only), or rebuilding page templates
   individually.
2. **Create the directory.** `mkdir -p themes/mytheme/{static,templates}` — add
   `themes/mytheme/root` too when you need site-root files (e.g. a service worker).
3. **Copy only what you need.** Start with `static/style.css` (copy from
   `references/default-style-reference.css`); copy more files from the
   embedded defaults only as your design requires.
4. **Author.** Use the [CSS Variable Reference](#css-variable-reference) and
   [Template Data Fields](#template-data-fields) sections as your contract.
5. **Restart Lesstruct.** `THEME_DIR` is read at startup; live edits to a
   theme file are not picked up until the server is restarted.
6. **Verify.** Hit each page URL in
   [Reference URLs](#reference-urls) and confirm your theme loads and the
   page renders.
7. **Maintain.** When upgrading Lesstruct, fetch the new default CSS
   (`curl -s http://localhost:8080/static/base.css > /tmp/new-default-base.css`)
   and compare against your override. New template fields and JS DOM
   contracts are listed in this snapshot; new server endpoints show up in
   the embedded JavaScript.

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Theme changes have no effect | `THEME_DIR` is empty, points to a missing directory, or the server was not restarted. |
| Page renders, but no styles | `<link rel="stylesheet" href="/static/base.css">` and/or `<link rel="stylesheet" href="/static/style.css">` are missing from your `layout.html`. |
| Search box or comment form is dead | You overrode `search.js` / `comments.js` / `layout.html` and the DOM ids no longer match. Restore the ids, or update the JS to match your new layout. |
| Tag links are broken for non-ASCII tags | The href was built with `.TagName` instead of `{{.TagName | urlpath}}`. |
| `{{t .Lang "ui.x"}}` shows the literal key | The translation is missing for the current language. Add it, or change the key. |
| KaTeX or highlight.js missing | Your `layout.html` does not load the CDN CSS/JS, or the assets are blocked by the network. |
