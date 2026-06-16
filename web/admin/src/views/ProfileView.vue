<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '@/utils/request'
import { useAuth } from '@/composables/useAuth'
import Toast from '@/components/molecules/Toast.vue'
import CustomFieldRenderer from '@/components/molecules/CustomFieldRenderer.vue'
import { validateCustomField } from '@/utils/validation'
import type { FieldSchema } from '@/types/customfield'

const { isAuthenticated, role } = useAuth()
const isAdmin = computed(() => role.value === 'Admin')

interface UserProfile {
  id: number
  username: string
  name: string
  email: string
  role: string
  profilePicture?: string
  createdAt: string
  updatedAt: string
  customFields?: Record<string, any>
}

const profile = ref<UserProfile | null>(null)
const isLoading = ref(false)
const error = ref('')

// Profile picture state
const isUploadingPicture = ref(false)
const isDeletingPicture = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

const defaultAvatar = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="%236b7280" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M20 21a8 8 0 1 0-16 0"/></svg>')

async function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  const formData = new FormData()
  formData.append('image', file)

  isUploadingPicture.value = true
  try {
    const response = await api.put<{ data: { profilePicture: string } }>('/api/profile/picture', formData)
    if (profile.value) {
      profile.value.profilePicture = response.data.data.profilePicture
    }
    displayToast('Profile picture updated successfully')
  } catch (err: any) {
    displayToast(err?.message || 'Failed to upload profile picture', 'error')
  } finally {
    isUploadingPicture.value = false
    if (fileInputRef.value) fileInputRef.value.value = ''
  }
}

async function handleDeletePicture() {
  isDeletingPicture.value = true
  try {
    await api.delete('/api/profile/picture')
    if (profile.value) {
      profile.value.profilePicture = undefined
    }
    displayToast('Profile picture deleted successfully')
  } catch (err: any) {
    displayToast(err?.message || 'Failed to delete profile picture', 'error')
  } finally {
    isDeletingPicture.value = false
  }
}

// Password form state
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const passwordSuccess = ref('')
const isChangingPassword = ref(false)

// Custom fields state
const customFields = ref<Record<string, any>>({})
const userFields = ref<FieldSchema[]>([])
const userSystemFields = ref<FieldSchema[]>([])
const customFieldErrors = ref<Record<string, string>>({})
const isFieldsLoading = ref(false)
const isSavingFields = ref(false)
const fieldsError = ref('')
const showFieldErrorSummary = ref(false)

// Toast state
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

let schemaFetchId = 0

function getDefaultFieldValue(field: FieldSchema): any {
  switch (field.type) {
    case 'checkbox': return false
    case 'number': return undefined
    default: return ''
  }
}

function validateCustomFieldOnBlur(field: FieldSchema) {
  const err = validateCustomField(field, customFields.value[field.slug])
  if (err) {
    customFieldErrors.value[field.slug] = err
  } else {
    delete customFieldErrors.value[field.slug]
  }
}

function validateAllCustomFields(): boolean {
  const fieldsToValidate = isAdmin.value
    ? [...userFields.value, ...userSystemFields.value]
    : userFields.value
  for (const field of fieldsToValidate) {
    validateCustomFieldOnBlur(field)
  }
  const valid = Object.keys(customFieldErrors.value).length === 0
  showFieldErrorSummary.value = !valid
  return valid
}

async function fetchUserFieldSchemas() {
  const id = ++schemaFetchId
  isFieldsLoading.value = true
  try {
    const response = await api.get<any>('/api/profile/user-fields')
    if (id !== schemaFetchId) return
    userFields.value = response.data.data?.fields || []
    userSystemFields.value = response.data.data?.systemFields || []
  } catch {
    if (id !== schemaFetchId) return
    userFields.value = []
    userSystemFields.value = []
  } finally {
    if (id === schemaFetchId) {
      isFieldsLoading.value = false
    }
  }
}

