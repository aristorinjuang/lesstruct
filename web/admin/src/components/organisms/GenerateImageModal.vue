<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/organisms/Modal.vue'
import Button from '@/components/atoms/Button.vue'

interface Props {
  isOpen?: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'generated', media: import('@/stores/domain/media').Media): void
  (e: 'error', message: string): void
}

const props = withDefaults(defineProps<Props>(), {
  isOpen: false,
})

const emit = defineEmits<Emits>()

const prompt = ref('')
const isLoading = ref(false)
const errorMessage = ref('')

watch(
  () => props.isOpen,
  (newVal) => {
    if (newVal) {
      prompt.value = ''
      errorMessage.value = ''
      isLoading.value = false
    }
  },
)

async function handleGenerate() {
  const trimmed = prompt.value.trim()
  if (!trimmed) {
    errorMessage.value = 'Please enter a prompt'
    return
  }

  isLoading.value = true
  errorMessage.value = ''

  try {
    const { useMediaStore } = await import('@/stores/domain/media')
    const mediaStore = useMediaStore()
    const media = await mediaStore.generateImage(trimmed)
    emit('generated', media)
    emit('close')
  } catch (err: unknown) {
    const error = err as { message?: string; response?: { data?: { error?: { message?: string } } } }
    errorMessage.value = error.response?.data?.error?.message || error.message || 'Failed to generate image'
    emit('error', errorMessage.value)
  } finally {
    isLoading.value = false
  }
}

function handleCancel() {
  emit('close')
}
</script>

<template>
  <Modal
    :is-open="isOpen"
    title="Generate with AI"
    :close-on-overlay-click="!isLoading"
    :close-on-escape="!isLoading"
    @close="handleCancel"
  >
    <div class="generate-modal__body">
      <label class="generate-modal__label" for="ai-prompt">
        Describe the image you want to generate
      </label>
      <textarea
        id="ai-prompt"
        v-model="prompt"
        class="generate-modal__textarea"
        :disabled="isLoading"
        placeholder="A serene mountain landscape at sunset with glowing orange sky..."
        rows="4"
        maxlength="1000"
        @keydown.enter.meta.exact.prevent="handleGenerate"
      />
      <p class="generate-modal__char-count">
        {{ prompt.length }}/1000
      </p>

      <div v-if="errorMessage" class="generate-modal__error">
        {{ errorMessage }}
      </div>
    </div>

    <template #footer>
      <div class="generate-modal__footer">
        <Button
          type="button"
          variant="secondary"
          :disabled="isLoading"
          @click="handleCancel"
        >
          Cancel
        </Button>
        <Button
          type="button"
          variant="primary"
          :is-loading="isLoading"
          :disabled="isLoading || !prompt.trim()"
          @click="handleGenerate"
        >
          Generate
        </Button>
      </div>
    </template>
  </Modal>
</template>

<style scoped>
.generate-modal__body {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.generate-modal__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.generate-modal__textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-family: inherit;
  color: var(--brand-dark-1);
  background-color: var(--color-background, #fff);
  resize: vertical;
  box-sizing: border-box;
}

.generate-modal__textarea:focus {
  outline: none;
  border-color: var(--color-info);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.generate-modal__textarea:disabled {
  background-color: var(--color-bg-muted);
  cursor: not-allowed;
}

.generate-modal__char-count {
  font-size: 0.75rem;
  color: var(--brand-dark-2, #9ca3af);
  text-align: right;
  margin: 0;
}

.generate-modal__error {
  padding: 0.625rem 0.75rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
  font-size: 0.8125rem;
}

.generate-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
