/**
 * convert.mjs — converts captured PNGs to WebP for the docs site.
 *
 * Why: PageSpeed flagged the screenshots as oversized image payloads. Playwright
 * can only emit PNG/JPEG, so every capture is written as PNG by the browser and
 * converted here. The PNG is deleted once the WebP exists.
 *
 * Usage:
 *   - Imported by capture.mjs right after each `page.screenshot` call.
 *   - Standalone batch mode (`node convert.mjs`) converts the existing files in
 *     site/static/screenshots/ and docs/assets/screenshots/, so switching the
 *     pipeline does not require re-capturing against a live instance.
 *
 * Exception: `og-card.png` stays PNG — it is the social-preview image, and PNG
 * is the safest format for link-preview scrapers.
 */

import sharp from 'sharp'
import { readdirSync, rmSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = resolve(__dirname, '..', '..')
const SITE_STATIC = join(REPO_ROOT, 'site', 'static', 'screenshots')
const DOCS_ASSETS = join(REPO_ROOT, 'docs', 'assets', 'screenshots')
const OG_IMAGE = 'og-card.png'

// Lossy at q=82 is visually indistinguishable from the source PNG for UI
// screenshots and cuts the payload by roughly 80%.
const WEBP_OPTIONS = { quality: 82, effort: 6 }

/**
 * Convert a PNG to WebP beside it, then delete the PNG.
 * Returns the path of the written `.webp` file.
 */
export async function pngToWebp(pngPath) {
  const webpPath = pngPath.replace(/\.png$/i, '.webp')
  await sharp(pngPath).webp(WEBP_OPTIONS).toFile(webpPath)
  rmSync(pngPath, { force: true })
  return webpPath
}

/** Batch-convert every PNG under the docs screenshot directories. */
async function convertAll() {
  let count = 0
  for (const dir of [SITE_STATIC, DOCS_ASSETS]) {
    let entries = []
    try {
      entries = readdirSync(dir)
    } catch {
      continue
    }
    for (const name of entries) {
      if (!name.toLowerCase().endsWith('.png') || name === OG_IMAGE) continue
      const pngPath = join(dir, name)
      const webpPath = await pngToWebp(pngPath)
      console.log(`  ${pngPath} -> ${webpPath}`)
      count++
    }
  }
  console.log(`\n  converted ${count} PNG(s) to WebP.\n`)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  convertAll().catch((err) => {
    console.error('\n[fatal]', err)
    process.exit(1)
  })
}