async function handleSaveFields() {
  if (!validateAllCustomFields()) return

  isSavingFields.value = true
  fieldsError.value = ''

  const payload: Record<string, any> = {}
  if (isAdmin.value) {
    for (const [key, value] of Object.entries(customFields.value)) {
      payload[key] = value
    }
  } else {
    const systemSlugs = new Set(userSystemFields.value.map(f => f.slug))
    for (const [key, value] of Object.entries(customFields.value)) {
      if (!systemSlugs.has(key)) {
        payload[key] = value
      }
    }
  }

  try {
    const response = await api.put<{ data: { profile: UserProfile } }>('/api/profile/custom-fields', { customFields: payload })
    profile.value = response.data.data.profile
    if (profile.value) {
      customFields.value = profile.value.customFields
        ? { ...profile.value.customFields }
        : {}
    }
    showFieldErrorSummary.value = false
    displayToast('Profile fields updated successfully')
  } catch (err: any) {
    fieldsError.value = err?.message || 'Failed to update profile fields'
  } finally {
    isSavingFields.value = false
  }
}

onMounted(async () => {
  if (!isAuthenticated.value) return

  isLoading.value = true
  error.value = ''

  try {
    const response = await api.get<{ data: { profile: UserProfile } }>('/api/profile')
    profile.value = response.data.data.profile

    if (profile.value) {
      customFields.value = profile.value.customFields
        ? { ...profile.value.customFields }
        : {}
    }

    fetchUserFieldSchemas()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load profile'
  } finally {
    isLoading.value = false
  }
})

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
}

function validatePasswordForm(): boolean {
  passwordError.value = ''

  if (!currentPassword.value) {
    passwordError.value = 'Current password is required'
    return false
  }

  if (!newPassword.value) {
    passwordError.value = 'New password is required'
    return false
  }

  if (newPassword.value.length < 12) {
    passwordError.value = 'New password must be at least 12 characters'
    return false
  }

  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = 'Passwords do not match'
    return false
  }

  return true
}

