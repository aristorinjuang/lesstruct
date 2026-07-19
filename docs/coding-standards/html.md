# GOHTML / Go Template Coding Standards

> **Scope.** This standard governs formatting and readability for all Go
> `html/template` files (`.gohtml`) in the Lesstruct default theme and in
> user-authored theme overrides. It applies to `internal/api/template/pages/`
> and to any `.gohtml` file under a `THEME_DIR/templates/` directory.

---

## Core Rules

### 1. Indentation

Use **tabs** — one per nesting level — to reflect logical / HTML nesting. This
matches `gofmt`, the WordPress HTML standard, and Hugo defaults.

```gohtml
<!-- Good -->
<div class="posts-grid container">
	{{- range .Posts}}
		<article class="post-card">
			<a href="/{{.Slug}}">
				<h2>{{.Title}}</h2>
			</a>
		</article>
	{{- end}}
</div>
```

### 2. Whitespace Trimming

Use `{{-` and `-}}` around **block-control actions** that span lines so
indentation does not leak into rendered output:

- `{{- define "name"}}` / `{{- end}}`
- `{{- range ...}}` / `{{- end}}`
- `{{- if ...}}` / `{{- else if ...}}` / `{{- else}}` / `{{- end}}`
- `{{- with ...}}` / `{{- end}}`
- `{{- template "name" ...}}`

```gohtml
<!-- Good — trimmed control actions, clean source AND clean output -->
{{- define "body"}}
<div class="not-found container">
	<h1>{{t .Lang "ui.not_found_404"}}</h1>
	<p>{{t .Lang "ui.page_not_found"}}</p>
	<a href="/" class="back-link">&larr; {{t .Lang "ui.back_to_home"}}</a>
</div>
{{- end}}
```

```gohtml
<!-- Bad — no trimming, blank lines leak into rendered HTML -->
{{define "body"}}
<div class="not-found container">
<h1>{{t .Lang "ui.not_found_404"}}</h1>
</div>
{{end}}
```

### 3. Inline Runs Stay on One Line

When an action sits **inside** an inline sequence (nav links, author byline,
language links, tags), keep the whole run on **one line**. Do not break it
across lines unless you intend to change the rendered spacing.

```gohtml
<!-- Good — author byline stays on one line (inline context) -->
{{if .Author}}
	{{if .AuthorAvatarURL}}<img src="{{.AuthorAvatarURL}}" class="author-avatar" alt="{{.Author}}">{{end}}
	<span class="author">{{t .Lang "ui.by_author"}} {{.Author}}</span>
{{end}}
```

```gohtml
<!-- Bad — newline between <img> and <span> adds a visible space -->
{{if .Author}}
	{{if .AuthorAvatarURL}}<img src="{{.AuthorAvatarURL}}" class="author-avatar" alt="{{.Author}}">
	{{end}}
	<span class="author">{{t .Lang "ui.by_author"}} {{.Author}}</span>
{{end}}
```

**Rule of thumb:** if two elements are both `display: inline` by default (e.g.
`<img>`, `<a>`, `<span>`, `<time>`, `<strong>`), a newline between them
renders as a visible space. A newline between two **block** elements (e.g.
`<div>`, `<section>`, `<article>`, `<header>`, `<footer>`, `<main>`, `<nav>`,
`<form>`) is insignificant.

### 4. Inline Conditionals

Keep single-element conditionals on one line — this reads cleanly and avoids
inline whitespace changes:

```gohtml
<!-- Good -->
{{if .OGImage}}<meta property="og:image" content="{{.OGImage}}">{{end}}
{{if .MetaDescription}}<p>{{.MetaDescription}}</p>{{end}}
```

### 5. Attribute Style

- Always **double-quote** attribute values.
- Use **lowercase** tag and attribute names.
- Preserve `style="display:none"` verbatim — test assertions check for it
  exactly.
- Keep the existing void-element style (no self-closing slash):
  `<img ...>` not `<img ... />`. Changing this is a separate decision.

### 6. Block-Level vs Inline Conditionals

Block-level conditionals wrap entire elements and get trimmed + indented:

```gohtml
<!-- Good — block-level range, indented -->
{{- range .Sections}}
	<section class="home-section container" data-post-type="{{.PostTypeSlug}}">
		<header class="home-section__header">
			<h2>{{.Title}}</h2>
		</header>
	</section>
{{- end}}
```

Inline conditionals that wrap or precede a single inline element stay on one
line with the element:

```gohtml
<!-- Good — inline conditional -->
{{if .ImageURL}}<img src="{{.ImageURL}}" alt="{{.Title}}" class="post-card__image" loading="lazy">{{end}}
```

### 7. `define` and `end`

- `{{- define "name"}}` starts at column 0.
- `{{- end}}` closes at column 0.
- `{{template "body" .}}` is indented inside its parent, with no trimming
  (the layout owns the `{{- define}}` / `{{- end}}` pair).

### 8. Comments

Do not add comments in markup unless documenting a non-obvious data contract.
If you do add a comment, use HTML comments:

```gohtml
<!-- Comments follow HTML syntax, not Go template comments -->
```

---

## Before / After Example

**Before** (flat, compact, hard to read):
```gohtml
{{define "body"}}
<div class="auth-page container">
<h1>{{t .Lang "ui.login"}}</h1>
<div id="auth-error" class="auth-error alert alert--error" style="display:none"></div>
<form id="login-form" class="auth-form">
<label for="username">{{t .Lang "ui.username_or_email"}}</label>
<input type="text" id="username" name="username" class="form-control" required>
<label for="password">{{t .Lang "ui.password_label"}}</label>
<input type="password" id="password" name="password" class="form-control" autocomplete="current-password" required>
<button type="submit" class="btn">{{t .Lang "ui.login"}}</button>
</form>
<div class="auth-links">
<p><a href="/register">{{t .Lang "ui.create_account"}}</a></p>
<p><a href="/forgot-password">{{t .Lang "ui.forgot_password_link"}}</a></p>
</div>
</div>
<script src="/static/auth.js"></script>
{{end}}
```

**After** (indented, trimmed, readable):
```gohtml
{{- define "body"}}
<div class="auth-page container">
	<h1>{{t .Lang "ui.login"}}</h1>
	<div id="auth-error" class="auth-error alert alert--error" style="display:none"></div>
	<form id="login-form" class="auth-form">
		<label for="username">{{t .Lang "ui.username_or_email"}}</label>
		<input type="text" id="username" name="username" class="form-control" required>
		<label for="password">{{t .Lang "ui.password_label"}}</label>
		<input type="password" id="password" name="password" class="form-control" autocomplete="current-password" required>
		<button type="submit" class="btn">{{t .Lang "ui.login"}}</button>
	</form>
	<div class="auth-links">
		<p><a href="/register">{{t .Lang "ui.create_account"}}</a></p>
		<p><a href="/forgot-password">{{t .Lang "ui.forgot_password_link"}}</a></p>
	</div>
</div>
<script src="/static/auth.js"></script>
{{- end}}
```

---

## Verification

After reformatting any `.gohtml` file:

1. Run `go test ./internal/api/template/... -v` — all substring assertions
   must still pass (especially `style="display:none"`, `href="/static/style.<hash>.css"`,
   `<p>Hello <strong>world</strong></p>`).
2. Visually inspect rendered HTML output via `httptest` to confirm whitespace
   changes are restricted to block contexts; no new visible-space regressions
   in inline runs.
3. Run `make lint && make test` at the end of the change.

---

## Why This Matters

Lesstruct ships server-rendered HTML **unminified** — only `base.css` and `style.css` are
minified. This means every tab and newline in a `.gohtml` file is sent to the
browser. Without trimming, reformatting for readability would inject blank lines
and extra whitespace into rendered output. Strategic `{{- -}}` trimming
lets us indent for humans **without** changing the output for browsers.

The standard applies to both the default theme (embedded at
`internal/api/template/pages/`) and user-authored theme overrides
(`themes/<name>/templates/`), since both are parsed by the same Go template
engine and rendered unminified.
