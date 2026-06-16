# Page Render Smoke Test

This is a manual smoke test for a Lesstruct theme. Run it after every change
to `themes/<name>/` and after every Lesstruct upgrade. The skill does not have
access to the Lesstruct source, so this test must be run by you, against your
running install, in a browser or with `curl`.

## Setup

1. Confirm `THEME_DIR` is set and Lesstruct is running.
2. Confirm at least one post exists (the smoke test uses `/<slug>`).
3. Confirm at least one user with author profile exists (for the author page).
4. Confirm at least one post has at least one tag (for the tag page).
5. Replace `<slug>`, `<username>`, and `<tag>` in the URLs below with values
   from your install.

## Page-by-page test

For each URL: load it in a browser (or `curl` it), then check the column on
the right. The "Smoke signal" column is the smallest signal that the page
rendered and your theme took effect.

| # | Page | URL | What to verify | Smoke signal |
|---|------|-----|----------------|--------------|
| 1 | Index | `GET /` | Post grid, navigation, search toggle, footer all styled per your theme | At least one `<article class="post-card">` is visible and styled differently from the default |
| 2 | Content | `GET /<slug>` | Body renders, tags link, author header, optional comments, language switcher | `<article class="content-article">` renders and the post body is present |
| 3 | Author | `GET /authors/<username>` | Avatar, name, custom fields, posts list | `<h1>` shows the author name |
| 4 | Tag | `GET /tags/<tag>` | Tag name heading, filtered posts | `<h1>` shows `#<tag>` |
| 5 | 404 | `GET /this-does-not-exist` | 404 status, custom 404 body | HTTP status is `404` and the body contains your custom 404 message |
| 6 | Login | `GET /login` | Form renders; `auth.js` toggles `#auth-error` on bad creds | `<form id="login-form">` renders; submitting bad creds shows the error element |
| 7 | Register | `GET /register` | Form renders; success message hides form on success | `<form id="register-form">` renders; submitting valid creds shows `#auth-success` |
| 8 | Forgot password | `GET /forgot-password` | Form renders; success message hides form on submit | `<form id="forgot-form">` renders; submitting shows `#auth-success` |
| 9 | Verify email | `GET /verify-email?token=<valid>` | Success message appears | `#auth-success` is visible after the request completes |
| 10 | Reset password | `GET /reset-password?token=<valid>` | Form renders; submit posts to `/api/auth/reset-password` | `<form id="reset-form">` renders and the `#new-password` input is present |

## Static asset checks

- [ ] `GET /static/style.css` returns your theme's CSS (or the embedded
      default if you did not override it).
- [ ] `GET /static/<file>` returns 200 for every file in your
      `themes/<name>/static/`.
- [ ] No `404` in the browser network panel for `/static/*` or
      `/api/v1/public/search`.

## Browser console checks

- [ ] No JavaScript errors in the browser console on any of the 10 pages.
- [ ] No 404s for fonts, images, or scripts.
- [ ] The mobile nav toggle (if your theme has one) opens and closes
      `.site-nav` when the `.nav-toggle` button is clicked.

## Quick command-line version

If you prefer to script the test:

```bash
BASE=http://localhost:8080

for path in / /authors/admin /tags/news /login /register /forgot-password; do
  status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE$path")
  echo "$status $path"
done

# 404 should return 404
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/this-does-not-exist"
```

Expected: every URL returns `200` except `/this-does-not-exist`, which returns
`404`.

## Reporting a failure

If a page fails the smoke test:

1. Check the [Troubleshooting](theme-development.md#troubleshooting) table in
   the loaded reference.
2. Check `themes/<name>/CHANGELOG.md` for the Lesstruct version you are
   running; if it is older than the theme, you may have hit a Lesstruct
   upgrade.
3. Re-run this skill with intent **repair after upgrade** — it will diff your
   theme against the new embedded defaults.