async function handleChangePassword() {
  if (!validatePasswordForm()) return

  isChangingPassword.value = true
  passwordError.value = ''
  passwordSuccess.value = ''

  try {
    await api.put('/api/profile/password', {
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    })
    passwordSuccess.value = 'Password updated successfully'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (err: any) {
    if (!err?.statusCode) {
      passwordError.value = 'Unable to connect to server. Please check your connection.'
      return
    }
    const code = err?.code
    if (code === 'INVALID_PASSWORD') {
      passwordError.value = 'Current password is incorrect or new password does not meet requirements'
    } else if (code === 'MISSING_FIELDS') {
      passwordError.value = 'Current password and new password are required'
    } else {
      passwordError.value = err?.message || 'Failed to update password'
    }
  } finally {
    isChangingPassword.value = false
  }
}
</script>

<template>
  <div class="profile-view">
    <Toast
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />
    <h1 class="page-title" style="margin-bottom: 1.5rem;">Profile</h1>
    <div class="profile-view__content">
      <div v-if="isLoading" class="profile-view__loading">
        <p>Loading profile...</p>
      </div>

      <div v-else-if="error" class="alert alert-error" role="alert">
        {{ error }}
      </div>

      <div v-else-if="profile" class="card">
        <h2 class="card-title">User Information</h2>

        <!-- Profile Picture -->
        <div class="profile-view__avatar-section">
          <div class="profile-view__avatar-wrapper">
            <img
              :src="profile.profilePicture || defaultAvatar"
              :alt="profile.name || profile.username"
              class="profile-view__avatar"
              :class="{ 'profile-view__avatar--placeholder': !profile.profilePicture }"
            />
          </div>
          <div class="profile-view__avatar-actions">
            <input
              ref="fileInputRef"
              type="file"
              accept="image/jpeg,image/png,image/gif,image/webp"
              class="profile-view__file-input"
              id="avatar-upload"
              @change="handleFileSelect"
              :disabled="isUploadingPicture"
            />
            <label for="avatar-upload" class="profile-view__button profile-view__button--secondary" :class="{ 'profile-view__button--disabled': isUploadingPicture }">
              {{ isUploadingPicture ? 'Uploading...' : 'Upload Picture' }}
            </label>
            <button
              v-if="profile.profilePicture"
              type="button"
              class="profile-view__button profile-view__button--danger"
              :disabled="isDeletingPicture"
              @click="handleDeletePicture"
            >
              {{ isDeletingPicture ? 'Deleting...' : 'Delete Picture' }}
            </button>
          </div>
        </div>

        <div class="profile-view__field">
          <label class="profile-view__label">Name</label>
          <p class="profile-view__value">{{ profile.name || profile.username }}</p>
        </div>
        <div class="profile-view__field">
          <label class="profile-view__label">Username</label>
          <p class="profile-view__value">{{ profile.username }}</p>
        </div>
        <div class="profile-view__field">
          <label class="profile-view__label">Email</label>
          <p class="profile-view__value">{{ profile.email }}</p>
        </div>
        <div class="profile-view__field">
          <label class="profile-view__label">Role</label>
          <p class="profile-view__value profile-view__role">{{ profile.role }}</p>
        </div>
        <div class="profile-view__field">
          <label class="profile-view__label">Member since</label>
          <p class="profile-view__value">{{ formatDate(profile.createdAt) }}</p>
        </div>
      </div>

      <!-- Profile Fields -->
      <div v-if="profile && (userFields.length > 0 || userSystemFields.length > 0)" class="card">
        <h2 class="card-title">Profile Fields</h2>

        <p v-if="showFieldErrorSummary && Object.keys(customFieldErrors).length > 0" class="alert alert-error" role="alert">
          Please fix {{ Object.keys(customFieldErrors).length }} field(s) below before saving.
        </p>
        <p v-if="fieldsError" class="profile-view__field-error" role="alert">{{ fieldsError }}</p>

        <div v-if="isFieldsLoading" class="profile-view__loading">
          <p>Loading fields...</p>
        </div>

        <template v-else>
          <!-- Custom fields -->
          <div v-if="userFields.length > 0" class="profile-view__fields-section">
            <CustomFieldRenderer
              v-for="field in userFields"
              :key="'profile-custom-' + field.slug"
              :field="field"
              :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
              :disabled="isSavingFields"
              :error="customFieldErrors[field.slug] || ''"
              @update:model-value="customFields[field.slug] = $event"
              @blur="validateCustomFieldOnBlur(field)"
            />
          </div>

          <!-- System fields (admin-editable, read-only for others) -->
          <div v-if="userSystemFields.length > 0" class="profile-view__fields-section">
            <div class="profile-view__section-divider"></div>
            <CustomFieldRenderer
              v-for="field in userSystemFields"
              :key="'profile-system-' + field.slug"
              :field="field"
              :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
              :disabled="!isAdmin || isSavingFields"
              :error="isAdmin ? (customFieldErrors[field.slug] || '') : ''"
              :system-field="true"
              @update:model-value="isAdmin ? (customFields[field.slug] = $event) : undefined"
              @blur="isAdmin ? validateCustomFieldOnBlur(field) : undefined"
            />
          </div>

          <button
            v-if="userFields.length > 0 || (isAdmin && userSystemFields.length > 0)"
            type="button"
            class="profile-view__button"
            :disabled="isSavingFields"
            @click="handleSaveFields"
          >
            {{ isSavingFields ? 'Saving...' : 'Save Profile' }}
          </button>
        </template>
      </div>

      <!-- Update Password -->
      <div v-if="profile" class="card">
        <h2 class="card-title">Update Password</h2>

        <p v-if="passwordSuccess" class="alert alert-success" role="status">{{ passwordSuccess }}</p>
        <p v-if="passwordError" class="profile-view__field-error" role="alert">{{ passwordError }}</p>

        <form class="profile-view__password-form" @submit.prevent="handleChangePassword">
          <div class="profile-view__field">
            <label for="current-password" class="profile-view__label">Current Password</label>
            <input
              id="current-password"
              v-model="currentPassword"
              type="password"
              class="form-input profile-view__input--password"
              autocomplete="current-password"
              :disabled="isChangingPassword"
            />
          </div>

          <div class="profile-view__field">
            <label for="new-password" class="profile-view__label">New Password</label>
            <input
              id="new-password"
              v-model="newPassword"
              type="password"
              class="form-input profile-view__input--password"
              autocomplete="new-password"
              :disabled="isChangingPassword"
            />
          </div>

          <div class="profile-view__field">
            <label for="confirm-password" class="profile-view__label">Confirm New Password</label>
            <input
              id="confirm-password"
              v-model="confirmPassword"
              type="password"
              class="form-input profile-view__input--password"
              autocomplete="new-password"
              :disabled="isChangingPassword"
            />
          </div>

          <button
            type="submit"
            class="profile-view__button"
            :disabled="isChangingPassword"
          >
            {{ isChangingPassword ? 'Updating...' : 'Update Password' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profile-view__content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.profile-view__loading {
  text-align: center;
  padding: 2rem;
  border-radius: 0.5rem;
}

.profile-view__avatar-section {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid var(--brand-light-2);
}

.profile-view__avatar-wrapper {
  flex-shrink: 0;
}

.profile-view__avatar {
  width: 96px;
  height: 96px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid var(--brand-light-2);
}

.profile-view__avatar--placeholder {
  padding: 12px;
  background-color: var(--brand-light-1);
}

.profile-view__avatar-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.profile-view__file-input {
  display: none;
}

.profile-view__button--secondary {
  background-color: transparent;
  color: var(--brand-primary);
  border: 1px solid var(--brand-primary);
  display: inline-block;
  text-align: center;
  line-height: 2.5;
  max-width: 200px;
}

.profile-view__button--secondary:hover:not(:disabled) {
  background-color: var(--brand-primary-light);
}

.profile-view__button--danger {
  background-color: transparent;
  color: var(--color-error);
  border: 1px solid var(--color-error);
  max-width: 200px;
}

.profile-view__button--danger:hover:not(:disabled) {
  background-color: rgba(239, 68, 68, 0.1);
}

.profile-view__button--disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.profile-view__field {
  margin-bottom: 1rem;
}

.profile-view__label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-2);
  margin-bottom: 0.25rem;
}

.profile-view__value {
  font-size: 1rem;
  color: var(--brand-dark-1);
  margin: 0;
}

.profile-view__role {
  text-transform: capitalize;
}

.profile-view__input--password {
  width: 100%;
  max-width: 400px;
}

.profile-view__field-error {
  margin: 0 0 1rem 0;
  font-size: 0.875rem;
  color: var(--color-error-dark);
}

.alert {
  margin-bottom: 1rem;
}

.profile-view__password-form {
  display: flex;
  flex-direction: column;
}

.profile-view__button {
  padding: 0.625rem 1rem;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
  max-width: 200px;
  transition: background-color 0.2s;
}

.profile-view__button:hover:not(:disabled) {
  background-color: var(--color-interactive-hover);
}

.profile-view__button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.profile-view__button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.profile-view__fields-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.profile-view__section-divider {
  height: 1px;
  background-color: var(--brand-light-2);
  margin: 0.25rem 0;
}

@media (min-width: 1024px) {
  .profile-view__fields-section {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .profile-view__fields-section .form-field--textarea,
  .profile-view__fields-section .form-field--checkbox {
    grid-column: 1 / -1;
  }
}

@media (max-width: 640px) {
  .profile-view__input--password,
  .profile-view__button {
    max-width: 100%;
  }
}
</style>
