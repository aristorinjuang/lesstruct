<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Modal from './Modal.vue'
import CustomFieldRenderer from '@/components/molecules/CustomFieldRenderer.vue'
import { useUserStore } from '@/stores/domain/user'
import { useAuth } from '@/composables/useAuth'
import { validateCustomField, validateCustomFields } from '@/utils/validation'
import type { UserRole } from '@/types/user'
import type { FieldSchema } from '@/types/customfield'

const props = defineProps<{
  isOpen: boolean
  userId: string
  userFields: FieldSchema[]
  userSystemFields: FieldSchema[]
}>()

const emit = defineEmits<{
  close: []
  updated: []
}>()

const userStore = useUserStore()
const { role: currentUserRole } = useAuth()

const isAdmin = computed(() => currentUserRole.value === 'Admin')

type FormState = 'form' | 'loading'
type FormErrors = Record<string, string>

const state = ref<FormState>('form')
const name = ref('')
const email = ref('')
const role = ref<UserRole>('Commentator')
const errors = ref<FormErrors>({})
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const customFields = ref<Record<string, any>>({})
const customFieldErrors = ref<Record<string, string>>({})

const roles: { value: UserRole; label: string }[] = [
  { value: 'Admin', label: 'Admin' },
  { value: 'Contributor', label: 'Contributor' },
  { value: 'Commentator', label: 'Commentator' },
]

