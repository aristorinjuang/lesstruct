<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import request, { ApiError } from '@/utils/request'
import ThemeToggle from '@/components/atoms/ThemeToggle.vue'

const route = useRoute()
const router = useRouter()

const newPassword = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const error = ref('')
const isSuccess = ref(false)
const token = ref('')

const ERROR_MESSAGES: Record<string, string> = {
  TOKEN_INVALID: 'This password reset link is invalid.',
  TOKEN_EXPIRED: 'This password reset link has expired. Please request a new one.',
  INVALID_PASSWORD: 'Password must be at least 12 characters with uppercase, lowercase, numbers, and special characters.',
  MISSING_FIELDS: 'Token and new password are required.',
}

function validatePassword(): string | null {
  if (newPassword.value.length < 12) {
    return 'Password must be at least 12 characters.'
  }
  if (!/[A-Z]/.test(newPassword.value)) {
    return 'Password must contain at least one uppercase letter.'
  }
  if (!/[a-z]/.test(newPassword.value)) {
    return 'Password must contain at least one lowercase letter.'
  }
  if (!/[0-9]/.test(newPassword.value)) {
    return 'Password must contain at least one digit.'
  }
  if (!/[!@#$%^&*()\-_=+[\]{}|;:'",.<>?/`~]/.test(newPassword.value)) {
    return 'Password must contain at least one special character.'
  }
  return null
}

function getErrorMessage(err: unknown): string {
  if (err instanceof DOMException && err.name === 'AbortError') {
    return 'Unable to connect to server. Please check your connection.'
  }

  if (err instanceof TypeError) {
    return 'Unable to connect to server. Please check your connection.'
  }

  if (err instanceof ApiError && err.code) {
    return ERROR_MESSAGES[err.code] || err.message
  }

  if (err instanceof ApiError) {
    return err.message || 'Request failed. Please try again.'
  }

  if (err instanceof Error) {
    return err.message || 'Request failed. Please try again.'
  }

  return 'An unexpected error occurred. Please try again.'
}

async function handleSubmit() {
  error.value = ''

  const validationError = validatePassword()
  if (validationError) {
    error.value = validationError
    return
  }

  if (newPassword.value !== confirmPassword.value) {
    error.value = 'Passwords do not match.'
    return
  }

  isLoading.value = true

  try {
    await request.post('/api/auth/reset-password', {
      token: token.value,
      newPassword: newPassword.value,
    })
    isSuccess.value = true
    setTimeout(() => {
      router.push('/login')
    }, 2000)
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    isLoading.value = false
  }
}

onMounted(() => {
  token.value = (route.query.token as string) || ''
  if (!token.value) {
    error.value = 'Invalid or missing reset token.'
  }
})
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
        <p class="login-view__subtitle">Set new password</p>

        <form v-if="!isSuccess" @submit.prevent="handleSubmit" class="login-view__form">
          <div class="login-view__field">
            <label for="new-password" class="login-view__label">New Password</label>
            <input
              id="new-password"
              v-model="newPassword"
              type="password"
              class="login-view__input"
              placeholder="Enter new password"
              required
              autocomplete="new-password"
            />
          </div>

          <div class="login-view__field">
            <label for="confirm-password" class="login-view__label">Confirm Password</label>
            <input
              id="confirm-password"
              v-model="confirmPassword"
              type="password"
              class="login-view__input"
              placeholder="Confirm new password"
              required
              autocomplete="new-password"
            />
          </div>

          <div v-if="error" class="login-view__error">
            {{ error }}
          </div>

          <button
            type="submit"
            class="login-view__button"
            :disabled="isLoading || !token"
          >
            {{ isLoading ? 'Resetting...' : 'Reset password' }}
          </button>
        </form>

        <div v-else class="login-view__success">
          <p class="login-view__success-message">
            Password reset successfully. Redirecting to sign in...
          </p>
        </div>
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

.login-view__success {
  text-align: center;
  padding: 1rem 0;
}

.login-view__success-message {
  color: var(--brand-dark-1);
  line-height: 1.6;
}
</style>
