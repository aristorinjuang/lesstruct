<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearAuth, getSessionRole, setSession, refreshSession, ADMIN_BOOT_ROLES, homePathFor } from '@/composables/useAuth'
import request, { ApiError } from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import ThemeToggle from '@/components/atoms/ThemeToggle.vue'

const router = useRouter()
const route = useRoute()

const step = ref<'credentials' | 'mfa'>('credentials')
const username = ref('')
const password = ref('')
const code = ref('')
const challengeToken = ref('')
const emailHint = ref('')
const resendCooldown = ref(0)
const isLoading = ref(false)
const error = ref('')
const notice = ref('')

onMounted(async () => {
  // A non-admin storefront session bounced here: probe the server session
  // and explain why the login view shows instead of a dashboard.
  // A valid boot-role session that lands here (typed or bookmarked URL)
  // keeps its session and returns to the dashboard — only stale roles
  // are wiped, so a crafted link can never log out an admin.
  if (route.query.reason === 'storefront-session') {
    const user = await refreshSession()
    const role = user?.role ?? getSessionRole()
    if (role && ADMIN_BOOT_ROLES.has(role)) {
      void router.replace(homePathFor(role))
      return
    }
    clearAuth()
    notice.value =
      'That session belongs to the storefront. Sign in with an admin account to continue.'
  }
})

let cooldownTimer: ReturnType<typeof setInterval> | undefined

// Clear error when user starts typing
watch([username, password, code], () => {
  if (error.value) {
    error.value = ''
  }
})

const ERROR_MESSAGES: Record<string, string> = {
  INVALID_CREDENTIALS: 'Invalid username or password',
  DEFAULT_CREDENTIALS_INVALID: 'Invalid username or password',
  MISSING_CREDENTIALS: 'Username and password are required',
  INVALID_USERNAME: 'Username exceeds maximum length',
  EMAIL_NOT_VERIFIED: 'Please verify your email address before logging in.',
  ACCOUNT_LOCKED: 'Account locked due to too many failed login attempts. Please try again later.',
  ACCOUNT_SUSPENDED: 'Your account has been suspended.',
  ACCOUNT_DELETED: 'Your account has been deleted.',
  TOKEN_GENERATION_FAILED: 'Failed to generate authentication token. Please try again.',
  MFA_UNAVAILABLE: 'We could not send your verification code. Please try again.',
  MFA_CHALLENGE_INVALID: 'Your verification session is no longer valid. Please sign in again.',
  MFA_CHALLENGE_EXPIRED: 'Your verification code expired. Please sign in again.',
  MFA_CODE_INVALID: 'Invalid verification code. Please try again.',
  MFA_MAX_ATTEMPTS: 'Too many invalid verification attempts. Please sign in again.',
  MFA_RESEND_COOLDOWN: 'Please wait before requesting another code.',
  INVALID_CODE_FORMAT: 'Verification codes are six digits.',
  MISSING_CHALLENGE: 'Your verification session is no longer valid. Please sign in again.',
}

// Codes that mean the challenge can never succeed: send the user back to the
// password step instead of letting them retry a dead code.
const DEAD_CHALLENGE_CODES = new Set([
  'MFA_CHALLENGE_INVALID',
  'MFA_CHALLENGE_EXPIRED',
  'MFA_MAX_ATTEMPTS',
])

/**
 * Map backend error codes to user-friendly messages
 */
function getErrorMessage(err: unknown): string {
  // Timeout errors (AbortError from AbortController)
  if (err instanceof DOMException && err.name === 'AbortError') {
    return 'Unable to connect to server. Please check your connection.'
  }

  // Network errors (TypeError from fetch failure)
  if (err instanceof TypeError) {
    return 'Unable to connect to server. Please check your connection.'
  }

  // Structured API errors with error code
  if (err instanceof ApiError && err.code) {
    return ERROR_MESSAGES[err.code] || err.message
  }

  // API errors without code (e.g., 401 without structured body)
  if (err instanceof ApiError) {
    if (err.statusCode === 401) {
      return 'Invalid username or password'
    }
    return err.message || 'Login failed. Please try again.'
  }

  // Generic errors
  if (err instanceof Error) {
    return err.message || 'Login failed. Please try again.'
  }

  return 'An unexpected error occurred. Please try again.'
}

function stopCooldown() {
  if (cooldownTimer) {
    clearInterval(cooldownTimer)
    cooldownTimer = undefined
  }
  resendCooldown.value = 0
}

function startCooldown(seconds: number) {
  stopCooldown()
  resendCooldown.value = seconds
  cooldownTimer = setInterval(() => {
    if (resendCooldown.value <= 1) {
      stopCooldown()
      return
    }
    resendCooldown.value -= 1
  }, 1000)
}

