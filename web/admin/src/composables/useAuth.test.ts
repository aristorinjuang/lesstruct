import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import {
  useAuth,
  setSession,
  clearAuth,
  refreshSession,
  getSessionRole,
  ADMIN_BOOT_ROLES,
  homePathFor,
} from './useAuth'
import request from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

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

const profileResponse = (role = 'Admin') => ({
  data: {
    data: {
      profile: { id: 1, username: 'admin', role },
    },
  },
})

describe('useAuth', () => {
  beforeEach(() => {
    localStorage.clear()
    clearAuth()
    vi.clearAllMocks()
  })

  const mountTestComponent = () => {
    return mount(TestComponent)
  }

  describe('script-readable storage', () => {
    it('never stores the session in localStorage', () => {
      setSession({ id: 1, username: 'admin', role: 'Admin' })

      expect(localStorage.getItem('lesstruct_admin_token')).toBeNull()
      expect(localStorage.getItem('lesstruct_admin_role')).toBeNull()
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('token')).toBeNull()
      expect(localStorage.getItem('user_role')).toBeNull()
    })
  })

  describe('session state', () => {
    it('starts unauthenticated without a session', () => {
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)
      expect(wrapper.vm.auth.userId.value).toBeNull()
      expect(wrapper.vm.auth.role.value).toBeNull()
    })

    it('derives identity from the session set at login', () => {
      setSession({ id: 1, username: 'admin', role: 'Admin' })
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(true)
      expect(wrapper.vm.auth.userId.value).toBe(1)
      expect(wrapper.vm.auth.role.value).toBe('Admin')
    })

    it('clears the session on logout', () => {
      setSession({ id: 1, username: 'admin', role: 'Admin' })
      clearAuth()
      const wrapper = mountTestComponent()

      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)
      expect(wrapper.vm.auth.role.value).toBeNull()
    })

    it('exposes the role for guards', () => {
      setSession({ id: '7', username: 'alice', role: 'Contributor' })

      expect(getSessionRole()).toBe('Contributor')
    })
  })

  describe('refreshSession', () => {
    it('hydrates the session from the profile probe', async () => {
      vi.mocked(request.get).mockResolvedValue(profileResponse())

      const user = await refreshSession()

      expect(request.get).toHaveBeenCalledWith('/api/profile')
      expect(user).toMatchObject({ username: 'admin', role: 'Admin' })

      const wrapper = mountTestComponent()
      expect(wrapper.vm.auth.isAuthenticated.value).toBe(true)
    })

    it('resolves to null when the probe is unauthorized', async () => {
      vi.mocked(request.get).mockRejectedValue(new Error('Unauthorized'))

      const user = await refreshSession()

      expect(user).toBeNull()

      const wrapper = mountTestComponent()
      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)
    })

    it('reuses a loaded session without probing again', async () => {
      vi.mocked(request.get).mockResolvedValue(profileResponse())
      await refreshSession()
      vi.mocked(request.get).mockClear()

      const user = await refreshSession()

      expect(request.get).not.toHaveBeenCalled()
      expect(user).toMatchObject({ username: 'admin' })
    })

    it('retries after a transient probe failure', async () => {
      vi.mocked(request.get).mockRejectedValueOnce(new TypeError('Failed to fetch'))
      vi.mocked(request.get).mockResolvedValue(profileResponse())

      expect(await refreshSession()).toBeNull()

      const user = await refreshSession()

      expect(request.get).toHaveBeenCalledTimes(2)
      expect(user).toMatchObject({ username: 'admin' })
    })

    it('caches a definitive 401 without probing again', async () => {
      vi.mocked(request.get).mockRejectedValue({ statusCode: 401 })

      expect(await refreshSession()).toBeNull()
      expect(await refreshSession()).toBeNull()

      expect(request.get).toHaveBeenCalledTimes(1)
    })
  })

  describe('cross-tab sync', () => {
    it('broadcasts login and logout for other tabs', async () => {
      const setItem = vi.spyOn(Storage.prototype, 'setItem')

      setSession({ id: 1, username: 'admin', role: 'Admin' })
      expect(setItem).toHaveBeenCalledWith('lesstruct_session_event', expect.any(String))

      clearAuth()
      expect(setItem).toHaveBeenCalledTimes(2)
    })

    it('drops the cached session when another tab changes it', async () => {
      vi.mocked(request.get).mockResolvedValue(profileResponse())
      await refreshSession()

      window.dispatchEvent(new StorageEvent('storage', { key: 'lesstruct_session_event' }))

      const wrapper = mountTestComponent()
      expect(wrapper.vm.auth.isAuthenticated.value).toBe(false)

      vi.mocked(request.get).mockClear()
      await refreshSession()
      expect(request.get).toHaveBeenCalledTimes(1)
    })
  })

  describe('ADMIN_BOOT_ROLES', () => {
    it('admits the staff roles', () => {
      expect(ADMIN_BOOT_ROLES.has('Admin')).toBe(true)
      expect(ADMIN_BOOT_ROLES.has('Contributor')).toBe(true)
      expect(ADMIN_BOOT_ROLES.has('Commentator')).toBe(true)
    })
  })

  describe('homePathFor', () => {
    it('sends admins to the dashboard', () => {
      expect(homePathFor('Admin')).toBe('/dashboard')
    })

    it('sends commentators to their profile', () => {
      expect(homePathFor('Commentator')).toBe('/profile')
    })

    it('sends content roles to the content list', () => {
      expect(homePathFor('Contributor')).toBe('/content?type=post')
    })

    it('falls back to the content list for unknown roles', () => {
      expect(homePathFor('Journalist')).toBe('/content?type=post')
    })
  })
})
