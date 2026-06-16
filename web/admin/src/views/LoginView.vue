<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { setAuthToken, setUserRole } from '@/composables/useAuth'
import request, { ApiError } from '@/utils/request'
import ThemeToggle from '@/components/atoms/ThemeToggle.vue'

const router = useRouter()

const username = ref('')
const password = ref('')
const isLoading = ref(false)
const error = ref('')

// Clear error when user starts typing
watch([username, password], () => {
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
}

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

async function handleLogin() {
  error.value = ''
  isLoading.value = true

  try {
    // Call the real backend API
    const response = await request.post<{
      data: {
        token: string
        user: {
          id: string
          username: string
          role: string
        }
      }
    }>('/api/auth/login', {
      username: username.value,
      password: password.value,
    })

    // Store the real JWT token (unwrap backend's { data: ... } envelope)
    setAuthToken(response.data.data.token)

    // Store the user's role for permission-based navigation
    setUserRole(response.data.data.user.role)

    // Check if first-login setup is needed (admin using default credentials)
    const userRole = response.data.data.user.role
    if (userRole === 'Admin') {
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
    } else if (userRole === 'Commentator') {
      router.push('/comment')
    } else {
      router.push('/content?type=post')
    }
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    isLoading.value = false
  }
}
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
        <p class="login-view__subtitle">Sign in to your account</p>

        <form @submit.prevent="handleLogin" class="login-view__form">
          <div class="login-view__field">
            <label for="username" class="login-view__label">Username</label>
            <input
              id="username"
              v-model="username"
              type="text"
              class="login-view__input"
              placeholder="username"
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

          <button
            type="submit"
            class="login-view__button"
            :disabled="isLoading"
          >
            {{ isLoading ? 'Signing in...' : 'Sign in' }}
          </button>
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

.login-view__input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.login-view__input::placeholder {
  color: var(--brand-dark-2);
}

.login-view__error {
  padding: 0.75rem;
  background-color: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.3);
  border-radius: 0.375rem;
  color: var(--color-error);
  font-size: 0.875rem;
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

.login-view__button {
  padding: 0.625rem 1rem;
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.login-view__button:hover:not(:disabled) {
  background-color: var(--brand-primary-hover);
}

.login-view__button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
