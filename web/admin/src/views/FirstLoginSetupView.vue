<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import request, { ApiError } from '@/utils/request'
import { clearAuth } from '@/composables/useAuth'
import ThemeToggle from '@/components/atoms/ThemeToggle.vue'

const router = useRouter()

const password = ref('')
const confirmPassword = ref('')
const email = ref('')
const isLoading = ref(false)
const error = ref('')
const successMessage = ref('')
let redirectTimer: ReturnType<typeof setTimeout> | null = null

watch([password, confirmPassword, email], () => {
  if (error.value) {
    error.value = ''
  }
})

onUnmounted(() => {
  if (redirectTimer) {
    clearTimeout(redirectTimer)
    redirectTimer = null
  }
})

const ERROR_MESSAGES: Record<string, string> = {
  INVALID_REQUEST: 'Invalid request. Please check all fields.',
  INVALID_DATABASE_TYPE: 'Invalid database type selected.',
  INVALID_PASSWORD:
    'Password must be at least 12 characters with mixed case, numbers, and special characters',
  INVALID_EMAIL: 'Please enter a valid email address',
  SETUP_ALREADY_COMPLETE:
    'First-login setup has already been completed',
  ADMIN_NOT_FOUND: 'Admin account not found. Please restart the application.',
  HASH_FAILED: 'Failed to process password. Please try again.',
  DATABASE_ERROR: 'A database error occurred. Please try again.',
  REQUEST_TIMEOUT: 'Request timed out. Please try again.',
}

function validatePasswordComplexity(pwd: string): string | null {
  if (pwd.length < 12) return 'Password must be at least 12 characters with mixed case, numbers, and special characters'
  if (!/[A-Z]/.test(pwd)) return 'Password must be at least 12 characters with mixed case, numbers, and special characters'
  if (!/[a-z]/.test(pwd)) return 'Password must be at least 12 characters with mixed case, numbers, and special characters'
  if (!/\d/.test(pwd)) return 'Password must be at least 12 characters with mixed case, numbers, and special characters'
  if (!/[^A-Za-z0-9]/.test(pwd)) return 'Password must be at least 12 characters with mixed case, numbers, and special characters'
  return null
}

function getErrorMessage(err: unknown): string {
  if (err instanceof DOMException && err.name === 'AbortError') {
    return 'Unable to connect to server'
  }

  if (err instanceof TypeError) {
    return 'Unable to connect to server'
  }

  if (err instanceof ApiError && err.code) {
    return ERROR_MESSAGES[err.code] || err.message
  }

  if (err instanceof ApiError) {
    return err.message || 'Setup failed. Please try again.'
  }

  if (err instanceof Error) {
    return err.message || 'Setup failed. Please try again.'
  }

  return 'An unexpected error occurred. Please try again.'
}

function handleSetupAlreadyComplete() {
  sessionStorage.setItem('first_login_complete', 'true')
  isLoading.value = false
  redirectTimer = setTimeout(() => {
    redirectTimer = null
    clearAuth()
    router.push('/login')
  }, 1500)
}

