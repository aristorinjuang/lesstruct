/**
 * Scene registry for the docs screenshot pipeline.
 *
 * Each scene becomes two PNGs — <name>-light.png and <name>-dark.png—written
 * to site/static/screenshots/ (served at /screenshots/ by the docs site) and,
 * when `readme: true`, mirrored to docs/assets/screenshots/ for the GitHub
 * README's raw-image URLs.
 *
 * `url` may be a string (a path on the running instance) or a function of the
 * discovered data `{ postSlug, pageSlug, customSlug, adminEditUrl }` that
 * capture.mjs collects before capturing. Adding a scene = adding an entry here.
 *
 * `lightOnly: true` is for scenes whose target has no dark mode (the content
 * site's default theme is light-only). The light shot is captured once and
 * copied to the -dark filename so the screenshot shortcode still resolves.
 *
 * @typedef {{ postSlug: string, pageSlug: string, customSlug: string, adminEditUrl: string }} Discovered
 */

const ADMIN_VIEWPORT = { width: 1440, height: 900 }
const PUBLIC_VIEWPORT = { width: 1440, height: 900 }

export const scenes = [
  // --- Admin SPA scenes (authenticated; admin has a real dark mode) --------

  {
    name: 'admin-hero',
    kind: 'admin',
    readme: true,
    viewport: ADMIN_VIEWPORT,
    fullPage: false,
    readySelector: '.ProseMirror',
    waitMs: 1500,
    /** @param {Discovered} d */
    url: (d) => d.adminEditUrl,
    note: 'Populated content editor (rich text, custom fields, SEO). Homepage hero + README + features.md.',
  },

  {
    name: 'dashboard',
    kind: 'admin',
    readme: true,
    viewport: ADMIN_VIEWPORT,
    fullPage: false,
    waitMs: 1500,
    url: '/admin/dashboard',
    note: 'Admin dashboard. features.md (engagement) + README.',
  },

  {
    name: 'media-library',
    kind: 'admin',
    viewport: ADMIN_VIEWPORT,
    fullPage: false,
    waitMs: 1500,
    url: '/admin/media',
    note: 'Media library. features.md (media).',
  },

  {
    name: 'api-keys',
    kind: 'admin',
    viewport: ADMIN_VIEWPORT,
    fullPage: false,
    waitMs: 1500,
    url: '/admin/profile/api-keys',
    note: 'API keys management. features.md (api).',
  },

  // --- Public content-site scenes (unauthenticated; light-only theme) ------
  // lightOnly: the default content theme has no dark variant, so the dark PNG
  // is a copy of the light one (keeps the screenshot shortcode's contract).

  {
    name: 'content-site-hero',
    kind: 'public',
    readme: true,
    lightOnly: true,
    viewport: PUBLIC_VIEWPORT,
    fullPage: true,
    waitMs: 800,
    url: '/',
    note: 'Content-site home (posts grid). README.',
  },

  {
    name: 'tour-blog',
    kind: 'public',
    lightOnly: true,
    viewport: PUBLIC_VIEWPORT,
    fullPage: true,
    waitMs: 800,
    /** @param {Discovered} d */
    url: (d) => (d.postSlug ? `/${d.postSlug}` : null),
    note: 'A published blog post (post_type=post). tour.md.',
  },

  {
    name: 'tour-page',
    kind: 'public',
    lightOnly: true,
    viewport: PUBLIC_VIEWPORT,
    fullPage: true,
    waitMs: 800,
    /** @param {Discovered} d */
    url: (d) => (d.pageSlug ? `/${d.pageSlug}` : null),
    note: 'A published static page (post_type=page). tour.md.',
  },

  {
    name: 'tour-custom',
    kind: 'public',
    lightOnly: true,
    viewport: PUBLIC_VIEWPORT,
    fullPage: true,
    waitMs: 800,
    /** @param {Discovered} d */
    url: (d) => (d.customSlug ? `/${d.customSlug}` : null),
    note: 'A published custom-post-type item (first non-builtin type). tour.md.',
  },

  // --- Social card ----------------------------------------------------------

  {
    name: 'og-card',
    kind: 'public',
    readme: false,
    // og-card is special: single PNG (no -light/-dark suffix), 1200x630.
    og: true,
    viewport: { width: 1200, height: 630 },
    fullPage: false,
    waitMs: 800,
    url: '/',
    note: 'OpenGraph card (1200x630). inject/head.html references /screenshots/og-card.png.',
  },
]
