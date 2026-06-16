# Theme Audit Checklist

Use this checklist to audit a `themes/<name>/` directory before declaring a
theme ready. Each item is a hard pass / fail.

## Environment

- [ ] `THEME_DIR` is set in `.env` or the environment the Lesstruct process
      runs in (not only in a shell session that has since closed).
- [ ] `THEME_DIR` resolves to an existing directory.
- [ ] Lesstruct was restarted after the most recent `THEME_DIR` change.
- [ ] The theme directory has a `static/` and a `templates/` subdirectory
      (even if empty).

## CSS (`static/style.css`)

- [ ] `static/style.css` is valid CSS (run it through a linter).
- [ ] `static/style.css` does not reference `style.src.css` (the source file
      is not served; only the minified `style.css` is).
- [ ] Every CSS variable the default theme defines is still defined in the
      override (or in a parent file the override loads). The full set is in
      the [CSS Variable Reference](theme-development.md#css-variable-reference).
- [ ] `@import` URLs (fonts, third-party CSS) resolve.

## Layout template (`templates/layout.html`)

- [ ] File defines `{{define "layout"}}…{{end}}` exactly once.
- [ ] The HTML element has `lang="{{.Lang}}"`.
- [ ] `<title>{{.PageTitle}}</title>` is present.
- [ ] `<link rel="stylesheet" href="/static/style.css">` is present.
- [ ] `<main>…{{template "body" .}}…</main>` is present.
- [ ] The Open Graph `<meta>` tags are present (otherwise share previews
      break).
- [ ] `<script src="/static/nav-auth.js"></script>` and
      `<script src="/static/search.js"></script>` are present (otherwise
      the nav login/logout toggle and search dropdown break).

## Page templates (any `templates/<page>.html` you ship)

- [ ] The template defines `{{define "body"}}…{{end}}` exactly once.
- [ ] The template uses the data fields listed in
      [Template Data Fields](theme-development.md#template-data-fields) by
      their documented names (`.Title`, `.PageTitle`, etc.).

## JavaScript DOM contract

If you override `static/comments.js`:

- [ ] The content template still has `<form id="comment-form" data-slug="…">`.
- [ ] The form has a `<textarea name="comment">`.
- [ ] `#comment-error` and `#comment-success` elements exist.
- [ ] `#comment-login-link` exists (shown to logged-out users).

If you override `static/auth.js`:

- [ ] The login template has `<form id="login-form">` with inputs named
      `username` and `password`.
- [ ] The register template has `<form id="register-form">` with inputs
      named `username`, `name`, `email`, and `password`.
- [ ] The forgot-password template has `<form id="forgot-form">` with an
      input named `email`.
- [ ] Each of the three templates has `#auth-error` and (for register and
      forgot) `#auth-success` elements.

If you override `static/verify-email.js`:

- [ ] `verify_email.html` has `#auth-error` and `#auth-success` elements.

If you override `static/reset-password.js`:

- [ ] `reset_password.html` has `<form id="reset-form">` with
      `<input id="new-password">`.
- [ ] `reset_password.html` has `#auth-error` and `#auth-success` elements.

If you override `static/nav-auth.js`:

- [ ] `layout.html` still has `<a id="nav-login">` and `<a id="nav-logout">`.
- [ ] If you keep the mobile nav toggle, `layout.html` still has
      `<button class="nav-toggle">` and `<nav class="site-nav">`.

If you override `static/search.js`:

- [ ] `layout.html` still has `<button class="search-toggle">`,
      `<input id="search-input">`, and `<div id="search-dropdown">`.

## CDN assets

- [ ] If your theme uses katex, `layout.html` loads
      `https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.css` and
      `katex.min.js`.
- [ ] If your theme uses highlight.js, `layout.html` loads the matching CSS
      and `<script src="/static/highlight.min.js"></script>`.
- [ ] If your theme does **not** use katex or highlight.js, the matching
      `<link>` and `<script>` tags have been removed from `layout.html`.

## Smoke test

- [ ] Run the page-by-page smoke test from
      [`page-render-smoke-test.md`](page-render-smoke-test.md) and confirm
      all 10 pages render correctly.

## Documentation

- [ ] `themes/<name>/CHANGELOG.md` records the Lesstruct version the theme
      was authored against.
- [ ] The CHANGELOG lists every file the theme overrides.
