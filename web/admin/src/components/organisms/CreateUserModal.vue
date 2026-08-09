<script setup lang="ts">
import { ref, computed } from 'vue'
import Modal from './Modal.vue'
import Button from '@/components/atoms/Button.vue'
import CustomFieldRenderer from '@/components/molecules/CustomFieldRenderer.vue'
import { useUserStore } from '@/stores/domain/user'
import { useAuth } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
import { validateCustomField, validateCustomFields } from '@/utils/validation'
import { ApiError } from '@/utils/request'
import type { UserRole } from '@/types/user'
import type { FieldSchema } from '@/types/customfield'

const props = defineProps<{
  isOpen: boolean
  userFields: FieldSchema[]
  userSystemFields: FieldSchema[]
}>()

const emit = defineEmits<{
  close: []
  created: []
}>()

const userStore = useUserStore()
const { role: currentUserRole } = useAuth()
const { commentsEnabled } = useConfig()

const isAdmin = computed(() => currentUserRole.value === 'Admin')

type FormState = 'form' | 'loading' | 'success'
type FormErrors = Record<string, string>

const state = ref<FormState>('form')
const username = ref('')
const displayName = ref('')
const email = ref('')
const role = ref<UserRole>('Contributor')
const errors = ref<FormErrors>({})
const generatedPassword = ref('')
const passwordCopied = ref(false)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const customFields = ref<Record<string, any>>({})
const customFieldErrors = ref<Record<string, string>>({})

// The Commentator role exists only for the comment system; it is not
// assignable when comments are disabled.
const roles = computed<{ value: UserRole; label: string }[]>(() => {
  const all: { value: UserRole; label: string }[] = [
    { value: 'Admin', label: 'Admin' },
    { value: 'Contributor', label: 'Contributor' },
  ]
  if (commentsEnabled.value) {
    all.push({ value: 'Commentator', label: 'Commentator' })
  }
  return all
})