async function finishLogin(user: { id: string | number; username: string; role: string }) {
  // The session lives in the HttpOnly cookie set by MFA-verify; keep only
  // the identity for navigation and role gating.
  setSession({ id: user.id, username: user.username, role: user.role })

  const role = user.role

  // Check if first-login setup is needed (admin using default credentials)
  if (role === 'Admin') {
    try {
      sessionStorage.removeItem('first_login_complete')
      const setupCheck = await request.get<{
        data: { firstLoginComplete: boolean }
      }>('/api/auth/first-login')
      if (!setupCheck.data.data.firstLoginComplete) {
        sessionStorage.setItem('first_login_complete', 'false')
        router.push('/first-login')
        return
      }
      sessionStorage.setItem('first_login_complete', 'true')
    } catch {
      // If check fails, proceed to dashboard
    }
    router.push('/dashboard')
  } else {
    // Commentators land on their profile (they manage no content and may
    // not need the comment system); every other role keeps the list.
    router.push(homePathFor(role))
  }
}

function backToCredentials() {
  stopCooldown()
  step.value = 'credentials'
  challengeToken.value = ''
  emailHint.value = ''
  code.value = ''
  error.value = ''
}

async function handleLogin() {
  error.value = ''
  isLoading.value = true

  try {
    // The password step never returns a session token: it returns the MFA
    // challenge that the emailed code completes.
    const response = await request.post<{
      data: {
        firstLoginRequired?: boolean
        mfaRequired?: boolean
        challengeToken?: string
        emailHint?: string
      }
    }>('/api/auth/login', {
      username: username.value,
      password: password.value,
    })

    const payload = response.data.data

    // The seeded admin has no email address until first-login setup finishes,
    // so open the setup page without a session (every later login uses MFA).
    if (payload.firstLoginRequired) {
      sessionStorage.setItem('first_login_complete', 'false')
      sessionStorage.removeItem('first_login_complete_ts')
      await router.push('/first-login')
      return
    }

    if (payload.mfaRequired && payload.challengeToken) {
      challengeToken.value = payload.challengeToken
      emailHint.value = payload.emailHint || ''
      code.value = ''
      step.value = 'mfa'
      startCooldown(60)
      return
    }

    // A session token on the password step would mean the server skipped the
    // second factor; refuse it instead of silently downgrading login.
    error.value = 'Login failed. Please try again.'
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    isLoading.value = false
  }
}

async function handleVerify() {
  error.value = ''
  isLoading.value = true

  try {
    const response = await request.post<{
      data: {
        user: {
          id: string
          username: string
          role: string
        }
      }
    }>('/api/auth/mfa/verify', {
      challengeToken: challengeToken.value,
      code: code.value,
    })

    stopCooldown()
    await finishLogin(response.data.data.user)
  } catch (err) {
    if (err instanceof ApiError && err.code && DEAD_CHALLENGE_CODES.has(err.code)) {
      stopCooldown()
      step.value = 'credentials'
      challengeToken.value = ''
      emailHint.value = ''
      code.value = ''
      // The typing watcher clears the error on the next flush; wait for it so
      // the session-expired message is the one the user sees.
      await nextTick()
      error.value = getErrorMessage(err)
    } else {
      error.value = getErrorMessage(err)
    }
  } finally {
    isLoading.value = false
  }
}

async function handleResend() {
  if (resendCooldown.value > 0 || isLoading.value) {
    return
  }

  error.value = ''
  isLoading.value = true

  try {
    const response = await request.post<{
      data: {
        message: string
        challengeToken: string
        emailHint: string
        expiresIn: number
      }
    }>('/api/auth/mfa/resend', {
      challengeToken: challengeToken.value,
    })

    const payload = response.data.data
    if (payload.challengeToken) {
      challengeToken.value = payload.challengeToken
    }
    emailHint.value = payload.emailHint || emailHint.value
    code.value = ''
    startCooldown(60)
  } catch (err) {
    error.value = getErrorMessage(err)

    // A failed resend has already rotated the challenge server-side, so the
    // held token is dead. Send the user back to the password step instead of
    // leaving them on a code form that can never succeed.
    if (
      err instanceof ApiError &&
      err.code &&
      (DEAD_CHALLENGE_CODES.has(err.code) || err.code === 'MFA_UNAVAILABLE')
    ) {
      backToCredentials()
      error.value = getErrorMessage(err)
    }
  } finally {
    isLoading.value = false
  }
}

onUnmounted(stopCooldown)
</script>

