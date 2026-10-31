import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import LoginView from './LoginView.vue'
import request, { ApiError } from '@/utils/request'

// Mock the request utility
vi.mock('@/utils/request', () => ({
  default: {
    post: vi.fn(),
    postWithTimeout: vi.fn(),
    get: vi.fn(),
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
const setSessionMock = vi.fn()
const clearAuthMock = vi.fn()
const getSessionRoleMock = vi.fn<() => string | null>()
const refreshSessionMock = vi.fn()
vi.mock('@/composables/useAuth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/composables/useAuth')>()
  return {
    ...actual,
    setSession: (user: unknown) => setSessionMock(user),
    clearAuth: () => clearAuthMock(),
    getSessionRole: () => getSessionRoleMock(),
    refreshSession: () => refreshSessionMock(),
  }
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', component: { template: '<div>Home</div>' } },
    { path: '/login', component: LoginView },
    { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
  ],
})

const challengeResponse = {
  data: {
    data: {
      mfaRequired: true,
      challengeToken: 'challenge-token',
      emailHint: 'a***@example.com',
      expiresIn: 300,
    },
  },
}

const sessionResponse = {
  data: {
    data: {
      user: {
        id: '1',
        username: 'admin',
        role: 'Admin',
      },
    },
  },
}

function flushPromises(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, 0))
}

async function submitCredentials(
  wrapper: ReturnType<typeof mount>,
  password = 'password123'
): Promise<void> {
  await wrapper.find('input#username').setValue('admin')
  await wrapper.find('input#password').setValue(password)
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  await wrapper.vm.$nextTick()
}

async function submitCode(
  wrapper: ReturnType<typeof mount>,
  code = '123456'
): Promise<void> {
  await wrapper.find('input#code').setValue(code)
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  await wrapper.vm.$nextTick()
}

