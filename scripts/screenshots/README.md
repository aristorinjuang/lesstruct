# Docs screenshot pipeline

Captures Lesstruct admin-panel and content-site screenshots (light + dark) for
the docs site (`site/static/screenshots/`, served at `/screenshots/`) and the
GitHub `README.md` (`docs/assets/screenshots/` mirror).

## One-time setup

```bash
# from the repo root
make screenshots-install      # npm install + npx playwright install chromium
cp scripts/screenshots/.env.local.example scripts/screenshots/.env.local
$EDITOR scripts/screenshots/.env.local   # set URL + admin username/password
```

The capture script reads **only** from `.env.local` — it does not inherit your
shell env. `.env.local` is gitignored; never commit real credentials.

## Capturing

Make sure a Lesstruct instance with sample content is running at the URL in
`.env.local`, then:

```bash
make screenshots             # runs scripts/screenshots/capture.mjs
```

Every scene is captured in both light and dark (except `og-card`, a single
image). Output is written to `site/static/screenshots/` and, for scenes flagged
`readme: true`, mirrored to `docs/assets/screenshots/`.

## Adding a scene

Edit `scenes.mjs` and add an entry:

```js
{
  name: 'my-scene',
  kind: 'admin',            // or 'public'
  readme: false,            // mirror to docs/assets/ for README?
  viewport: { width: 1440, height: 900 },
  fullPage: false,
  readySelector: '.ProseMirror',  // optional: CSS to wait for
  waitMs: 1500,             // extra settle time after networkidle
  url: '/admin/some-view',  // or (d) => `/path/${d.publicSlugs[0]}`
  note: 'What this scene shows.',
}
```

Then reference it in a docs page with the `screenshot` shortcode:

```
{{< screenshot src="my-scene" alt="..." caption="..." >}}
```

The shortcode fails the docs build if the light PNG is missing, so a referenced
but uncaptured scene cannot ship by accident. When a `-dark.png` is also present
the shortcode emits a `<picture>` with a dark variant; when it is absent (the
content site's default theme is light-only) it emits a plain `<img>`. Set
`lightOnly: true` on a scene to capture light only.

## How it works

- **Auth.** Logs into the admin SPA once (`/admin/login`) and persists the
  storage state (cookies + localStorage JWT) to `.auth/admin.json`, which is
  deleted at the end of the run. The JWT is never written anywhere else.
- **Color schemes.** Emulates `prefers-color-scheme` per context. For the admin
  SPA, an init script sets `localStorage['lesstruct-theme'] = 'system'` so the
  app's theme follows the emulated scheme; the content site responds to the CSS
  media query directly.
- **Discovery.** Published slugs are scraped from the content-site home (`/`);
  the first content-edit URL is scraped from `/admin/content?type=post`. Scene
  URLs can be functions of this discovered data, so the pipeline does not hardcode slugs or IDs.
- **Moderation scenes.** Not included by default because they need data in a
  non-clean state. Add them later as a `kind: 'admin'` scene with a `setup`
  hook that seeds + cleans up via the API.

## Version pin

The captured PNGs record whatever the running instance renders. When the admin
UI or default theme changes meaningfully, re-run `make screenshots` against an
up-to-date instance and commit the new PNGs in the same change as the docs
update (per `AGENTS.md`).