const emailPattern = /^[a-zA-Z0-9](?:[a-zA-Z0-9._%+-]*[a-zA-Z0-9])?@[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?\.[a-zA-Z]{2,}$/

const editingUser = computed(() =>
  userStore.users.find(u => String(u.id) === String(props.userId))
)

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getDefaultFieldValue(field: FieldSchema): any {
  switch (field.type) {
    case 'checkbox': return false
    case 'number': return undefined
    default: return ''
  }
}

function populateForm() {
  const user = editingUser.value
  if (!user) {
    errors.value = { general: 'User not found. They may have been deleted.' }
    return
  }
  name.value = user.name || ''
  email.value = user.email || ''
  role.value = user.role as UserRole || 'Commentator'
  customFields.value = { ...user.customFields }
  customFieldErrors.value = {}
  errors.value = {}
  state.value = 'form'
}

function resetForm() {
  name.value = ''
  email.value = ''
  role.value = 'Commentator'
  customFields.value = {}
  customFieldErrors.value = {}
  errors.value = {}
  state.value = 'form'
}

watch(() => props.isOpen, (open) => {
  if (open) {
    populateForm()
  } else {
    resetForm()
  }
})

function validate(): boolean {
  const newErrors: FormErrors = {}

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

async function handleSubmit() {
  if (!validate()) return

  state.value = 'loading'

  try {
    await userStore.updateUser(props.userId, {
      name: name.value,
      email: email.value,
      role: role.value,
      customFields: Object.keys(customFields.value).length > 0
        ? customFields.value
        : undefined,
    })
    emit('updated')
    emit('close')
  } catch (err: unknown) {
    state.value = 'form'
    const apiErr = err as { statusCode?: number; code?: string; message?: string }
    if (!apiErr?.statusCode) {
      errors.value = { general: 'Unable to connect to server. Please check your connection.' }
      return
    }
    const code = apiErr?.code
    const errorMap: Record<string, string> = {
      VALIDATION_ERROR: 'Invalid email format',
      INVALID_ROLE: 'Invalid user role',
      EMAIL_EXISTS: 'Email address already in use by another user',
      USER_NOT_FOUND: 'User not found',
    }
    errors.value = { general: errorMap[code] || apiErr?.message || 'Failed to update user' }
  }
}

function handleClose() {
  if (state.value === 'loading') return
  resetForm()
  emit('close')
}
</script>

<template>
  <Modal :is-open="isOpen" title="Edit User" @close="handleClose">
    <form class="edit-user-modal" @submit.prevent="handleSubmit">
      <p v-if="errors.general" class="edit-user-modal__error">{{ errors.general }}</p>

      <div class="edit-user-modal__field">
        <label class="edit-user-modal__label">Username</label>
        <input
          class="edit-user-modal__input edit-user-modal__input--disabled"
          :value="editingUser?.username"
          disabled
        />
      </div>

      <div class="edit-user-modal__field">
        <label for="edit-name" class="edit-user-modal__label">Name</label>
        <input
          id="edit-name"
          v-model="name"
          type="text"
          class="edit-user-modal__input"
          autocomplete="off"
          :disabled="state === 'loading'"
          placeholder="Optional"
        />
      </div>

      <div class="edit-user-modal__field">
        <label for="edit-email" class="edit-user-modal__label">Email</label>
        <input
          id="edit-email"
          v-model="email"
          type="email"
          class="edit-user-modal__input"
          :class="{ 'edit-user-modal__input--error': errors.email }"
          autocomplete="off"
          :disabled="state === 'loading'"
        />
        <span v-if="errors.email" class="edit-user-modal__field-error">{{ errors.email }}</span>
      </div>

      <div class="edit-user-modal__field">
        <label for="edit-role" class="edit-user-modal__label">Role</label>
        <select
          id="edit-role"
          v-model="role"
          class="edit-user-modal__input"
          :disabled="state === 'loading'"
        >
          <option v-for="r in roles" :key="r.value" :value="r.value">{{ r.label }}</option>
        </select>
      </div>

      <!-- Custom fields validation summary -->
      <div
        v-if="Object.keys(customFieldErrors).length > 0"
        class="edit-user-modal__fields-section"
      >
        <div class="edit-user-modal__error">
          Please fix the following:
          <ul>
            <li v-for="(msg, slug) in customFieldErrors" :key="slug">
              {{ msg }}
            </li>
          </ul>
        </div>
      </div>

      <!-- Custom fields section -->
      <div v-if="userFields.length > 0" class="edit-user-modal__fields-section">
        <div class="edit-user-modal__section-divider"></div>
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

      <!-- System fields section -->
      <div v-if="userSystemFields.length > 0" class="edit-user-modal__fields-section">
        <div class="edit-user-modal__section-divider"></div>
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

      <div class="edit-user-modal__actions">
        <button
          type="button"
          class="edit-user-modal__button edit-user-modal__button--cancel"
          :disabled="state === 'loading'"
          @click="handleClose"
        >
          Cancel
        </button>
        <button
          type="submit"
          class="edit-user-modal__button edit-user-modal__button--save"
          :disabled="state === 'loading'"
        >
          {{ state === 'loading' ? 'Saving...' : 'Save Changes' }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.edit-user-modal {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.edit-user-modal__error {
  margin: 0;
  color: var(--color-error-dark);
  font-size: 0.875rem;
}

.edit-user-modal__field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.edit-user-modal__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-2);
}

.edit-user-modal__input {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  min-height: 44px;
}

.edit-user-modal__input:focus {
  outline: 2px solid var(--brand-primary);
  outline-offset: 1px;
  border-color: var(--brand-primary);
}

.edit-user-modal__input--error {
  border-color: var(--color-error-dark);
}

.edit-user-modal__input--disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.edit-user-modal__field-error {
  font-size: 0.8125rem;
  color: var(--color-error-dark);
}

.edit-user-modal__section-divider {
  height: 1px;
  background-color: var(--brand-light-2);
  margin: 0.25rem 0;
}

.edit-user-modal__fields-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.edit-user-modal__fields-loading {
  margin: 0;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  opacity: 0.7;
}

.edit-user-modal__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  margin-top: 0.5rem;
}

.edit-user-modal__button {
  padding: 0.625rem 1rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
  border: 1px solid transparent;
  min-height: 44px;
  min-width: 80px;
}

.edit-user-modal__button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.edit-user-modal__button--cancel {
  background-color: var(--brand-light-1);
  color: var(--brand-dark-2);
  border-color: var(--brand-light-2);
}

.edit-user-modal__button--cancel:hover:not(:disabled) {
  background-color: var(--brand-light-2);
}

.edit-user-modal__button--save {
  background-color: var(--brand-primary);
  color: white;
}

.edit-user-modal__button--save:hover:not(:disabled) {
  background-color: var(--color-interactive-hover);
}

.edit-user-modal__button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

@media (max-width: 639px) {
  .edit-user-modal__actions {
    flex-direction: column-reverse;
  }

  .edit-user-modal__button {
    width: 100%;
  }
}
</style>
