# Theme Development Guide

> **Audience.** This is the **developer-facing** reference for Lesstruct theme
> development. It references source-tree paths (e.g. `internal/api/template/`).
>
> If you are an end user of Lesstruct — i.e. you have installed the binary and
> want to customise the public site via `themes/<name>/` — use the
> **user-facing** snapshot bundled with the `lesstruct-theme-development` skill
> at `skills/lesstruct-theme-development/references/theme-development.md`. It
> covers the same contract (CSS variables, template blocks, JS DOM contract,
> CDN assets) but with no source-tree references.

Lesstruct supports custom themes for the public-facing content site. Themes override the
default CSS, JavaScript, and (optionally) HTML templates without modifying the core source.

## How Themes Work

1. Create a theme directory with your custom files.
2. Set the `THEME_DIR` environment variable to point to it.
3. At startup, Lesstruct resolves each template and static file through a
   `compositeFS` (`internal/api/template/theme.go:44-58`) and `readThemeFile`
   (`internal/api/template/theme.go:30-41`):
   - If the file exists under `THEME_DIR`, that copy is used.
   - If it is missing, the embedded default from `internal/api/template/` is used.

This means you can ship a **partial** theme — a single `style.css`, `base.css` + `style.css`, or a full
`layout.html`, or anything in between — and the rest stays on the embedded defaults.

## Theme Directory Structure

```
themes/
  mytheme/
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
```