<template>
  <div class="login-view">
    <div class="login-view__container">
      <div class="login-view__card">
        <div class="login-view__theme-toggle">
          <ThemeToggle />
        </div>
        <div class="login-view__logo">
          <img src="/logo.webp" alt="Lesstruct logo" class="login-view__logo-image" />
        </div>
        <h1 class="login-view__title">Lesstruct</h1>
        <p class="login-view__subtitle">
          {{ step === 'mfa' ? 'Check your email' : 'Sign in to your account' }}
        </p>

        <div v-if="notice" class="login-view__notice">
          {{ notice }}
        </div>

        <form v-if="step === 'credentials'" @submit.prevent="handleLogin" class="login-view__form">
          <div class="login-view__field">
            <label for="username" class="login-view__label">Username or Email</label>
            <input
              id="username"
              v-model="username"
              type="text"
              class="login-view__input"
              placeholder="username or email"
              required
              autocomplete="username"
            />
          </div>

          <div class="login-view__field">
            <label for="password" class="login-view__label">Password</label>
            <input
              id="password"
              v-model="password"
              type="password"
              class="login-view__input"
              placeholder="••••••••"
              required
              autocomplete="current-password"
            />
          </div>

          <div class="login-view__forgot">
            <router-link to="/forgot-password">Forgot password?</router-link>
          </div>

          <div v-if="error" class="login-view__error">
            {{ error }}
          </div>

          <Button
            type="submit"
            class="login-view__button"
            :is-loading="isLoading"
            full-width
          >
            {{ isLoading ? 'Signing in...' : 'Sign in' }}
          </Button>
        </form>

        <form v-else @submit.prevent="handleVerify" class="login-view__form">
          <p class="login-view__hint">
            Enter the 6-digit code we sent to
            <strong>{{ emailHint || 'your email address' }}</strong>.
          </p>

          <div class="login-view__field">
            <label for="code" class="login-view__label">Verification code</label>
            <input
              id="code"
              v-model="code"
              type="text"
              inputmode="numeric"
              pattern="[0-9]{6}"
              maxlength="6"
              class="login-view__input login-view__input--code"
              placeholder="000000"
              required
              autocomplete="one-time-code"
            />
          </div>

          <div v-if="error" class="login-view__error">
            {{ error }}
          </div>

          <Button
            type="submit"
            class="login-view__button"
            :is-loading="isLoading"
            full-width
          >
            {{ isLoading ? 'Verifying...' : 'Verify code' }}
          </Button>

          <div class="login-view__actions">
            <button
              type="button"
              class="login-view__link-button"
              :disabled="resendCooldown > 0 || isLoading"
              @click="handleResend"
            >
              {{ resendCooldown > 0 ? `Resend code in ${resendCooldown}s` : 'Resend code' }}
            </button>
            <button
              type="button"
              class="login-view__link-button"
              :disabled="isLoading"
              @click="backToCredentials"
            >
              Use a different account
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-view {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--brand-light-1);
  padding: 1rem;
}

.login-view__container {
  width: 100%;
  max-width: 400px;
}

.login-view__card {
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 2rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  position: relative;
}

.login-view__theme-toggle {
  position: absolute;
  top: 1rem;
  right: 1rem;
}

.login-view__logo {
  display: flex;
  justify-content: center;
  margin-bottom: 1rem;
}

.login-view__logo-image {
  width: 80px;
  height: 80px;
  object-fit: contain;
}

.login-view__title {
  font-size: 1.5rem;
  font-weight: 700;
  text-align: center;
  margin: 0 0 0.5rem 0;
  color: var(--brand-dark-2);
}

.login-view__subtitle {
  text-align: center;
  color: var(--brand-dark-2);
  margin: 0 0 2rem 0;
}

.login-view__form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.login-view__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.login-view__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.login-view__input {
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.login-view__input--code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 1.25rem;
  letter-spacing: 0.5rem;
  text-align: center;
}

.login-view__input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.login-view__input::placeholder {
  color: var(--brand-dark-2);
}

.login-view__hint {
  margin: 0;
  color: var(--brand-dark-2);
  font-size: 0.875rem;
  text-align: center;
}

.login-view__error {
  padding: 0.75rem;
  background-color: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.3);
  border-radius: 0.375rem;
  color: var(--color-error);
  font-size: 0.875rem;
}

.login-view__notice {
  padding: 0.75rem;
  background-color: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: 0.375rem;
  color: var(--brand-dark-2);
  font-size: 0.875rem;
  margin-bottom: 1.25rem;
}

.login-view__forgot {
  text-align: right;
  font-size: 0.8125rem;
}

.login-view__forgot a {
  color: var(--brand-primary);
  text-decoration: none;
}

.login-view__forgot a:hover {
  text-decoration: underline;
}

.login-view__actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  text-align: center;
  font-size: 0.8125rem;
}

.login-view__link-button {
  background: none;
  border: none;
  padding: 0;
  color: var(--brand-primary);
  cursor: pointer;
  font-size: inherit;
}

.login-view__link-button:disabled {
  color: var(--brand-dark-2);
  cursor: not-allowed;
}

.login-view__link-button:not(:disabled):hover {
  text-decoration: underline;
}

.login-view__button {
  margin-top: 0.25rem;
}
</style>
