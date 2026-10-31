import { ref, computed } from 'vue'
import request from '@/utils/request'

export interface SessionUser {
  id: string | number
  username: string
  role: string
}

// Legacy browser-session keys. The session now lives in the HttpOnly
// `lesstruct_jwt` cookie, so any JWT readable from script storage is stale —
// purge it on boot so a past XSS-readable token dies on first load.
const LEGACY_SESSION_KEYS = [
  'lesstruct_admin_token',
  'lesstruct_admin_role',
  'auth_token',
  'token',
  'user_role',
]

if (typeof localStorage !== 'undefined') {
  for (const key of LEGACY_SESSION_KEYS) {
    localStorage.removeItem(key)
  }
}

// Shared reactive session state — persists across all useAuth() calls.
// The identity is hydrated from /api/profile (cookie-authenticated), never
// parsed from a token.
const currentUser = ref<SessionUser | null>(null)
const sessionLoaded = ref(false)

// In-flight profile probe shared by concurrent guard calls so one navigation
// burst issues exactly one request.
let sessionPromise: Promise<SessionUser | null> | null = null

// Cross-tab session sync. The session itself lives in the HttpOnly cookie,
// which no tab can read — so tabs coordinate through a storage event instead:
// any login or logout writes this key, and every other tab drops its cached
// identity and re-probes on next use. The value is just a timestamp; it
// carries no session data.
const SESSION_EVENT_KEY = 'lesstruct_session_event'

if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event) => {
    if (event.key === SESSION_EVENT_KEY) {
      currentUser.value = null
      sessionLoaded.value = false
    }
  })
}

function broadcastSessionChange() {
  if (typeof localStorage !== 'undefined') {
    try {
      localStorage.setItem(SESSION_EVENT_KEY, String(Date.now()))
    } catch {
      // Storage unavailable: cross-tab sync is best-effort only.
    }
  }
}

export function setSession(user: SessionUser | null) {
  currentUser.value = user
  sessionLoaded.value = true
  broadcastSessionChange()
}

export function clearAuth() {
  currentUser.value = null
  // Forget the probe result too: the next guard re-probes the server, so a
  // failed server logout (cookie still alive) re-discovers the session
  // instead of showing a stale signed-out UI.
  sessionLoaded.value = false
  broadcastSessionChange()
}

async function loadSession(): Promise<SessionUser | null> {
  try {
    const response = await request.get<{
      data: { profile: { id: string | number; username: string; role: string } }
    }>('/api/profile')
    const profile = response.data?.data?.profile
    if (profile && profile.username && profile.role) {
      currentUser.value = { id: profile.id, username: profile.username, role: profile.role }
    } else {
      currentUser.value = null
    }
    sessionLoaded.value = true
  } catch (err) {
    currentUser.value = null
    // A 401 definitively means "no session" — cache it like a success so
    // anonymous browsing doesn't probe on every navigation. Any other
    // failure is transient: stay unloaded so the next navigation retries
    // instead of showing a signed-out UI for a live session.
    if ((err as { statusCode?: unknown } | null | undefined)?.statusCode === 401) {
      sessionLoaded.value = true
    }
  }
  return currentUser.value
}

export function refreshSession(): Promise<SessionUser | null> {
  if (sessionLoaded.value) {
    return Promise.resolve(currentUser.value)
  }
  if (sessionPromise) {
    return sessionPromise
  }
  sessionPromise = loadSession().finally(() => {
    sessionPromise = null
  })
  return sessionPromise
}

export function useAuth() {
  const userId = computed(() => currentUser.value?.id ?? null)

  const isAuthenticated = computed(() => currentUser.value !== null)

  const role = computed(() => currentUser.value?.role ?? null)

  return {
    user: currentUser,
    userId,
    isAuthenticated,
    role,
    sessionLoaded,
  }
}

/**
 * Get the stored session role without reactivity, for router guards.
 * Only meaningful after refreshSession() has resolved.
 */
export function getSessionRole(): string | null {
  return currentUser.value?.role ?? null
}

/**
 * Roles allowed to boot the admin SPA. A storefront session (e.g. a
 * Commentator from the shop) shares the same session cookie but must not
 * boot the admin shell: the router bounces it to the login view with a
 * message.
 */
export const ADMIN_BOOT_ROLES = new Set(['Admin', 'Contributor', 'Commentator'])

/**
 * Landing page for a role after login and for role-blind redirects (root,
 * unknown URLs, login visits). Admins get the dashboard, Commentators get
 * their profile — they manage no content and may not need the comment
 * system — and every other role keeps the content list.
 */
export function homePathFor(role: string | null | undefined): string {
  if (role === 'Admin') return '/dashboard'
  if (role === 'Commentator') return '/profile'
  return '/content?type=post'
}
