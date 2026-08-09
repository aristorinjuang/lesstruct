<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import Modal from './Modal.vue'
import Button from '@/components/atoms/Button.vue'
import { useApiKeys } from '@/composables/useApiKeys'

const props = defineProps<{ isOpen: boolean }>()
const emit = defineEmits<{ close: []; created: [] }>()

const { create } = useApiKeys()

type FormState = 'form' | 'loading' | 'success'
const state = ref<FormState>('form')
const name = ref('')
const errors = ref<{ name?: string; general?: string }>({})
const generatedKey = ref('')
const keyCopied = ref(false)

// Mirrors the backend ValidateKeyName constraint (name <= 120 chars).
const KEY_NAME_MAX_LENGTH = 120

let copyTimer: ReturnType<typeof setTimeout> | null = null

function clearCopyTimer(): void {
  if (copyTimer !== null) {
    clearTimeout(copyTimer)
    copyTimer = null
  }
}

function resetForm() {
  name.value = ''
  errors.value = {}
  generatedKey.value = ''
  keyCopied.value = false
  state.value = 'form'
  clearCopyTimer()
}

// Secret hygiene: wipe the transient key whenever the dialog closes — including
// when the parent toggles `isOpen` to false directly (route change, global
// escape) without emitting a `close` event through the Modal.
watch(
  () => props.isOpen,
  (open) => {
    if (!open) resetForm()
  },
)

onUnmounted(() => {
  clearCopyTimer()
})

function validate(): boolean {
  const e: { name?: string } = {}
  const trimmed = name.value.trim()
  if (!trimmed) {
    e.name = 'Key name is required'
  } else if (trimmed.length > KEY_NAME_MAX_LENGTH) {
    e.name = `Key name must be ${KEY_NAME_MAX_LENGTH} characters or fewer`
  }
  errors.value = e
  return Object.keys(e).length === 0
}

async function handleSubmit() {
  // Guard against re-entry (e.g. Enter key in the input while loading).
  if (state.value === 'loading') return
  if (!validate()) return
  state.value = 'loading'
  try {
    const result = await create(name.value.trim())
    generatedKey.value = result.key
    state.value = 'success'
    emit('created')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (err: any) {
    state.value = 'form'
    if (!err?.statusCode) {
      errors.value = { general: 'Unable to connect to server. Please check your connection.' }
      return
    }
    const errorMap: Record<string, string> = {
      VALIDATION_ERROR: 'Invalid key name or name already in use',
      UNAUTHORIZED: 'You must be signed in to do this',
    }
    errors.value = { general: errorMap[err?.code] || 'Failed to create API key' }
  }
}

function markCopied(): void {
  keyCopied.value = true
  clearCopyTimer()
  copyTimer = setTimeout(() => {
    keyCopied.value = false
    copyTimer = null
  }, 2000)
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(generatedKey.value)
    markCopied()
    return
  } catch {
    // Clipboard API unavailable (insecure context) — fall back to execCommand.
  }
  const textArea = document.createElement('textarea')
  textArea.value = generatedKey.value
  textArea.setAttribute('readonly', '')
  // Move off-screen so the auxiliary element doesn't flash or scroll the page.
  textArea.style.position = 'absolute'
  textArea.style.left = '-9999px'
  document.body.appendChild(textArea)
  textArea.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(textArea)
  if (ok) markCopied()
}

function handleClose() {
  if (state.value === 'loading') return
  // Wipe the transient key from memory before closing (secret hygiene).
  resetForm()
  emit('close')
}
</script>

<template>
  <Modal :is-open="isOpen" title="Create API Key" @close="handleClose">
    <form v-if="state !== 'success'" class="api-key-create-dialog" @submit.prevent="handleSubmit">
      <p v-if="errors.general" class="api-key-create-dialog__error">{{ errors.general }}</p>

      <div class="api-key-create-dialog__field">
        <label for="api-key-name" class="api-key-create-dialog__label">Key name</label>
        <input
          id="api-key-name"
          v-model="name"
          type="text"
          class="api-key-create-dialog__input"
          :class="{ 'api-key-create-dialog__input--error': errors.name }"
          autocomplete="off"
          :disabled="state === 'loading'"
          placeholder="e.g. Production deploy key"
        />
        <span v-if="errors.name" class="api-key-create-dialog__field-error">{{ errors.name }}</span>
      </div>

      <div class="api-key-create-dialog__actions">
        <Button
          type="button"
          variant="neutral"
          class="api-key-create-dialog__button"
          :disabled="state === 'loading'"
          @click="handleClose"
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          class="api-key-create-dialog__button"
          :is-loading="state === 'loading'"
        >
          {{ state === 'loading' ? 'Creating...' : 'Create' }}
        </Button>
      </div>
    </form>

    <div v-else class="api-key-create-dialog__success">
      <p class="api-key-create-dialog__success-title">API key created</p>
      <div class="api-key-create-dialog__key-section">
        <label class="api-key-create-dialog__label">API key</label>
        <div class="api-key-create-dialog__key-row">
          <code class="api-key-create-dialog__key">{{ generatedKey }}</code>
          <Button
            type="button"
            variant="ghost"
            class="api-key-create-dialog__copy-button"
            :aria-label="keyCopied ? 'Copied' : 'Copy API key'"
            @click="copyKey"
          >
            {{ keyCopied ? 'Copied!' : 'Copy' }}
          </Button>
        </div>
        <p class="api-key-create-dialog__warning">
          This key will not be shown again. Copy it now.
        </p>
      </div>
      <div class="api-key-create-dialog__actions">
        <Button
          type="button"
          variant="primary"
          class="api-key-create-dialog__button"
          @click="handleClose"
        >
          Done
        </Button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.api-key-create-dialog {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.api-key-create-dialog__error {
  margin: 0;
  color: var(--color-error-dark);
  font-size: 0.875rem;
}

.api-key-create-dialog__field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.api-key-create-dialog__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-2);
}

.api-key-create-dialog__input {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  min-height: 44px;
}

.api-key-create-dialog__input:focus {
  outline: 2px solid var(--brand-primary);
  outline-offset: 1px;
  border-color: var(--brand-primary);
}

.api-key-create-dialog__input--error {
  border-color: var(--color-error-dark);
}

.api-key-create-dialog__field-error {
  font-size: 0.8125rem;
  color: var(--color-error-dark);
}

.api-key-create-dialog__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
  margin-top: 0.5rem;
}

.api-key-create-dialog__button {
  min-width: 80px;
}

.api-key-create-dialog__success {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.api-key-create-dialog__success-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--brand-dark-1);
}

.api-key-create-dialog__key-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.api-key-create-dialog__key-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.api-key-create-dialog__key {
  flex: 1;
  padding: 0.5rem;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-family: monospace;
  word-break: break-all;
  user-select: all;
}

.api-key-create-dialog__copy-button {
  flex-shrink: 0;
  white-space: nowrap;
}

.api-key-create-dialog__warning {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--color-warning-dark);
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-bg);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
}

@media (max-width: 639px) {
  .api-key-create-dialog__actions {
    flex-direction: column-reverse;
  }

  .api-key-create-dialog__button {
    width: 100%;
  }
}
</style>