async function handleSetup() {
  error.value = ''
  successMessage.value = ''

  if (!password.value || !confirmPassword.value || !email.value) {
    error.value = 'All fields are required'
    return
  }

  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }

  const passwordError = validatePasswordComplexity(password.value)
  if (passwordError) {
    error.value = passwordError
    return
  }

  isLoading.value = true

  try {
    await request.post<{
      data: {
        message: string
        redirect: string
      }
    }>('/api/auth/first-login', {
      password: password.value,
      email: email.value,
      databaseType: 'sqlite',
    })

    sessionStorage.removeItem('first_login_complete')
    successMessage.value = 'Setup completed successfully! Redirecting to login...'
    clearAuth()
    redirectTimer = setTimeout(() => {
      redirectTimer = null
      router.push('/login')
    }, 1500)
  } catch (err) {
    if (err instanceof ApiError && err.code === 'SETUP_ALREADY_COMPLETE') {
      error.value = ERROR_MESSAGES.SETUP_ALREADY_COMPLETE
      handleSetupAlreadyComplete()
      return
    }

    error.value = getErrorMessage(err)
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="first-login-setup">
    <div class="first-login-setup__container">
      <div class="first-login-setup__card">
        <div class="first-login-setup__theme-toggle">
          <ThemeToggle />
        </div>
        <div class="first-login-setup__logo">
          <img src="/logo.webp" alt="Lesstruct logo" class="first-login-setup__logo-image" />
        </div>
        <h1 class="first-login-setup__title">Initial Setup</h1>
        <p class="first-login-setup__subtitle">Secure your admin account</p>

        <form @submit.prevent="handleSetup" class="first-login-setup__form" novalidate>
          <div class="first-login-setup__field">
            <label for="email" class="first-login-setup__label">Email</label>
            <input
              id="email"
              v-model="email"
              type="email"
              class="first-login-setup__input"
              placeholder="admin@example.com"
              required
              autocomplete="email"
              :aria-invalid="!!error"
              :aria-describedby="error ? 'setup-error' : undefined"
            />
          </div>

          <div class="first-login-setup__field">
            <label for="password" class="first-login-setup__label">New Password</label>
            <input
              id="password"
              v-model="password"
              type="password"
              class="first-login-setup__input"
              placeholder="••••••••••••"
              required
              autocomplete="new-password"
              :aria-invalid="!!error"
              :aria-describedby="error ? 'setup-error' : undefined"
            />
          </div>

          <div class="first-login-setup__field">
            <label for="confirmPassword" class="first-login-setup__label">Confirm Password</label>
            <input
              id="confirmPassword"
              v-model="confirmPassword"
              type="password"
              class="first-login-setup__input"
              placeholder="••••••••••••"
              required
              autocomplete="new-password"
              :aria-invalid="!!error"
              :aria-describedby="error ? 'setup-error' : undefined"
            />
          </div>

          <div v-if="error" id="setup-error" class="first-login-setup__error" role="alert">
            {{ error }}
          </div>

          <div v-if="successMessage" class="first-login-setup__success" role="status">
            {{ successMessage }}
          </div>

          <button
            type="submit"
            class="first-login-setup__button"
            :disabled="isLoading || !!successMessage"
          >
            {{ isLoading ? 'Setting up...' : 'Complete Setup' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.first-login-setup {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--brand-light-1);
  padding: 1rem;
}

.first-login-setup__container {
  width: 100%;
  max-width: 400px;
}

.first-login-setup__card {
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 2rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  position: relative;
}

.first-login-setup__theme-toggle {
  position: absolute;
  top: 1rem;
  right: 1rem;
}

.first-login-setup__logo {
  display: flex;
  justify-content: center;
  margin-bottom: 1rem;
}

.first-login-setup__logo-image {
  width: 80px;
  height: 80px;
  object-fit: contain;
}

.first-login-setup__title {
  font-size: 1.5rem;
  font-weight: 700;
  text-align: center;
  margin: 0 0 0.5rem 0;
  color: var(--brand-dark-2);
}

.first-login-setup__subtitle {
  text-align: center;
  color: var(--brand-dark-2);
  margin: 0 0 2rem 0;
}

.first-login-setup__form {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.first-login-setup__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.first-login-setup__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.first-login-setup__input {
  padding: 0.75rem 1rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 1rem;
  min-height: 44px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.first-login-setup__input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.first-login-setup__input::placeholder {
  color: var(--brand-dark-2);
}

.first-login-setup__error {
  padding: 0.75rem;
  background-color: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.3);
  border-radius: 0.375rem;
  color: var(--color-error);
  font-size: 0.875rem;
}

.first-login-setup__success {
  padding: 0.75rem;
  background-color: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.3);
  border-radius: 0.375rem;
  color: var(--color-success);
  font-size: 0.875rem;
}

.first-login-setup__button {
  padding: 0.75rem 1rem;
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
  border: none;
  border-radius: 0.375rem;
  font-size: 1rem;
  font-weight: 500;
  min-height: 44px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.first-login-setup__button:hover:not(:disabled) {
  background-color: var(--brand-primary-hover);
}

.first-login-setup__button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