const usernamePattern = /^[a-zA-Z0-9_-]{1,50}$/
const emailPattern = /^[a-zA-Z0-9](?:[a-zA-Z0-9._%+-]*[a-zA-Z0-9])?@[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?\.[a-zA-Z]{2,}$/

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getDefaultFieldValue(field: FieldSchema): any {
  switch (field.type) {
    case 'checkbox': return false
    case 'number': return undefined
    default: return ''
  }
}

function resetForm() {
  username.value = ''
  displayName.value = ''
  email.value = ''
  role.value = commentsEnabled.value ? 'Commentator' : 'Contributor'
  errors.value = {}
  generatedPassword.value = ''
  passwordCopied.value = false
  customFields.value = {}
  customFieldErrors.value = {}
  state.value = 'form'
}

function validateCustomFieldOnBlur(fieldSlug: string) {
  const field = [...props.userFields, ...props.userSystemFields].find(f => f.slug === fieldSlug)
  if (!field) return
  const err = validateCustomField(field, customFields.value[fieldSlug])
  if (err) {
    customFieldErrors.value[fieldSlug] = err
  } else {
    delete customFieldErrors.value[fieldSlug]
  }
}

function validate(): boolean {
  const newErrors: FormErrors = {}

  if (!username.value.trim()) {
    newErrors.username = 'Username is required'
  } else if (!usernamePattern.test(username.value)) {
    newErrors.username = 'Username must be 1-50 characters and contain only letters, numbers, underscores, and hyphens'
  }

  if (!email.value.trim()) {
    newErrors.email = 'Email is required'
  } else if (!emailPattern.test(email.value)) {
    newErrors.email = 'Invalid email format'
  }

  if (!role.value) {
    newErrors.role = 'Role is required'
  }

  errors.value = newErrors

  const fieldsToValidate = isAdmin.value
    ? [...props.userFields, ...props.userSystemFields]
    : props.userFields
  const fieldErrors = validateCustomFields(
    fieldsToValidate,
    customFields.value as Record<string, unknown>,
  )
  customFieldErrors.value = fieldErrors

  return Object.keys(newErrors).length === 0 && Object.keys(fieldErrors).length === 0
}

async function handleSubmit() {
  if (!validate()) return

  state.value = 'loading'

  try {
    const password = await userStore.createUser({
      username: username.value,
      name: displayName.value,
      email: email.value,
      role: role.value,
      customFields: Object.keys(customFields.value).length > 0
        ? customFields.value
        : undefined,
    })
    generatedPassword.value = password
    state.value = 'success'
    emit('created')
  } catch (err: unknown) {
    state.value = 'form'
    const apiErr = err as ApiError
    if (!apiErr?.statusCode) {
      errors.value = { general: 'Unable to connect to server. Please check your connection.' }
      return
    }
    const code = apiErr?.code
    const errorMap: Record<string, string> = {
      VALIDATION_ERROR: 'Invalid username, email, or role',
      USERNAME_EXISTS: 'Username already exists',
      EMAIL_EXISTS: 'Email address already registered',
      EMAIL_BLOCKED: 'This email address has been blocked',
      INVALID_ROLE: 'Invalid user role',
      MISSING_FIELDS: 'Username, email, and role are required',
    }
    errors.value = { general: errorMap[code ?? ''] || apiErr?.message || 'Failed to create user' }
  }
}

async function copyPassword() {
  try {
    await navigator.clipboard.writeText(generatedPassword.value)
    passwordCopied.value = true
    setTimeout(() => {
      passwordCopied.value = false
    }, 2000)
  } catch {
    const textArea = document.createElement('textarea')
    textArea.value = generatedPassword.value
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    passwordCopied.value = true
    setTimeout(() => {
      passwordCopied.value = false
    }, 2000)
  }
}

function handleClose() {
  if (state.value === 'loading') return
  resetForm()
  emit('close')
}
</script>

<template>
  <Modal :is-open="isOpen" title="Create User" @close="handleClose">
    <form v-if="state !== 'success'" class="create-user-modal" @submit.prevent="handleSubmit">
      <p v-if="errors.general" class="create-user-modal__error">{{ errors.general }}</p>

      <div class="create-user-modal__field">
        <label for="create-username" class="create-user-modal__label">Username</label>
        <input
          id="create-username"
          v-model="username"
          type="text"
          class="create-user-modal__input"
          :class="{ 'create-user-modal__input--error': errors.username }"
          autocomplete="off"
          :disabled="state === 'loading'"
        />
        <span v-if="errors.username" class="create-user-modal__field-error">{{ errors.username }}</span>
      </div>

      <div class="create-user-modal__field">
        <label for="create-name" class="create-user-modal__label">Display Name</label>
        <input
          id="create-name"
          v-model="displayName"
          type="text"
          class="create-user-modal__input"
          autocomplete="off"
          :disabled="state === 'loading'"
          placeholder="Optional"
        />
      </div>

      <div class="create-user-modal__field">
        <label for="create-email" class="create-user-modal__label">Email</label>
        <input
          id="create-email"
          v-model="email"
          type="email"
          class="create-user-modal__input"
          :class="{ 'create-user-modal__input--error': errors.email }"
          autocomplete="off"
          :disabled="state === 'loading'"
        />
        <span v-if="errors.email" class="create-user-modal__field-error">{{ errors.email }}</span>
      </div>

      <div class="create-user-modal__field">
        <label for="create-role" class="create-user-modal__label">Role</label>
        <select
          id="create-role"
          v-model="role"
          class="create-user-modal__input"
          :disabled="state === 'loading'"
        >
          <option v-for="r in roles" :key="r.value" :value="r.value">{{ r.label }}</option>
        </select>
      </div>

      <!-- Custom fields validation summary -->
      <div
        v-if="Object.keys(customFieldErrors).length > 0"
        class="create-user-modal__fields-section"
      >
        <div class="create-user-modal__error">
          Please fix the following:
          <ul>
            <li v-for="(msg, slug) in customFieldErrors" :key="slug">
              {{ msg }}
            </li>
          </ul>
        </div>
      </div>

      <!-- Custom fields section -->
      <div v-if="userFields.length > 0" class="create-user-modal__fields-section">
        <div class="create-user-modal__section-divider"></div>
        <CustomFieldRenderer
          v-for="field in userFields"
          :key="'user-custom-' + field.slug"
          :field="field"
          :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
          :error="customFieldErrors[field.slug] ?? ''"
          :disabled="state === 'loading'"
          @update:model-value="(val: unknown) => { customFields[field.slug] = val; delete customFieldErrors[field.slug] }"
          @blur="validateCustomFieldOnBlur(field.slug)"
        />
      </div>

      <!-- System fields section (admin only) -->
      <div v-if="userSystemFields.length > 0" class="create-user-modal__fields-section">
        <div class="create-user-modal__section-divider"></div>
        <CustomFieldRenderer
          v-for="field in userSystemFields"
          :key="'user-system-' + field.slug"
          :field="field"
          :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
          :error="isAdmin ? (customFieldErrors[field.slug] ?? '') : ''"
          :disabled="!isAdmin || state === 'loading'"
          :system-field="true"
          @update:model-value="isAdmin ? (customFields[field.slug] = ($event as unknown)) : undefined"
          @blur="isAdmin ? validateCustomFieldOnBlur(field.slug) : undefined"
        />
      </div>

      <div class="create-user-modal__actions">
        <Button
          type="button"
          variant="neutral"
          class="create-user-modal__button"
          :disabled="state === 'loading'"
          @click="handleClose"
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          class="create-user-modal__button"
          :is-loading="state === 'loading'"
        >
          {{ state === 'loading' ? 'Creating...' : 'Create' }}
        </Button>
      </div>
    </form>

    <div v-else class="create-user-modal__success">
      <p class="create-user-modal__success-title">User created successfully</p>
      <div class="create-user-modal__password-section">
        <label class="create-user-modal__label">Password</label>
        <div class="create-user-modal__password-row">
          <code class="create-user-modal__password">{{ generatedPassword }}</code>
          <Button
            type="button"
            variant="ghost"
            class="create-user-modal__copy-button"
            :aria-label="passwordCopied ? 'Copied' : 'Copy password'"
            @click="copyPassword"
          >
            {{ passwordCopied ? 'Copied!' : 'Copy' }}
          </Button>
        </div>
        <p class="create-user-modal__warning">
          Please share this password securely. It will not be shown again.
        </p>
      </div>
      <div class="create-user-modal__actions">
        <Button
          type="button"
          variant="primary"
          class="create-user-modal__button"
          @click="handleClose"
        >
          Done
        </Button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.create-user-modal {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.create-user-modal__error {
  margin: 0;
  color: var(--color-error-dark);
  font-size: 0.875rem;
}

.create-user-modal__field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.create-user-modal__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-2);
}

