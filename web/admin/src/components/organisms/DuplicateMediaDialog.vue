<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import Modal from './Modal.vue'
import type { Media } from '@/stores/domain/media'

interface Props {
  visible: boolean
  existingMedia: Media | null
  showUseExisting?: boolean
}

interface Emits {
  (e: 'use-existing', media: Media): void
  (e: 'upload-anyway'): void
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  showUseExisting: true,
})

const emit = defineEmits<Emits>()

const useExistingButton = ref<HTMLButtonElement>()

watch(() => props.visible, async (visible) => {
  if (visible && props.showUseExisting) {
    await nextTick()
    useExistingButton.value?.focus()
  }
})

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function handleUseExisting() {
  if (props.existingMedia) {
    emit('use-existing', props.existingMedia)
  }
}

function handleUploadAnyway() {
  emit('upload-anyway')
}

function handleClose() {
  emit('close')
}
</script>

<template>
  <Modal :is-open="visible" title="Duplicate Image Detected" @close="handleClose">
    <div class="duplicate-dialog">
      <div class="duplicate-dialog__warning">
        <svg
          class="duplicate-dialog__warning-icon"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
          />
        </svg>
        <p class="duplicate-dialog__warning-text">
          This image already exists in your media library
        </p>
      </div>

      <div v-if="existingMedia" class="duplicate-dialog__preview">
        <img
          :src="existingMedia.url"
          :alt="existingMedia.altText || existingMedia.originalFilename"
          class="duplicate-dialog__thumbnail"
        />
        <div class="duplicate-dialog__details">
          <p class="duplicate-dialog__filename">{{ existingMedia.originalFilename }}</p>
          <p v-if="existingMedia.fileSize" class="duplicate-dialog__size">
            {{ formatFileSize(existingMedia.fileSize) }}
          </p>
        </div>
      </div>

      <div class="duplicate-dialog__actions">
        <button
          type="button"
          class="duplicate-dialog__button duplicate-dialog__button--cancel"
          @click="handleClose"
          aria-label="Cancel"
        >
          Cancel
        </button>
        <button
          v-if="showUseExisting"
          ref="useExistingButton"
          type="button"
          class="duplicate-dialog__button duplicate-dialog__button--primary"
          @click="handleUseExisting"
          aria-label="Use existing image"
        >
          Use Existing
        </button>
        <button
          type="button"
          class="duplicate-dialog__button duplicate-dialog__button--secondary"
          @click="handleUploadAnyway"
          aria-label="Upload anyway"
        >
          Upload Anyway
        </button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.duplicate-dialog {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.duplicate-dialog__warning {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.875rem;
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-bg);
  border-radius: 0.5rem;
}

.duplicate-dialog__warning-icon {
  width: 1.25rem;
  height: 1.25rem;
  color: var(--color-warning-dark);
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.duplicate-dialog__warning-text {
  margin: 0;
  font-size: 0.875rem;
  color: var(--color-warning-dark);
  line-height: 1.5;
}

.duplicate-dialog__preview {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.875rem;
  background-color: var(--color-background-soft, #f9fafb);
  border: 1px solid var(--brand-light-2, #e5e7eb);
  border-radius: 0.5rem;
}

.duplicate-dialog__thumbnail {
  width: 4rem;
  height: 4rem;
  object-fit: cover;
  border-radius: 0.375rem;
  border: 1px solid var(--brand-light-2, #e5e7eb);
}

.duplicate-dialog__details {
  flex: 1;
  min-width: 0;
}

.duplicate-dialog__filename {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1, #374151);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.duplicate-dialog__size {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  color: var(--brand-dark-2, #6b7280);
}

.duplicate-dialog__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}

.duplicate-dialog__button {
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

.duplicate-dialog__button--primary {
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
  order: -1;
}

.duplicate-dialog__button--primary:hover {
  background-color: var(--brand-primary-hover);
}

.duplicate-dialog__button--secondary {
  background-color: var(--brand-light-1);
  color: var(--brand-dark-2);
  border-color: var(--brand-light-2);
}

.duplicate-dialog__button--secondary:hover {
  background-color: var(--brand-light-2);
}

.duplicate-dialog__button--cancel {
  background-color: transparent;
  color: var(--brand-dark-2);
}

.duplicate-dialog__button--cancel:hover {
  background-color: var(--brand-light-1);
}

.duplicate-dialog__button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

[data-theme='dark'] .duplicate-dialog__warning {
  background-color: var(--color-warning-bg);
  border-color: var(--color-warning-border);
}

[data-theme='dark'] .duplicate-dialog__warning-icon {
  color: var(--color-warning-border);
}

[data-theme='dark'] .duplicate-dialog__warning-text {
  color: var(--color-warning-bg);
}

[data-theme='dark'] .duplicate-dialog__preview {
  background-color: var(--color-background-soft, #1e293b);
  border-color: var(--brand-light-2, #374151);
}

@media (max-width: 639px) {
  .duplicate-dialog__actions {
    flex-direction: column-reverse;
  }

  .duplicate-dialog__button {
    width: 100%;
  }

  .duplicate-dialog__button--primary {
    order: 0;
  }
}
</style>
