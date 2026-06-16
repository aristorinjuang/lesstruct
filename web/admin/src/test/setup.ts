import { vi } from 'vitest'

/**
 * Global test setup.
 *
 * jsdom does not implement a number of layout-related DOM methods that
 * components may call (e.g. scrollIntoView). Stub them as no-ops so any
 * component that uses them can be mounted without throwing.
 */
if (typeof Element !== 'undefined' && typeof Element.prototype.scrollIntoView !== 'function') {
  Element.prototype.scrollIntoView = vi.fn()
}
