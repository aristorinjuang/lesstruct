/**
 * capture.mjs — captures every scene in scenes.mjs in both light and dark
 * color schemes from a running Lesstruct instance.
 *
 * Flow:
 *   1. Read config from .env.local (this script does NOT inherit your shell
 *      env — .env.local is the single source of truth).
 *   2. Launch headless Chromium.
 *   3. Log into the admin SPA once; persist the session cookie to
 *      a storageState file reused by every admin scene.
 *   4. Discover dynamic URLs (published slugs from the content-site home;
 *      first content-edit URL from the admin content list).
 *   5. For each scene × {light, dark}: emulate the color scheme, navigate,
 *      settle, screenshot as PNG, convert to WebP (except the OG card). Write
 *      to site/static/screenshots/ and, for scenes flagged readme:true, mirror
 *      to docs/assets/screenshots/.
 *
 * Run via: `make screenshots` (which installs Playwright + chromium first).
 */

import { chromium } from 'playwright'
import { readFileSync, writeFileSync, mkdirSync, copyFileSync, existsSync, rmSync } from 'node:fs'
import { resolve, dirname, join, basename } from 'node:path'
import { fileURLToPath } from 'node:url'

import { scenes } from './scenes.mjs'
import { pngToWebp } from './convert.mjs'

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
  for (const k of ['LESSTRUCT_URL', 'LESSTRUCT_ADMIN_USERNAME', 'LESSTRUCT_ADMIN_PASSWORD', 'LESSTRUCT_API_KEY', 'MAILTRAP_API_TOKEN', 'MAILTRAP_SANDBOX_ID', 'MAILTRAP_ADMIN_EMAIL']) {
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

const MFA_CODE_RE = /verification code is:\s*(\d{6})/i
const MAILTRAP_MESSAGES_URL = (sandboxId) => `https://mailtrap.io/api/sandboxes/${sandboxId}/messages`

// fetchLatestMFACode polls the Mailtrap Email Sandbox for the newest login
// verification code email addressed to the admin, returning the 6-digit code.
// Returns null when Mailtrap is unconfigured or no fresh code arrives.
async function fetchLatestMFACode(cfg, afterId = 0, { tries = 12, intervalMs = 5000 } = {}) {
  if (!cfg.MAILTRAP_API_TOKEN || !cfg.MAILTRAP_SANDBOX_ID) return null
  const headers = { 'Api-Token': cfg.MAILTRAP_API_TOKEN }
  const adminEmail = (cfg.MAILTRAP_ADMIN_EMAIL || '').toLowerCase()
  for (let attempt = 0; attempt < tries; attempt++) {
    try {
      const res = await fetch(MAILTRAP_MESSAGES_URL(cfg.MAILTRAP_SANDBOX_ID), { headers })
      if (!res.ok) throw new Error(`Mailtrap API answered ${res.status}`)
      const messages = await res.json()
      const match = messages.find(
        (m) => m.id > afterId && (!adminEmail || (m.to_email || '').toLowerCase() === adminEmail) &&
          /verification code/i.test(m.subject || ''),
      )
      if (match) {
        const bodyRes = await fetch(
          `https://mailtrap.io/api/sandboxes/${cfg.MAILTRAP_SANDBOX_ID}/messages/${match.id}/body.txt`,
          { headers },
        )
        if (!bodyRes.ok) throw new Error(`Mailtrap body API answered ${bodyRes.status}`)
        const code = MFA_CODE_RE.exec(await bodyRes.text())?.[1]
        if (code) return code
      }
    } catch (err) {
      console.warn(`\n  [!] Mailtrap poll ${attempt + 1}/${tries} failed: ${err.message}`)
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return null
}

async function latestMessageId(cfg) {
  try {
    const res = await fetch(MAILTRAP_MESSAGES_URL(cfg.MAILTRAP_SANDBOX_ID), {
      headers: { 'Api-Token': cfg.MAILTRAP_API_TOKEN },
    })
    if (!res.ok) return 0
    const messages = await res.json()
    return messages.reduce((max, m) => Math.max(max, m.id || 0), 0)
  } catch {
    return 0
  }
}

async function login(browser, baseUrl, cfg) {
  const { LESSTRUCT_ADMIN_USERNAME: username, LESSTRUCT_ADMIN_PASSWORD: password } = cfg
  const context = await browser.newContext({ colorScheme: 'light', viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  try {
    // Snapshot the inbox first so a stale code from an earlier run can never
    // satisfy the poll below.
    const seenId = await latestMessageId(cfg)
    await page.goto(`${baseUrl}/admin/login`, { waitUntil: 'networkidle' })
    await page.fill('#username', username)
    await page.fill('#password', password)
    await page.click('button[type="submit"]')
    // Successful admin login lands on the dashboard. (If first-login setup is
    // incomplete this will time out — finish setup in a browser first.)
    //
    // Since two-step login, a correct password lands on the emailed-code step
    // instead: poll the Mailtrap sandbox for the fresh code and submit it.
    const codeInput = page.locator('#code')
    try {
      await codeInput.waitFor({ timeout: 15000 })
    } catch {
      const errText = await page.locator('.login-view__error').textContent().catch(() => null)
      console.warn(`\n  [!] MFA step never appeared (url=${page.url()} error=${errText || '—'})`)
      await page.waitForURL('**/admin/dashboard**', { timeout: 20000 })
      await page.waitForTimeout(1500)
      await mkdirSync(dirname(AUTH_FILE), { recursive: true })
      await context.storageState({ path: AUTH_FILE })
      return true
    }
    process.stdout.write('\n  password accepted, waiting for the emailed code ... ')
    const code = await fetchLatestMFACode(cfg, seenId)
    if (!code) throw new Error('no login verification code arrived in the Mailtrap sandbox')
    console.log('code received')
    await codeInput.fill(code)
    await page.click('button[type="submit"]')
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
  // API-based discovery: query the admin API (session cookie from
  // storageState rides the same-origin fetch automatically) for one
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
  const empty = { postSlug: '', pageSlug: '', customSlug: '', adminEditUrl: '', productEditUrl: '' }
  try {
    await page.goto(`${baseUrl}/admin/dashboard`, { waitUntil: 'networkidle' })
    await page.waitForTimeout(500)
    const result = await page.evaluate(async (builtin) => {
      const get = async (url) => {
        try {
          const res = await fetch(url)
          if (!res.ok) return {}
          return await res.json()
        } catch {
          return {}
        }
      }
      const [posts, pages, typesResp, products] = await Promise.all([
        get('/api/v1/content_items?post_type=post&limit=50'),
        get('/api/v1/content_items?post_type=page&limit=50'),
        get('/api/v1/post_types'),
        get('/api/v1/content_items?post_type=product&limit=50'),
      ])
      const pickPublished = (j) =>
        (j?.data || []).find((c) => c && c.status === 'published') || (j?.data || [])[0]
      const post = pickPublished(posts)
      const pageItem = pickPublished(pages)
      const product = pickPublished(products)
      const types = Array.isArray(typesResp?.data) ? typesResp.data : []
      const customType = types.find((t) => t && t.slug && !builtin.includes(t.slug) && t.slug !== 'product')
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
        productEditUrl: product?.id != null ? `/admin/content/${product.id}/edit` : '',
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
      if (scene.scrollSelector) {
        await page.waitForSelector(scene.scrollSelector, { timeout: 15000 }).catch(() => {})
        await page.evaluate(
          ([sel, dy]) => {
            document.querySelector(sel)?.scrollIntoView({ block: 'start' })
            if (dy) window.scrollBy(0, dy)
          },
          [scene.scrollSelector, scene.scrollOffset || 0],
        )
      }
      if (scene.waitMs) await page.waitForTimeout(scene.waitMs)

      const sitePath = join(SITE_STATIC, file)
      await page.screenshot({ path: sitePath, fullPage: !!scene.fullPage })

      // Playwright only emits PNG/JPEG; the docs site ships WebP. The OG card
      // stays PNG because link-preview scrapers handle it most reliably.
      const outputPath = scene.og ? sitePath : await pngToWebp(sitePath)

      if (scene.readme) {
        const docsPath = join(DOCS_ASSETS, basename(outputPath))
        copyFileSync(outputPath, docsPath)
      }
      console.log(`  - ${scene.name} [${mode}] -> ${basename(outputPath)}`)
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
  // Optional scene filter: `node capture.mjs --only=orders,payments` (or
  // ONLY=orders,payments) captures just those scenes, leaving the other
  // committed screenshots untouched.
  const onlyArg = process.argv.find((a) => a.startsWith('--only='))?.slice('--only='.length)
    || process.env.ONLY || ''
  const only = new Set(onlyArg.split(',').map((s) => s.trim()).filter(Boolean))
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
    await login(browser, baseUrl, cfg)
    console.log('ok')

    process.stdout.write('  discovering content + edit URL ... ')
    const discovered = await discover(browser, baseUrl)
    console.log(
      `ok (post:${discovered.postSlug || '—'} | page:${discovered.pageSlug || '—'} | custom:${discovered.customSlug || '—'} | edit:${discovered.adminEditUrl ? 'found' : 'MISSING'} | product:${discovered.productEditUrl ? 'found' : 'MISSING'})`,
    )

    const wanted = only.size ? scenes.filter((s) => only.has(s.name)) : scenes
    if (only.size) {
      const unknown = [...only].filter((n) => !scenes.some((s) => s.name === n))
      if (unknown.length) {
        console.error(`\n[fatal] unknown scene(s): ${unknown.join(', ')}`)
        process.exit(1)
      }
      console.log(`\n  capturing only: ${wanted.map((s) => s.name).join(', ')}`)
    }
    console.log('\n  capturing scenes:')
    for (const scene of wanted) {
      await captureScene(browser, baseUrl, scene, discovered)
    }
    console.log(`\n  done. WebP screenshots written to site/static/screenshots/ (and docs/assets/screenshots/ for README).\n`)
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
