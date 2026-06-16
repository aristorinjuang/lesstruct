import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import LoginView from './LoginView.vue'
import request, { ApiError } from '@/utils/request'

// Mock the request utility
vi.mock('@/utils/request', () => ({
  default: {
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

// Mock useAuth
const setAuthTokenMock = vi.fn()
const setUserRoleMock = vi.fn()
vi.mock('@/composables/useAuth', () => ({
  setAuthToken: (token: string) => setAuthTokenMock(token),
  setUserRole: (role: string) => setUserRoleMock(role),
}))

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', component: { template: '<div>Home</div>' } },
    { path: '/login', component: LoginView },
    { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
  ],
})

function flushPromises(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, 0))
}

describe('LoginView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
  })

  const mountWrapper = () => {
    return mount(LoginView, {
      global: {
        plugins: [router],
        stubs: {
          ThemeToggle: true,
        },
      },
    })
  }

  describe('successful login', () => {
    it('should call backend API with username and password', async () => {
      const mockResponse = {
        data: {
          data: {
            token: 'real-jwt-token',
            user: {
              id: '1',
              username: 'admin',
              role: 'Admin',
            },
          },
        },
      }
      vi.mocked(request.post).mockResolvedValue(mockResponse)

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      await flushPromises()

      expect(request.post).toHaveBeenCalledWith('/api/auth/login', {
        username: 'admin',
        password: 'password123',
      })
    })

    it('should store token and redirect to dashboard on successful login', async () => {
      const mockResponse = {
        data: {
          data: {
            token: 'real-jwt-token',
            user: {
              id: '1',
              username: 'admin',
              role: 'Admin',
            },
          },
        },
      }
      vi.mocked(request.post).mockResolvedValue(mockResponse)

      const wrapper = mountWrapper()
      const pushSpy = vi.spyOn(router, 'push')

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(setAuthTokenMock).toHaveBeenCalledWith('real-jwt-token')
      expect(setUserRoleMock).toHaveBeenCalledWith('Admin')
      expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    })

    it('should show loading state during login', async () => {
      vi.mocked(request.post).mockImplementation(
        () => new Promise(resolve => setTimeout(() => resolve({
          data: {
            data: {
              token: 'real-jwt-token',
              user: { id: '1', username: 'admin', role: 'Admin' },
            },
          },
        }), 100))
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      // Check loading state immediately
      const button = wrapper.find('.login-view__button')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.text()).toBe('Signing in...')

      // Wait for completion
      await new Promise(resolve => setTimeout(resolve, 150))
      await wrapper.vm.$nextTick()

      // Loading should be done
      expect(button.attributes('disabled')).toBeUndefined()
    })
  })

  describe('error handling', () => {
    it('should display error for invalid credentials (401 without code)', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401)
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('wrongpassword')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Invalid username or password')
    })

    it('should display specific message for INVALID_CREDENTIALS error code', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Invalid username or password', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('wrongpassword')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Invalid username or password')
    })

    it('should display specific message for ACCOUNT_LOCKED error code', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Account locked due to too many failed login attempts', 403, 'ACCOUNT_LOCKED')
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Account locked due to too many failed login attempts. Please try again later.')
    })

    it('should display error message for network errors', async () => {
      vi.mocked(request.post).mockRejectedValue(new TypeError('Failed to fetch'))

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toContain('Unable to connect to server')
    })

    it('should display error message for timeout (AbortError)', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new DOMException('The operation was aborted', 'AbortError')
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('password123')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toContain('Unable to connect to server')
    })

    it('should clear error message when user starts typing', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('wrongpassword')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.login-view__error').exists()).toBe(true)

      // Type in email field
      await wrapper.find('input[type="text"]').setValue('admin2')

      // Error should be cleared
      expect(wrapper.find('.login-view__error').exists()).toBe(false)
    })

    it('should reset isLoading after failed login', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await wrapper.find('input[type="text"]').setValue('admin')
      await wrapper.find('input[type="password"]').setValue('wrongpassword')
      await wrapper.find('form').trigger('submit')

      await flushPromises()
      await wrapper.vm.$nextTick()

      // After error, button should be re-enabled (isLoading reset by finally)
      const button = wrapper.find('.login-view__button')
      expect(button.attributes('disabled')).toBeUndefined()
      expect(button.text()).toBe('Sign in')

      // Error message should be displayed
      expect(wrapper.find('.login-view__error').exists()).toBe(true)
    })
  })

  describe('form validation', () => {
    it('should require email and password fields', async () => {
      const wrapper = mountWrapper()

      const emailInput = wrapper.find('input[type="text"]')
      const passwordInput = wrapper.find('input[type="password"]')

      expect(emailInput.attributes('required')).toBeDefined()
      expect(passwordInput.attributes('required')).toBeDefined()
    })

    it('should have correct autocomplete attributes', async () => {
      const wrapper = mountWrapper()

      const emailInput = wrapper.find('input[type="text"]')
      const passwordInput = wrapper.find('input[type="password"]')

      expect(emailInput.attributes('autocomplete')).toBe('username')
      expect(passwordInput.attributes('autocomplete')).toBe('current-password')
    })
  })
})
