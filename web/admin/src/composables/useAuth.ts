import { ref, computed } from 'vue'

interface JwtPayload {
  sub?: string
  user_id?: string | number
  exp?: number
  iat?: number
}

/**
 * Parse JWT token without external library
 * @param token JWT token string
 * @returns Decoded payload or null if invalid
 */
function parseJwt(token: string): JwtPayload | null {
  try {
    const base64Url = token.split('.')[1]
    if (!base64Url) {
      return null
    }

    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )

    return JSON.parse(jsonPayload) as JwtPayload
  } catch {
    return null
  }
}

/**
 * Check if token is expired
 * @param payload Decoded JWT payload
 * @returns true if token is expired or will expire in less than 1 minute
 */
function isTokenExpired(payload: JwtPayload): boolean {
  if (!payload.exp) {
    return false
  }

  const now = Math.floor(Date.now() / 1000)
  const expiresInSeconds = payload.exp - now

  // Consider token expired if less than 1 minute remaining
  return expiresInSeconds < 60
}

// Shared reactive token state — persists across all useAuth() calls
const token = ref<string | null>(
  typeof localStorage !== 'undefined' ? localStorage.getItem('auth_token') : null
)

// Shared reactive user role state
const userRole = ref<string | null>(
  typeof localStorage !== 'undefined' ? localStorage.getItem('user_role') : null
)

// Listen for storage changes from other tabs/windows
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event) => {
    if (event.key === 'auth_token') {
      token.value = event.newValue
    }
    if (event.key === 'user_role') {
      userRole.value = event.newValue
    }
  })
}

export function setAuthToken(newToken: string | null) {
  if (newToken === null) {
    localStorage.removeItem('auth_token')
  } else {
    localStorage.setItem('auth_token', newToken)
  }
  token.value = newToken
}

export function setUserRole(role: string | null) {
  if (role === null) {
    localStorage.removeItem('user_role')
  } else {
    localStorage.setItem('user_role', role)
  }
  userRole.value = role
}

export function clearAuth() {
  setAuthToken(null)
  setUserRole(null)
}

export function useAuth() {
  const payload = computed(() => {
    if (!token.value) {
      return null
    }
    return parseJwt(token.value)
  })

  const userId = computed(() => {
    if (!payload.value) {
      return null
    }

    // Try user_id first, then sub (subject)
    const userIdValue = payload.value.user_id ?? payload.value.sub ?? null

    if (userIdValue === null) {
      return null
    }

    // Parse user_id as integer if it's a fully numeric string, otherwise return as-is
    if (typeof userIdValue === 'string') {
      const parsed = Number(userIdValue)
      return isFinite(parsed) && String(parsed) === userIdValue ? parsed : userIdValue
    }

    return userIdValue
  })

  const isAuthenticated = computed(() => {
    if (!token.value || !payload.value) {
      return false
    }

    return !isTokenExpired(payload.value)
  })

  const isTokenExpiredValue = computed(() => {
    if (!payload.value) {
      return true
    }
    return isTokenExpired(payload.value)
  })

  const role = computed(() => userRole.value)

  return {
    token,
    payload,
    userId,
    isAuthenticated,
    isTokenExpired: isTokenExpiredValue,
    role,
  }
}

/**
 * Get authentication status without reactivity
 * Useful for router guards where computed values aren't available
 * @returns true if authenticated with a valid non-expired token
 */
export function getAuthStatus(): boolean {
  const currentToken = typeof localStorage !== 'undefined' ? localStorage.getItem('auth_token') : null
  if (!currentToken) {
    return false
  }

  const payload = parseJwt(currentToken)
  if (!payload) {
    return false
  }

  return !isTokenExpired(payload)
}
