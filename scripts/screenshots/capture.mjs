/**
 * capture.mjs — captures every scene in scenes.mjs in both light and dark
 * color schemes from a running Lesstruct instance.
 *
 * Flow:
 *   1. Read config from .env.local (this script does NOT inherit your shell
 *      env — .env.local is the single source of truth).
 *   2. Launch headless Chromium.
 *   3. Log into the admin SPA once; persist cookies + localStorage (JWT) to
 *      a storageState file reused by every admin scene.
 *   4. Discover dynamic URLs (published slugs from the content-site home;
 *      first content-edit URL from the admin content list).
 *   5. For each scene × {light, dark}: emulate the color scheme, navigate,
 *      settle, screenshot. Write to site/static/screenshots/ and, for scenes
 *      flagged readme:true, mirror to docs/assets/screenshots/.
 *
 * Run via: `make screenshots` (which installs Playwright + chromium first).
 */

import { chromium } from 'playwright'
import { readFileSync, writeFileSync, mkdirSync, copyFileSync, existsSync, rmSync } from 'node:fs'
import { resolve, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { scenes } from './scenes.mjs'

const __dirname = dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = resolve(__dirname, '..', '..')
const SITE_STATIC = join(REPO_ROOT, 'site', 'static', 'screenshots')
const DOCS_ASSETS = join(REPO_ROOT, 'docs', 'assets', 'screenshots')
const AUTH_FILE = join(__dirname, '.auth', 'admin.json')

const MODES = ['light', 'dark']

// Chromium resolves "localhost" inconsistently (it may try ::1 and refuse even
// when 127.0.0.1 works), and the Lesstruct admin SPA issues API requests to its
// own origin, so the browser must treat localhost as same-host. Forcing
// localhost -> 127.0.0.1 at the resolver layer makes both the page load and the
// SPA's API calls succeed against an IPv4-bound instance.
const LAUNCH_ARGS = ['--host-resolver-rules=MAP localhost 127.0.0.1']

// --- config ----------------------------------------------------------------

function loadEnvFile(cfg, envPath) {
  if (!existsSync(envPath)) return
  for (const line of readFileSync(envPath, 'utf8').split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    const eq = trimmed.indexOf('=')
    if (eq === -1) continue
    const key = trimmed.slice(0, eq).trim()
    const value = trimmed.slice(eq + 1).trim().replace(/^["']|["']$/g, '')
    cfg[key] = value
  }
}

function loadEnv() {
  // Read dotenv files (later sources override earlier), then let real process
  // env vars override — so `LESSTRUCT_URL=... node capture.mjs` also works.
  // Sources, in order: scripts/screenshots/.env.local, then repo-root .env.local.
  const cfg = {}
  loadEnvFile(cfg, join(__dirname, '.env.local'))
  loadEnvFile(cfg, join(REPO_ROOT, '.env.local'))
  for (const k of ['LESSTRUCT_URL', 'LESSTRUCT_ADMIN_USERNAME', 'LESSTRUCT_ADMIN_PASSWORD', 'LESSTRUCT_API_KEY']) {
    if (process.env[k]) cfg[k] = process.env[k]
  }
  if (!cfg.LESSTRUCT_URL && !cfg.LESSTRUCT_ADMIN_USERNAME) {
    console.error('\n[!] No config found. Either:')
    console.error('    - put .env.local in scripts/screenshots/ or the repo root, or')
    console.error('    - export LESSTRUCT_URL / LESSTRUCT_ADMIN_USERNAME / LESSTRUCT_ADMIN_PASSWORD in the environment.')
    process.exit(1)
  }
  return cfg
}

function check(cfg) {
  const missing = ['LESSTRUCT_URL', 'LESSTRUCT_ADMIN_USERNAME', 'LESSTRUCT_ADMIN_PASSWORD'].filter(
    (k) => !cfg[k],
  )
  if (missing.length) {
    console.error(`\n[!] Missing in .env.local: ${missing.join(', ')}\n`)
    process.exit(1)
  }
}

// --- login -----------------------------------------------------------------

async function login(browser, baseUrl, username, password) {
  const context = await browser.newContext({ colorScheme: 'light', viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  try {
    await page.goto(`${baseUrl}/admin/login`, { waitUntil: 'networkidle' })
    await page.fill('#username', username)
    await page.fill('#password', password)
    await page.click('button[type="submit"]')
    // Successful admin login lands on the dashboard. (If first-login setup is
    // incomplete this will time out — finish setup in a browser first.)
    await page.waitForURL('**/admin/dashboard**', { timeout: 20000 })
    await page.waitForTimeout(1500)
    await mkdirSync(dirname(AUTH_FILE), { recursive: true })
    await context.storageState({ path: AUTH_FILE })
    return true
  } finally {
    await context.close()
  }
}

// --- discovery -------------------------------------------------------------

const SKIP_PREFIXES = ['/admin', '/api', '/static', '/assets', '/favicon', '/robots', '/sitemap']
const BUILTIN_POST_TYPES = ['post', 'page', 'media', 'comment']

async function discover(browser, baseUrl) {
  // API-based discovery: query the admin API (JWT from storageState) for one
  // published slug of each post type the tour needs (post / page / a custom
  // type), plus the edit URL for the editor hero. The admin content list uses
  // programmatic router navigation, so slugs/IDs cannot be scraped from the DOM
  // reliably — the API is the stable source.
  const ctx = await browser.newContext({
    storageState: AUTH_FILE,
    colorScheme: 'light',
    viewport: { width: 1440, height: 900 },
  })
  const page = await ctx.newPage()
  const empty = { postSlug: '', pageSlug: '', customSlug: '', adminEditUrl: '' }
  try {
    await page.goto(`${baseUrl}/admin/dashboard`, { waitUntil: 'networkidle' })
    await page.waitForTimeout(500)
    const result = await page.evaluate(async (builtin) => {
      const token = localStorage.getItem('auth_token')
      if (!token) return {}
      const h = { Authorization: `Bearer ${token}` }
      const get = async (url) => {
        try {
          const res = await fetch(url, { headers: h })
          if (!res.ok) return {}
          return await res.json()
        } catch {
          return {}
        }
      }
      const [posts, pages, typesResp] = await Promise.all([
        get('/api/v1/content_items?post_type=post&limit=50'),
        get('/api/v1/content_items?post_type=page&limit=50'),
        get('/api/v1/post_types'),
      ])
      const pickPublished = (j) =>
        (j?.data || []).find((c) => c && c.status === 'published') || (j?.data || [])[0]
      const post = pickPublished(posts)
      const pageItem = pickPublished(pages)
      const types = Array.isArray(typesResp?.data) ? typesResp.data : []
      const customType = types.find((t) => t && t.slug && !builtin.includes(t.slug))
      let customItem
      if (customType) {
        const c = await get(`/api/v1/content_items?post_type=${encodeURIComponent(customType.slug)}&limit=50`)
        customItem = pickPublished(c)
      }
      return {
        postSlug: post?.slug || '',
        pageSlug: pageItem?.slug || '',
        customSlug: customItem?.slug || '',
        adminEditUrl: post?.id != null ? `/admin/content/${post.id}/edit` : '',
      }
    }, BUILTIN_POST_TYPES)
    return { ...empty, ...result }
  } finally {
    await ctx.close()
  }
}

// --- capture ---------------------------------------------------------------

async function captureScene(browser, baseUrl, scene, discovered) {
  const resolved = typeof scene.url === 'function' ? scene.url(discovered) : scene.url
  if (!resolved) {
    console.warn(`  - ${scene.name}: SKIPPED (no URL resolved; not enough discovered data)`)
    return
  }

  const targets = scene.og
    ? [{ mode: 'light', file: `${scene.name}.png` }] // OG card: single image, no suffix
    : scene.lightOnly
      ? [{ mode: 'light', file: `${scene.name}-light.png` }] // light-only theme: no dark variant
      : MODES.map((m) => ({ mode: m, file: `${scene.name}-${m}.png` }))

  for (const { mode, file } of targets) {
    const contextOptions = {
      colorScheme: mode,
      viewport: scene.viewport,
    }
    if (scene.kind === 'admin') {
      contextOptions.storageState = AUTH_FILE
    }
    const context = await browser.newContext(contextOptions)

    // For the admin SPA, force theme=system so it follows the emulated color
    // scheme (the app reads localStorage 'lesstruct-theme' on init).
    if (scene.kind === 'admin') {
      await context.addInitScript(() => {
        try {
          localStorage.setItem('lesstruct-theme', 'system')
        } catch {
          /* ignore */
        }
      })
    }

    const page = await context.newPage()
    try {
      await page.goto(`${baseUrl}${resolved}`, { waitUntil: 'networkidle', timeout: 30000 })
      if (scene.readySelector) {
        await page.waitForSelector(scene.readySelector, { timeout: 15000 }).catch(() => {})
      }
      if (scene.waitMs) await page.waitForTimeout(scene.waitMs)

      const sitePath = join(SITE_STATIC, file)
      await page.screenshot({ path: sitePath, fullPage: !!scene.fullPage })

      if (scene.readme) {
        const docsPath = join(DOCS_ASSETS, file)
        copyFileSync(sitePath, docsPath)
      }
      console.log(`  - ${scene.name} [${mode}] -> ${file}`)
    } catch (err) {
      console.warn(`  - ${scene.name} [${mode}]: FAILED (${err.message})`)
    } finally {
      await context.close()
    }
  }
}

// --- main ------------------------------------------------------------------

async function main() {
  const cfg = loadEnv()
  check(cfg)
  // Keep the URL as-is (the admin SPA calls its own origin, so the browser
  // origin must match). Chromium resolves "localhost" inconsistently, so we
  // force localhost -> 127.0.0.1 at the browser layer (see LAUNCH_ARGS); the
  // URL string itself stays on localhost to keep API calls same-origin.
  const baseUrl = cfg.LESSTRUCT_URL.replace(/\/$/, '')

  mkdirSync(SITE_STATIC, { recursive: true })
  mkdirSync(DOCS_ASSETS, { recursive: true })

  console.log(`\nLesstruct docs screenshot capture`)
  console.log(`  instance: ${baseUrl}`)
  console.log(`  output:   ${SITE_STATIC}\n`)

  const browser = await chromium.launch({ headless: true, args: LAUNCH_ARGS })

  try {
    process.stdout.write('  logging in as admin ... ')
    await login(browser, baseUrl, cfg.LESSTRUCT_ADMIN_USERNAME, cfg.LESSTRUCT_ADMIN_PASSWORD)
    console.log('ok')

    process.stdout.write('  discovering content + edit URL ... ')
    const discovered = await discover(browser, baseUrl)
    console.log(
      `ok (post:${discovered.postSlug || '—'} | page:${discovered.pageSlug || '—'} | custom:${discovered.customSlug || '—'} | edit:${discovered.adminEditUrl ? 'found' : 'MISSING'})`,
    )

    console.log('\n  capturing scenes:')
    for (const scene of scenes) {
      await captureScene(browser, baseUrl, scene, discovered)
    }
    console.log(`\n  done. PNGs written to site/static/screenshots/ (and docs/assets/screenshots/ for README).\n`)
  } finally {
    await browser.close()
    // Clean up the ephemeral auth state (contains the JWT).
    if (existsSync(AUTH_FILE)) rmSync(AUTH_FILE, { force: true })
  }
}

main().catch((err) => {
  console.error('\n[fatal]', err)
  process.exit(1)
})