.create-user-modal__input {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  min-height: 44px;
}

.create-user-modal__input:focus {
  outline: 2px solid var(--brand-primary);
  outline-offset: 1px;
  border-color: var(--brand-primary);
}

.create-user-modal__input--error {
  border-color: var(--color-error-dark);
}

.create-user-modal__field-error {
  font-size: 0.8125rem;
  color: var(--color-error-dark);
}

.create-user-modal__section-divider {
  height: 1px;
  background-color: var(--brand-light-2);
  margin: 0.25rem 0;
}

.create-user-modal__fields-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.create-user-modal__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  margin-top: 0.5rem;
}

.create-user-modal__button {
  min-width: 80px;
}

.create-user-modal__copy-button {
  flex-shrink: 0;
  white-space: nowrap;
}

.create-user-modal__success {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.create-user-modal__success-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--brand-dark-1);
}

.create-user-modal__password-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.create-user-modal__password-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.create-user-modal__password {
  flex: 1;
  padding: 0.5rem 0.75rem;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-family: monospace;
  word-break: break-all;
  user-select: all;
}

.create-user-modal__copy-button {
  flex-shrink: 0;
  white-space: nowrap;
}

.create-user-modal__warning {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-warning-dark);
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-bg);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
}

@media (max-width: 639px) {
  .create-user-modal__actions {
    flex-direction: column-reverse;
  }

  .create-user-modal__button {
    width: 100%;
  }
}
</style>
