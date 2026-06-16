import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { useAuth, setAuthToken, getAuthStatus } from './useAuth'

// Helper component to test composable in proper Vue context
const TestComponent = defineComponent({
  setup() {
    const auth = useAuth()
    return { auth }
  },
  render() {
    return h('div', ['test'])
  },
})

function base64UrlEncode(str: string): string {
  return btoa(str)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '')
}

function createToken(payload: Record<string, unknown>): string {
  const header = { alg: 'HS256', typ: 'JWT' }
  return `${base64UrlEncode(JSON.stringify(header))}.${base64UrlEncode(JSON.stringify(payload))}.signature`
}

const now = Math.floor(Date.now() / 1000)

describe('useAuth', () => {
  beforeEach(() => {
    localStorage.clear()
    setAuthToken(null)
    vi.clearAllMocks()
  })

  const mountTestComponent = () => {
    return mount(TestComponent)
  }

  describe('JWT parsing with string user_id', () => {
    it('should parse numeric user_id from a fake JWT-shaped string', () => {
      // Fake JWT-shaped string used only to exercise the parser.
      // Signature is literally the string 'test' — not a signed token.
      const fakeBackendTokenLikeShape = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6ImFkbWluIiwicm9sZSI6IkFkbWluIiwiaWF0IjoxNzE0NTk2MDAwLCJleHAiOjE3MTQ2ODE2MDB9.test'

      localStorage.setItem('auth_token', fakeBackendTokenLikeShape)
      setAuthToken(fakeBackendTokenLikeShape)

      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.userId.value).toBe(1)
    })

    it('should handle string user_id in JWT payload', () => {
      const token = createToken({
        user_id: '123',
        username: 'testuser',
        role: 'Admin',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.userId.value).toBe(123)
    })

    it('should handle numeric user_id in JWT payload', () => {
      const token = createToken({
        user_id: 456,
        username: 'testuser',
        role: 'Admin',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.userId.value).toBe(456)
    })

    it('should not silently truncate mixed alphanumeric strings', () => {
      const token = createToken({
        user_id: '1abc',
        username: 'testuser',
        role: 'Admin',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      // Should return original string, not truncated number
      expect(wrapper.vm.auth.userId.value).toBe('1abc')
    })

    it('should fallback to sub when user_id is not present', () => {
      const token = createToken({
        sub: '789',
        username: 'testuser',
        role: 'Admin',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.userId.value).toBe(789)
    })

    it('should return null when neither user_id nor sub is present', () => {
      const token = createToken({
        username: 'testuser',
        role: 'Admin',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.userId.value).toBeNull()
    })
  })

  describe('token expiration', () => {
    it('should return false for isAuthenticated when token is expired', () => {
      const token = createToken({
        user_id: '1',
        username: 'testuser',
        iat: now - 7200,
        exp: now - 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)
    })

    it('should return true for isAuthenticated when token is valid', () => {
      const token = createToken({
        user_id: '1',
        username: 'testuser',
        iat: now,
        exp: now + 3600,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(true)
    })

    it('should consider token expired if less than 1 minute remaining', () => {
      const token = createToken({
        user_id: '1',
        username: 'testuser',
        iat: now,
        exp: now + 30,
      })

      setAuthToken(token)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)
    })
  })

  describe('setAuthToken', () => {
    it('should store token in localStorage', () => {
      const token = 'test-token'
      setAuthToken(token)

      expect(localStorage.getItem('auth_token')).toBe(token)
    })

    it('should remove token from localStorage when passed null', () => {
      localStorage.setItem('auth_token', 'test-token')
      setAuthToken(null)

      expect(localStorage.getItem('auth_token')).toBeNull()
    })
  })

  describe('getAuthStatus', () => {
    it('should return false when no token exists', () => {
      expect(getAuthStatus()).toBe(false)
    })

    it('should return true for valid token', () => {
      const token = createToken({
        user_id: '1',
        username: 'testuser',
        iat: now,
        exp: now + 3600,
      })

      localStorage.setItem('auth_token', token)

      expect(getAuthStatus()).toBe(true)
    })

    it('should return false for expired token', () => {
      const token = createToken({
        user_id: '1',
        username: 'testuser',
        iat: now - 7200,
        exp: now - 3600,
      })

      localStorage.setItem('auth_token', token)

      expect(getAuthStatus()).toBe(false)
    })
  })

  describe('storage event listener', () => {
    it('should update token when storage changes from another tab', async () => {
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.token.value).toBeNull()

      window.dispatchEvent(new StorageEvent('storage', {
        key: 'auth_token',
        newValue: 'new-token',
      }))

      await nextTick()
      expect(wrapper.vm.auth.token.value).toBe('new-token')
    })
  })
})
