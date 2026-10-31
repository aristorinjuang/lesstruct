import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import type { Router } from 'vue-router'

vi.mock('@/composables/useAuth', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/composables/useAuth')>()
  return {
    ...actual,
    refreshSession: vi.fn(),
  }
})

vi.mock('@/composables/useConfig', () => ({
  useConfig: () => ({
    commentsEnabled: { value: true },
    fetchConfig: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    postWithTimeout: vi.fn(),
  },
  ApiError: class ApiError extends Error {
    statusCode: number
    code?: string
    constructor(message: string, statusCode: number, code?: string) {
      super(message)
      this.name = 'ApiError'
      this.statusCode = statusCode
      this.code = code
    }
  },
}))

// NOTE: the router and the mocked auth module must be imported dynamically.
// Static top-level imports deadlock worker collection against the async
// useAuth mock factory above.
let router: Router
let refreshSession: Mock

const adminSession = { id: '1', username: 'admin', role: 'Admin' }
const commentatorSession = { id: '7', username: 'cam', role: 'Commentator' }

describe('router boot guard', () => {
  beforeEach(async () => {
    router = (await import('./index')).default
    const auth = await import('@/composables/useAuth')
    refreshSession = vi.mocked(auth.refreshSession)
    refreshSession.mockReset()
    sessionStorage.clear()
    // Skip the first-login bootstrap check: it is orthogonal to the boot guard.
    sessionStorage.setItem('first_login_complete', 'true')
    // Reset to a public route without tripping guards.
    refreshSession.mockResolvedValue(null)
    await router.push('/login')
  })

  it('bounces a stale non-admin role to login with the storefront reason', async () => {
    refreshSession.mockResolvedValue({ id: '9', username: 'subscriber', role: 'Subscriber' })

    await router.push('/dashboard')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.reason).toBe('storefront-session')
  })

  it('lets a boot role through to the protected route', async () => {
    refreshSession.mockResolvedValue(adminSession)

    await router.push('/dashboard')

    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('still sends unauthenticated visitors to login without a reason', async () => {
    refreshSession.mockResolvedValue(null)

    await router.push('/dashboard')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.reason).toBeUndefined()
  })

  it('sends an authenticated boot role away from a plain login visit', async () => {
    // The storefront-session landing is exempt from this bounce so the
    // stale session can reach the login view and be cleared there.
    refreshSession.mockResolvedValue(adminSession)
    await router.push('/forgot-password')

    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('redirects an unknown sub-route to the dashboard for a boot role', async () => {
    refreshSession.mockResolvedValue(adminSession)

    await router.push('/content/2')

    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('lands a commentator on their profile from the root redirect', async () => {
    refreshSession.mockResolvedValue(commentatorSession)

    await router.push('/')

    expect(router.currentRoute.value.path).toBe('/profile')
  })

  it('keeps a contributor on the content list from the root redirect', async () => {
    refreshSession.mockResolvedValue({ id: '2', username: 'ed', role: 'Contributor' })

    await router.push('/')

    expect(router.currentRoute.value.path).toBe('/content')
    expect(router.currentRoute.value.query.type).toBe('post')
  })

  it('redirects an unknown sub-route to the commentator home', async () => {
    refreshSession.mockResolvedValue(commentatorSession)

    await router.push('/content/2')

    expect(router.currentRoute.value.path).toBe('/profile')
  })

  it('sends an authenticated commentator away from a plain login visit to their profile', async () => {
    refreshSession.mockResolvedValue(commentatorSession)
    await router.push('/forgot-password')

    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/profile')
  })

  it('sends an unauthenticated visitor on an unknown sub-route to login', async () => {
    refreshSession.mockResolvedValue(null)

    await router.push('/content/2')

    expect(router.currentRoute.value.path).toBe('/login')
  })
})