The theme can override **any subset** of these files. Any file not present falls
back to the embedded default at `internal/api/template/static/` or
`internal/api/template/pages/`. You can also add per-post-type templates (e.g.
`page.html`, `event.html`) — see [Per-post-type templates](#per-post-type-templates).

## Quick Start: CSS-Only Theme

The simplest theme overrides only the CSS.

### 1. Create the theme directory

```bash
mkdir -p themes/mytheme/static
```

### 2. Start from the readable source

The minified `internal/api/template/static/base.css` and `style.css` are the files browsers receive.
The readable, documented sources are `internal/api/template/static/base.src.css` and
`style.src.css` (commented, organised by section). Copy the readable sources:

```bash
cp internal/api/template/static/base.src.css themes/mytheme/static/base.css
cp internal/api/template/static/style.src.css themes/mytheme/static/style.css
```

Theme authors do not need to run `make css`. Browsers receive your `base.css` and `style.css`
verbatim. (If you maintain `.src.css` files for your own authoring convenience and want
to ship minified versions, run `make css` against your source — but the theme
override is happy with any valid CSS.)

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

> **Note on the brand tokens.** The `--color-*` brand tokens are marked **LOCKED** in
> the embedded `style.src.css:32-35` — that means the embedded source will not change
> those values, not that themes cannot override them. Your theme is free to redefine
> any token. The lock exists so the upstream visual identity stays stable.

### 4. Configure the theme

Set `THEME_DIR` in your `.env`:

```bash
THEME_DIR=themes/mytheme
```

### 5. Restart Lesstruct

Themes are loaded at startup. Restart the server to apply changes.

## CSS Variable Reference

The full set, defined in `internal/api/template/static/style.src.css:36-85`.

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

The default theme imports Inter from Google Fonts at the top of `base.css`. To
switch:

1. Replace the `@import` line at the top of your `base.css` with the new font's
   `@import` (or self-host and `@font-face` it).
2. Override `--font-sans` on `:root`.
3. Override `--font-mono` if you want a different monospace stack.

```css
@import url('https://fonts.googleapis.com/css2?family=Fira+Sans:wght@400;600;700&display=swap');

:root {
  --font-sans: 'Fira Sans', -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --font-mono: 'Fira Code', monospace;
}
```

## Dark Theme Example

Invert the light/dark variables, then re-tune the shadows and surfaces for a dark
backdrop:

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

`THEME_DIR` is loaded in `internal/config/config.go:117` and passed to
`template.NewTemplates` in `main.go:672-678`.

## Fallback Behavior

`compositeFS` (`internal/api/template/theme.go:44-58`) wraps the theme directory on
top of the embedded filesystem. For every request:

- If the file exists in `THEME_DIR/...`, that copy is served.
- Otherwise, the embedded default is served.

This is independent for static files and for each named template. You can override
`style.css` only and keep the embedded `search.js`, `auth.js`, `layout.html`, and
every other file. The fallbacks compose — partial themes are the normal case.

When `THEME_DIR` is empty or unset, no disk access happens for the content site;
all files come from the embedded filesystem.

## Template Overrides

Themes can override any of the 10 templates in `internal/api/template/pages/`. Place
your overrides in `themes/<name>/templates/`.

### Per-post-type templates

Every content row is rendered through a per-post-type template. The lookup chain
is:

1. **Theme** `<theme>/templates/<postType>.html` — e.g. `templates/page.html`
2. **Theme** `<theme>/templates/post.html`
3. **Embedded** `pages/<postType>.gohtml` (reserved for future built-ins)
4. **Embedded** `pages/post.gohtml` — the universal default

This means you can ship a `page.html` that omits the related-posts section,
author box, and date metadata, while `post.gohtml` keeps the full blog chrome.
If a theme only ships `post.html`, every post type (including `page`) renders
through that one template. If a theme ships neither, the embedded `post.gohtml`
is used for everything.

### Block contract

Templates use Go's `html/template` with two `{{define}}` blocks:

- **`layout.html`** must define `{{define "layout"}}…{{end}}` — the outer page
  shell (DOCTYPE, `<head>`, header, footer). It must call `{{template "body" .}}`
  inside a `<main>` element. Layouts are cloned per page in
  `internal/api/template/template.go:201-210`, so each page template is parsed
  against a fresh copy of the layout.
- **All other templates** must define `{{define "body"}}…{{end}}` — page-specific
  content that is rendered inside the layout's `<main>` element.

> If you override `layout.html`, the default `body` block from the embedded
> page templates still works. If you override only page templates, they continue
> to use the embedded `layout.html`. Either is supported; mix as you wish.

### GOHTML Formatting

All `.gohtml` files must follow the Lesstruct GOHTML coding standard
(`docs/coding-standards/html.md`). Key rules:

- **Tabs** for indentation (one per nesting level).
- **Strategic `{{- -}}` trimming** around block-control actions (`define`,
  `range`, `if`, `with`, `end`) so source is indented without injecting
  blank lines into rendered output.
- **Inline runs on one line** — `{{if}}…{{end}}` around inline elements
  (nav links, author bylines, language links) must not be broken across
  lines, as newlines between inline elements create visible spaces.
- `style="display:none"` must be kept verbatim — test assertions check it.

Lesstruct ships server-rendered HTML unminified. Without trimming, formatting
changes would ship whitespace to the browser. The standard exists so source is
readable for humans and AI while rendered output stays clean.

### Template Data Fields

The structs are defined in `internal/api/template/template.go`. Every page
embeds `LayoutData`, so the layout's `.` is always populated with the fields
below.

**`LayoutData`** — available to every page:

| Field | Type | Description |
|-------|------|-------------|
| `.PageTitle` | `string` | HTML `<title>` content |
| `.Title` | `string` | Page heading |
| `.Description` | `string` | Meta description |
| `.OGTitle` | `string` | Open Graph title |
| `.OGDesc` | `string` | Open Graph description |
| `.OGImage` | `string` | Open Graph image URL (may be empty) |
| `.NavigationItems` | `[]NavigationItem` | Nav items, each with `.Title`, `.URL`, `.IsActive` |
| `.CurrentPath` | `string` | Current request path |
| `.Lang` | `string` | Current language code (e.g. `"en"`, `"fr"`); **required** by `<html lang="…">` and `{{t}}` calls |
| `.LanguageLinks` | `[]LanguageLink` | Alternate-language links (`.Code`, `.Name`, `.URL`); empty if no translations exist |
| `.SiteConfig` | `SiteConfig` | Site-wide identity (see below); same on every page |

**`IndexData`** — homepage (rendered via `homepage.html`) and post-type listing pages (rendered via `index.html`). Both use the same data struct; override `homepage.html` to give the homepage a distinct layout from listing pages:

| Field | Type | Description |
|-------|------|-------------|
| `.Posts` | `[]PostItem` | Latest posts (homepage) or posts of the listed type. Paginated via `?page=N`; filterable by `?year=` and `?month=` on homepage, post-type listings, and tag pages |
| `.Tags` | `[]string` | Distinct tags across all published content; populated on the homepage for tag clouds (empty on post-type listings) |
| `.Sections` | `[]HomeSection` | Per-post-type homepage sections; **only populated when `[[homepage_section]]` blocks are configured** in `config.toml` (see `docs/configuration.md`). Empty otherwise — fall back to `.Posts` |
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

**`SiteConfig`** — site-wide identity (from the optional `[site_config]` block in `config.toml`; see `docs/configuration.md`):

| Field | Type | Description |
|-------|------|-------------|
| `.Name` | `string` | Site name. Always populated (defaults to `"Lesstruct"` when unconfigured). Drives `og:site_name` and the `PageTitle` suffix; use it as the logo text and the logo `alt`. |
| `.Logo` | `string` | Optional logo image URL/path (e.g. `/uploads/logo.png`). Empty means no image — render `.Name` as text. |

**`PaginationData`** — embedded in `IndexData`, `AuthorData`, and `TagData`:

| Field | Type | Description |
|-------|------|-------------|
| `.CurrentPage` | `int` | 1-based page number from `?page=N` (defaults to 1) |
| `.HasPrev` | `bool` | True when not on page 1 |
| `.HasNext` | `bool` | True when another page exists (detected without a COUNT query) |
| `.PrevURL` | `string` | URL of the previous page; page 1 links to the bare path |
| `.NextURL` | `string` | URL of the next page |

Page size is set by the `POSTS_PER_PAGE` env var (default 50, max 100). Use `{{if or .HasPrev .HasNext}}…{{end}}` to render a `<nav class="pagination">`.

**`PostItem`** — a card in the post grid:

| Field | Type | Description |
|-------|------|-------------|
| `.Slug` | `string` | URL slug |
| `.Title` | `string` | Post title |
| `.MetaDescription` | `string` | Short description |
| `.ImageURL` | `string` | Cover image URL (may be empty) |
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
| `.CustomFieldsFormatted` | `[]FormattedField` | Author "About" custom fields |
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
**`ResetPasswordData`** (`reset_password.html`) — each embeds `LayoutData` only.
The dedicated structs exist so future per-page fields can be added without
breaking the layout contract.

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
> pulled by the default. See [CDN Assets Pulled by the Default Layout](#cdn-assets-pulled-by-the-default-layout).

### Template Helper Functions

Registered in `internal/api/template/template.go:191-194`:

- `{{urlpath "string"}}` — URL-encodes a string. Used in tag links so non-ASCII
  tag names resolve correctly: `<a href="/tags/{{.TagName | urlpath}}">`.
- `{{t .Lang "ui.key"}}` — translates a UI string for the given language. Falls
  back through the configured languages and finally English; returns the key
  itself if no translation is found. The catalog lives in
  `internal/i18n/catalog.go` and the source strings in
  `internal/i18n/translations/*.toml`. Common keys:

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

  Run `ls internal/i18n/translations/` to see every supported language.

- `{{assetURL "string"}}` — `assetURL "base.css"` → `/static/base.<hash>.css`,
  `assetURL "style.css"` → `/static/style.<hash>.css` —
  Returns a versioned URL for the named static asset. Currently `base.css` and
  `style.css` are supported. The hash is computed from both files at startup;
  cache headers are `immutable` for versioned URLs.

## Static File Overrides

Any file in `internal/api/template/static/` can be replaced by a same-named file
under `themes/<name>/static/`. The files are served at `/static/<filename>`.

| File | Used by | DOM contract |
|------|---------|--------------|
| `base.css` | All pages (linked from `layout.html`) | Imports Google Fonts, defines CSS custom properties (`:root` tokens), resets, base element styles, and `.content-body` typography. Override freely. |
| `style.css` | All pages (linked from `layout.html`) | Defines layout primitives, components, header/nav, grid, article wrappers, tags, comments, auth, footer, and responsive rules. Override freely. |

The default layout uses `{{assetURL "base.css"}}` and `{{assetURL "style.css"}}` to emit content-hashed filenames with immutable cache headers. Plain `/static/base.css` and `/static/style.css` still work for backward compatibility.
| `nav-auth.js` | `layout.html` | Expects `#nav-login`, `#nav-logout`; reads `localStorage.token` or `localStorage.auth_token`; handles `.nav-toggle` / `.site-nav` for mobile. |
| `search.js` | `layout.html` | Expects `.search-toggle`, `.search-box`, `#search-input`, `#search-dropdown`; fetches `/api/v1/public/search?q=…`. |
| `auth.js` | `login.html`, `register.html`, `forgot_password.html` | Expects `#login-form`/`#register-form`/`#forgot-form`, inputs named `username`/`name`/`email`/`password`, and `#auth-error` / `#auth-success` elements. POSTs to `/api/auth/login`, `/api/auth/register`, `/api/auth/forgot-password`. |
| `comments.js` | `post.html` (only when `AllowComments` is true) | Expects `#comment-form[data-slug]`, `#comment-error`, `#comment-success`, `#comment-login-link`; reads `localStorage.token`; POSTs to `/api/v1/content_items/<slug>/comments`. |
| `math.js` | `layout.html` | KaTeX auto-render; depends on katex from CDN (see below). |
| `verify-email.js` | `verify_email.html` | Reads `?token=` from the URL; calls `/api/auth/verify-email?token=…`; toggles `#auth-error` / `#auth-success`. |
| `reset-password.js` | `reset_password.html` | Reads `?token=` from the URL; POSTs to `/api/auth/reset-password`; expects `#new-password` input. |
| `highlight.min.js` | `layout.html` | Provides the global `hljs`. The default layout also runs `hljs.highlightAll()` on `DOMContentLoaded`. |

If you override any JS file, keep the DOM contract above — the default page
templates look for those exact ids and classes. If you change them, you must
also change the corresponding page template.

## CDN Assets Pulled by the Default Layout

The default `layout.html` (`internal/api/template/pages/layout.gohtml`) loads the
following from `cdn.jsdelivr.net`:

- `katex@0.16.11/dist/katex.min.css` and `katex.min.js` — math rendering.
- `highlight.js@11.11.1/styles/github-dark.min.css` — code-block theme.

If your theme drops katex and/or highlight.js (for example, you use a different
math library or a different syntax highlighter), update `layout.html` to drop the
matching `<link>` / `<script>` tags and override `math.js` and `highlight.min.js`
accordingly. Otherwise, the assets will be requested and unused.

## What Does NOT Theme

`THEME_DIR` only affects the public content site. It does not change:

- The admin SPA (`web/admin/`, served from `internal/api/static/admin/`).
- Any `/api/*` JSON response.
- Plugin behaviour, hooks, or capabilities.
- Email templates or other server-rendered channels.

To rebrand the admin panel, edit the Vue source under `web/admin/` and rebuild
with `make build-admin`. To change API responses, edit the handlers under
`internal/api/handlers/`.

## Theme Authoring Workflow

Recommended sequence for a new theme:

1. **Pick a base.** Decide whether you are re-skinning (CSS only), rearranging
   the layout (`layout.html` only), or rebuilding page templates individually.
2. **Create the directory.** `mkdir -p themes/mytheme/{static,templates}`.
3. **Copy only what you need.** Start with `static/base.css` and `static/style.css`; copy more files
   from `internal/api/template/static/` and `internal/api/template/pages/` only
   as your design requires.
4. **Author.** Use the [CSS Variable Reference](#css-variable-reference) and
   [Template Data Fields](#template-data-fields) sections as your contract.
5. **Restart Lesstruct.** `THEME_DIR` is read at startup; live edits to a theme
   file are not picked up until the server is restarted.
6. **Verify.** Hit each of the 10 pages (`/`, `/<slug>`, `/authors/<username>`,
   `/tags/<tag>`, `/404`, `/login`, `/register`, `/forgot-password`,
   `/verify-email?token=…`, `/reset-password?token=…`) and confirm your theme
   loads and the page renders. Run `go test ./internal/api/template/...` to
   confirm the embedded fallback paths still work.
7. **Maintain.** When upgrading Lesstruct, run the **theme development skill**
   (`lesstruct-theme-development`) to detect drift between your theme files and
   any new embedded defaults.

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Theme changes have no effect | `THEME_DIR` is empty, points to a missing directory, or the server was not restarted. |
| Page renders, but no styles | `<link rel="stylesheet" href="/static/base.css">` and/or `<link rel="stylesheet" href="/static/style.css">` are missing from your `layout.html`. |
| Search box or comment form is dead | You overrode `search.js` / `comments.js` / `layout.html` and the DOM ids no longer match. Restore the ids, or update the JS to match your new layout. |
| Tag links are broken for non-ASCII tags | The href was built with `.TagName` instead of `{{.TagName | urlpath}}`. |
| `{{t .Lang "ui.x"}}` shows the literal key | The translation is missing in `internal/i18n/translations/<lang>.toml`. Add it, or change the key. |
| KaTeX or highlight.js missing | Your `layout.html` does not load the CDN CSS/JS, or the assets are blocked by the network. |