describe('LoginView', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    getSessionRoleMock.mockReturnValue(null)
    refreshSessionMock.mockResolvedValue(null)
  })

  afterEach(() => {
    vi.useRealTimers()
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

  describe('two-step login', () => {
    it('explains the bounce when a storefront session lands on login', async () => {
      await router.push('/login?reason=storefront-session')
      await router.isReady()

      const wrapper = mountWrapper()
      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(clearAuthMock).toHaveBeenCalled()
      expect(wrapper.find('.login-view__notice').exists()).toBe(true)
      expect(wrapper.text()).toContain('That session belongs to the storefront.')

      wrapper.unmount()
      await router.push('/login')
      await router.isReady()
    })

    it('keeps a valid boot-role session instead of clearing it on a crafted reason', async () => {
      getSessionRoleMock.mockReturnValue('Admin')
      refreshSessionMock.mockResolvedValue({ id: '1', username: 'admin', role: 'Admin' })
      await router.push('/login?reason=storefront-session')
      await router.isReady()
      const replaceSpy = vi.spyOn(router, 'replace')

      const wrapper = mountWrapper()
      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(clearAuthMock).not.toHaveBeenCalled()
      expect(wrapper.find('.login-view__notice').exists()).toBe(false)
      expect(replaceSpy).toHaveBeenCalledWith('/dashboard')

      wrapper.unmount()
      await router.push('/login')
      await router.isReady()
    })

    it('returns a commentator to their profile on a crafted reason', async () => {
      getSessionRoleMock.mockReturnValue('Commentator')
      refreshSessionMock.mockResolvedValue({ id: '7', username: 'cam', role: 'Commentator' })
      await router.push('/login?reason=storefront-session')
      await router.isReady()
      const replaceSpy = vi.spyOn(router, 'replace')

      const wrapper = mountWrapper()
      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(clearAuthMock).not.toHaveBeenCalled()
      expect(wrapper.find('.login-view__notice').exists()).toBe(false)
      expect(replaceSpy).toHaveBeenCalledWith('/profile')

      wrapper.unmount()
      await router.push('/login')
      await router.isReady()
    })

    it('should call backend API with username and password', async () => {
      vi.mocked(request.post).mockResolvedValue(challengeResponse)

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      expect(request.post).toHaveBeenCalledWith('/api/auth/login', {
        username: 'admin',
        password: 'password123',
      })

      wrapper.unmount()
    })

    it('should route the seeded admin to first-login setup without a session', async () => {
      vi.mocked(request.post).mockResolvedValue({
        data: { data: { firstLoginRequired: true } },
      })

      const wrapper = mountWrapper()
      const pushSpy = vi.spyOn(router, 'push')

      await submitCredentials(wrapper, 'admin')

      expect(setSessionMock).not.toHaveBeenCalled()
      expect(pushSpy).toHaveBeenCalledWith('/first-login')
      expect(sessionStorage.getItem('first_login_complete')).toBe('false')

      wrapper.unmount()
    })

    it('should show the code step instead of issuing a token after the password step', async () => {
      vi.mocked(request.post).mockResolvedValue(challengeResponse)

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      expect(wrapper.find('input#code').exists()).toBe(true)
      expect(wrapper.find('input#password').exists()).toBe(false)
      expect(wrapper.text()).toContain('a***@example.com')
      expect(setSessionMock).not.toHaveBeenCalled()

      wrapper.unmount()
    })

    it('should refuse a session token returned by the password step', async () => {
      // A token on the password step would mean the server skipped the
      // second factor: the client must refuse it, never store it.
      vi.mocked(request.post).mockResolvedValue({
        data: {
          data: {
            token: 'rogue-jwt',
            user: { id: '1', username: 'admin', role: 'Admin' },
          },
        },
      })

      const wrapper = mountWrapper()
      const pushSpy = vi.spyOn(router, 'push')

      await submitCredentials(wrapper)

      expect(setSessionMock).not.toHaveBeenCalled()
      expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
      expect(wrapper.find('input#username').exists()).toBe(true)
      expect(wrapper.find('.login-view__error').text()).toBe('Login failed. Please try again.')

      wrapper.unmount()
    })

    it('should store the session and redirect to dashboard after verifying the code', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockResolvedValueOnce(sessionResponse)

      const wrapper = mountWrapper()
      const pushSpy = vi.spyOn(router, 'push')

      await submitCredentials(wrapper)
      await submitCode(wrapper)

      expect(request.post).toHaveBeenCalledWith('/api/auth/mfa/verify', {
        challengeToken: 'challenge-token',
        code: '123456',
      })
      expect(setSessionMock).toHaveBeenCalledWith({ id: '1', username: 'admin', role: 'Admin' })
      expect(pushSpy).toHaveBeenCalledWith('/dashboard')

      wrapper.unmount()
    })

    it('should store the session and redirect a commentator to their profile', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockResolvedValueOnce({
          data: {
            data: {
              user: { id: '7', username: 'cam', role: 'Commentator' },
            },
          },
        })

      const wrapper = mountWrapper()
      const pushSpy = vi.spyOn(router, 'push')

      await submitCredentials(wrapper)
      await submitCode(wrapper)

      expect(setSessionMock).toHaveBeenCalledWith({ id: '7', username: 'cam', role: 'Commentator' })
      expect(pushSpy).toHaveBeenCalledWith('/profile')

      wrapper.unmount()
    })

    it('should request a new code after the cooldown and keep the challenge', async () => {
      vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockResolvedValueOnce({
          data: {
            data: {
              message: 'A new verification code has been sent.',
              challengeToken: 'challenge-token-2',
              emailHint: 'a***@example.com',
              expiresIn: 300,
            },
          },
        })
        .mockResolvedValueOnce(sessionResponse)

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const resendButton = wrapper.findAll('.login-view__actions button')[0]!
      expect(resendButton.attributes('disabled')).toBeDefined()

      vi.advanceTimersByTime(60000)
      await wrapper.vm.$nextTick()

      expect(resendButton.attributes('disabled')).toBeUndefined()

      await resendButton.trigger('click')
      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(request.post).toHaveBeenCalledWith('/api/auth/mfa/resend', {
        challengeToken: 'challenge-token',
      })

      // The resend replaces the challenge: the next verify must use the
      // rotated token, not the original one.
      await submitCode(wrapper)
      expect(request.post).toHaveBeenLastCalledWith('/api/auth/mfa/verify', {
        challengeToken: 'challenge-token-2',
        code: '123456',
      })

      wrapper.unmount()
    })

    it('should return to the password step when a resend cannot be delivered', async () => {
      vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockRejectedValueOnce(
          new ApiError(
            'We could not send a new code. Please sign in again.',
            503,
            'MFA_UNAVAILABLE'
          )
        )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const resendButton = wrapper.findAll('.login-view__actions button')[0]!
      vi.advanceTimersByTime(60000)
      await wrapper.vm.$nextTick()

      await resendButton.trigger('click')
      await flushPromises()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('input#code').exists()).toBe(false)
      expect(wrapper.find('input#username').exists()).toBe(true)
      expect(wrapper.find('.login-view__error').text()).toBe(
        'We could not send your verification code. Please try again.'
      )

      wrapper.unmount()
    })

    it('should return to the password step when the challenge is dead', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockRejectedValueOnce(
          new ApiError(
            'Your verification session is no longer valid',
            401,
            'MFA_CHALLENGE_INVALID'
          )
        )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)
      await submitCode(wrapper)

      expect(wrapper.find('input#code').exists()).toBe(false)
      expect(wrapper.find('input#username').exists()).toBe(true)
      expect(wrapper.find('.login-view__error').text()).toBe(
        'Your verification session is no longer valid. Please sign in again.'
      )

      wrapper.unmount()
    })

    it('should return to the password step when the attempt budget is exhausted', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockRejectedValueOnce(
          new ApiError('Too many invalid attempts', 401, 'MFA_MAX_ATTEMPTS')
        )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)
      await submitCode(wrapper)

      expect(wrapper.find('input#code').exists()).toBe(false)
      expect(wrapper.find('input#username').exists()).toBe(true)

      wrapper.unmount()
    })

    it('should show an error and stay on the code step for a wrong code', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockRejectedValueOnce(
          new ApiError('Invalid verification code', 401, 'MFA_CODE_INVALID')
        )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)
      await submitCode(wrapper, '000000')

      expect(wrapper.find('input#code').exists()).toBe(true)
      expect(wrapper.find('.login-view__error').text()).toBe(
        'Invalid verification code. Please try again.'
      )

      wrapper.unmount()
    })

    it('should let the user return to the password step', async () => {
      vi.mocked(request.post).mockResolvedValue(challengeResponse)

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const backButton = wrapper.findAll('.login-view__actions button')[1]!
      await backButton.trigger('click')

      expect(wrapper.find('input#username').exists()).toBe(true)
      expect(wrapper.find('input#code').exists()).toBe(false)

      wrapper.unmount()
    })

    it('should display an error when the server cannot send the code', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Verification unavailable', 503, 'MFA_UNAVAILABLE')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      expect(wrapper.find('.login-view__error').text()).toBe(
        'We could not send your verification code. Please try again.'
      )

      wrapper.unmount()
    })
  })

  describe('loading states', () => {
    it('should show loading state during the password step', async () => {
      vi.mocked(request.post).mockImplementation(
        () =>
          new Promise(resolve =>
            setTimeout(
              () =>
                resolve({
                  data: {
                    data: {
                      mfaRequired: true,
                      challengeToken: 'challenge-token',
                      emailHint: 'a***@example.com',
                    },
                  },
                }),
              100
            )
          )
      )

      const wrapper = mountWrapper()

      await wrapper.find('input#username').setValue('admin')
      await wrapper.find('input#password').setValue('password123')
      await wrapper.find('form').trigger('submit')

      // Check loading state immediately
      const button = wrapper.find('.login-view__button')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.text()).toBe('Signing in...')

      // Wait for completion
      await new Promise(resolve => setTimeout(resolve, 150))
      await wrapper.vm.$nextTick()

      // Loading should be done and the code step visible
      expect(wrapper.find('input#code').exists()).toBe(true)

      wrapper.unmount()
    })

    it('should show loading state during the code step', async () => {
      vi.mocked(request.post)
        .mockResolvedValueOnce(challengeResponse)
        .mockImplementationOnce(
          () =>
            new Promise(resolve =>
              setTimeout(() => resolve(sessionResponse), 100)
            )
        )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      await wrapper.find('input#code').setValue('123456')
      await wrapper.find('form').trigger('submit')

      const button = wrapper.find('.login-view__button')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.text()).toBe('Verifying...')

      await new Promise(resolve => setTimeout(resolve, 150))
      await wrapper.vm.$nextTick()

      expect(setSessionMock).toHaveBeenCalledWith({ id: '1', username: 'admin', role: 'Admin' })

      wrapper.unmount()
    })
  })

  describe('error handling', () => {
    it('should display error for invalid credentials (401 without code)', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401)
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper, 'wrongpassword')

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Invalid username or password')
    })

    it('should display specific message for INVALID_CREDENTIALS error code', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Invalid username or password', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper, 'wrongpassword')

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Invalid username or password')
    })

    it('should display specific message for ACCOUNT_LOCKED error code', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Account locked due to too many failed login attempts', 403, 'ACCOUNT_LOCKED')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toBe('Account locked due to too many failed login attempts. Please try again later.')
    })

    it('should display error message for network errors', async () => {
      vi.mocked(request.post).mockRejectedValue(new TypeError('Failed to fetch'))

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toContain('Unable to connect to server')
    })

    it('should display error message for timeout (AbortError)', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new DOMException('The operation was aborted', 'AbortError')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const errorElement = wrapper.find('.login-view__error')
      expect(errorElement.exists()).toBe(true)
      expect(errorElement.text()).toContain('Unable to connect to server')
    })

    it('should clear error message when user starts typing', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper, 'wrongpassword')

      expect(wrapper.find('.login-view__error').exists()).toBe(true)

      // Type in the username field
      await wrapper.find('input#username').setValue('admin2')

      // Error should be cleared
      expect(wrapper.find('.login-view__error').exists()).toBe(false)
    })

    it('should reset isLoading after failed login', async () => {
      vi.mocked(request.post).mockRejectedValue(
        new ApiError('Unauthorized', 401, 'INVALID_CREDENTIALS')
      )

      const wrapper = mountWrapper()

      await submitCredentials(wrapper, 'wrongpassword')

      // After error, button should be re-enabled (isLoading reset by finally)
      const button = wrapper.find('.login-view__button')
      expect(button.attributes('disabled')).toBeUndefined()
      expect(button.text()).toBe('Sign in')

      // Error message should be displayed
      expect(wrapper.find('.login-view__error').exists()).toBe(true)
    })
  })

  describe('form validation', () => {
    it('should require username and password fields', async () => {
      const wrapper = mountWrapper()

      const usernameInput = wrapper.find('input#username')
      const passwordInput = wrapper.find('input#password')

      expect(usernameInput.attributes('required')).toBeDefined()
      expect(passwordInput.attributes('required')).toBeDefined()
    })

    it('should have correct autocomplete attributes on the password step', async () => {
      vi.mocked(request.post).mockResolvedValue(challengeResponse)

      const wrapper = mountWrapper()

      const usernameInput = wrapper.find('input#username')
      const passwordInput = wrapper.find('input#password')

      expect(usernameInput.attributes('autocomplete')).toBe('username')
      expect(passwordInput.attributes('autocomplete')).toBe('current-password')

      wrapper.unmount()
    })

    it('should mark the code input for one-time-code autofill', async () => {
      vi.mocked(request.post).mockResolvedValue(challengeResponse)

      const wrapper = mountWrapper()

      await submitCredentials(wrapper)

      const codeInput = wrapper.find('input#code')
      expect(codeInput.attributes('required')).toBeDefined()
      expect(codeInput.attributes('autocomplete')).toBe('one-time-code')
      expect(codeInput.attributes('inputmode')).toBe('numeric')
      expect(codeInput.attributes('maxlength')).toBe('6')

      wrapper.unmount()
    })
  })
})
