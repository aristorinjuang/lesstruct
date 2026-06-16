<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { Media, MediaVariant } from '@/stores/domain/media'
import { useAuth } from '@/composables/useAuth'

interface Props {
  media: Media | null
}

interface Emits {
  (e: 'close'): void
  (e: 'delete'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { userId, role } = useAuth()

const canDelete = computed(() => {
  if (!props.media) return false
  return role.value === 'Admin' || props.media.userId === userId.value
})

const variantEntries = computed<[string, MediaVariant][] | null>(() => {
  if (!props.media?.variants || Object.keys(props.media.variants).length === 0) {
    return null
  }
  return Object.entries(props.media.variants).sort(([a], [b]) => a.localeCompare(b))
})

const copiedSuffix = ref<string | null>(null)
const copyTimer = ref<ReturnType<typeof setTimeout> | null>(null)

async function copyVariantUrl(url: string, suffix: string) {
  let success = false
  try {
    await navigator.clipboard.writeText(url)
    success = true
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = url
    textarea.style.position = 'fixed'
    textarea.style.top = '-9999px'
    textarea.style.left = '-9999px'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    success = document.execCommand('copy')
    document.body.removeChild(textarea)
  }
  if (success) {
    if (copyTimer.value) clearTimeout(copyTimer.value)
    copiedSuffix.value = suffix
    copyTimer.value = setTimeout(() => { copiedSuffix.value = null }, 2000)
  }
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function formatDimensions(width: number, height: number): string {
  return `${width} x ${height}`
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

function handleDownload() {
  if (props.media?.url) {
    const link = document.createElement('a')
    link.href = props.media.url
    link.download = props.media.originalFilename
    link.target = '_blank'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }
}

function handleDelete() {
  emit('delete')
}

function handleBackdropClick(event: MouseEvent) {
  if (event.target === event.currentTarget) {
    emit('close')
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  document.body.style.overflow = 'hidden'
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.body.style.overflow = ''
  if (copyTimer.value) clearTimeout(copyTimer.value)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="media"
      class="media-detail-modal__backdrop"
      @click="handleBackdropClick"
    >
      <div class="media-detail-modal" role="dialog" aria-label="Media details">
        <button
          type="button"
          class="media-detail-modal__close"
          aria-label="Close"
          @click="$emit('close')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="20" height="20">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>

        <div class="media-detail-modal__image-wrapper">
          <img
            :src="media.url"
            :alt="media.altText"
            class="media-detail-modal__image"
          />
        </div>

        <div class="media-detail-modal__info">
          <h2 class="media-detail-modal__filename" :title="media.originalFilename">
            {{ media.originalFilename }}
          </h2>

          <div class="media-detail-modal__metadata">
            <div class="media-detail-modal__meta-item">
              <span class="media-detail-modal__meta-label">Uploaded</span>
              <span class="media-detail-modal__meta-value">{{ formatDateTime(media.createdAt) }}</span>
            </div>
            <div class="media-detail-modal__meta-item">
              <span class="media-detail-modal__meta-label">File size</span>
              <span class="media-detail-modal__meta-value">{{ formatFileSize(media.fileSize) }}</span>
            </div>
            <div class="media-detail-modal__meta-item">
              <span class="media-detail-modal__meta-label">Dimensions</span>
              <span class="media-detail-modal__meta-value">{{ formatDimensions(media.width, media.height) }}</span>
            </div>
            <div v-if="media.uploadedBy" class="media-detail-modal__meta-item">
              <span class="media-detail-modal__meta-label">Uploaded by</span>
              <span class="media-detail-modal__meta-value">{{ media.uploadedBy }}</span>
            </div>
          </div>

          <div v-if="variantEntries" class="media-detail-modal__variants">
            <h3 class="media-detail-modal__variants-title">Variants</h3>
            <div
              v-for="[suffix, variant] in variantEntries"
              :key="suffix"
              class="media-detail-modal__variant-row"
            >
              <span class="media-detail-modal__variant-suffix">{{ suffix }}</span>
              <span class="media-detail-modal__variant-dims">{{ variant.width }} &times; {{ variant.height }}</span>
              <button
                type="button"
                class="media-detail-modal__btn media-detail-modal__btn--copy"
                @click="copyVariantUrl(variant.url, suffix)"
              >
                {{ copiedSuffix === suffix ? 'Copied!' : 'Copy URL' }}
              </button>
            </div>
          </div>

          <div class="media-detail-modal__actions">
            <button
              type="button"
              class="media-detail-modal__btn media-detail-modal__btn--download"
              @click="handleDownload"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
                <path d="M10.75 2.75a.75.75 0 00-1.5 0v8.614L6.295 8.235a.75.75 0 10-1.09 1.03l4.25 4.5a.75.75 0 001.09 0l4.25-4.5a.75.75 0 00-1.09-1.03l-2.955 3.129V2.75z" />
                <path d="M3.5 12.75a.75.75 0 00-1.5 0v2.5A2.75 2.75 0 004.75 18h10.5A2.75 2.75 0 0018 15.25v-2.5a.75.75 0 00-1.5 0v2.5c0 .69-.56 1.25-1.25 1.25H4.75c-.69 0-1.25-.56-1.25-1.25v-2.5z" />
              </svg>
              Download
            </button>
            <button
              v-if="canDelete"
              type="button"
              class="media-detail-modal__btn media-detail-modal__btn--delete"
              @click="handleDelete"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
                <path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022 1.005 10.74A3.75 3.75 0 009.038 19.5h1.924a3.75 3.75 0 003.719-3.559l1.005-10.74.149.022a.75.75 0 10.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 00-1.5.06l.3 7.5a.75.75 0 101.5-.06l-.3-7.5zm4.34.06a.75.75 0 10-1.5-.06l-.3 7.5a.75.75 0 101.5.06l.3-7.5z" clip-rule="evenodd" />
              </svg>
              Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.media-detail-modal__backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
  padding: 1rem;
}

.media-detail-modal {
  position: relative;
  background-color: var(--color-background, #fff);
  border-radius: 0.75rem;
  max-width: 640px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
}

.media-detail-modal__close {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  z-index: 10;
  background-color: rgba(255, 255, 255, 0.9);
  border: none;
  border-radius: 50%;
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--brand-dark-2, #374151);
  transition: background-color 0.15s;
}

.media-detail-modal__close:hover {
  background-color: var(--color-bg-muted);}

.media-detail-modal__image-wrapper {
  border-radius: 0.75rem 0.75rem 0 0;
  overflow: hidden;
  background-color: var(--color-bg-muted);
  align-items: center;
  justify-content: center;
  max-height: 400px;
}

.media-detail-modal__image {
  width: 100%;
  height: 100%;
  object-fit: contain;
  max-height: 400px;
}

.media-detail-modal__info {
  padding: 1.5rem;
}

.media-detail-modal__filename {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--brand-dark-1, #111827);
  margin: 0 0 1rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-detail-modal__metadata {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}

.media-detail-modal__meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.875rem;
}

.media-detail-modal__meta-label {
  color: var(--brand-dark-2, #6b7280);
}

.media-detail-modal__meta-value {
  color: var(--brand-dark-1, #111827);
  font-weight: 500;
}

.media-detail-modal__variants {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
  padding: 0.75rem;
  border: 1px solid var(--brand-light-2, #e5e7eb);
  border-radius: 0.375rem;
  background-color: var(--color-surface, #f9fafb);
}

.media-detail-modal__variants-title {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--brand-dark-2, #6b7280);
  margin: 0 0 0.25rem;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.media-detail-modal__variant-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
}

.media-detail-modal__variant-suffix {
  font-weight: 600;
  color: var(--brand-dark-1, #111827);
  min-width: 5rem;
  font-family: monospace;
}

.media-detail-modal__variant-dims {
  color: var(--brand-dark-2, #6b7280);
  flex: 1;
}

.media-detail-modal__btn--copy {
  background-color: var(--brand-primary, #22d3ee);
  color: var(--brand-dark-1, #111827);
  min-width: 44px;
  min-height: 44px;
  font-size: 0.75rem;
}

.media-detail-modal__btn--copy:hover {
  background-color: var(--brand-primary-hover);
}

.media-detail-modal__actions {
  display: flex;
  gap: 0.75rem;
}

.media-detail-modal__btn {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.15s;
}

.media-detail-modal__btn--download {
  background-color: var(--brand-dark-1, #111827);
  color: var(--color-white);
}

.media-detail-modal__btn--download:hover {
  background-color: var(--brand-dark-2, #374151);
}

.media-detail-modal__btn--delete {
  background-color: var(--color-error-bg);
  color: var(--color-error);
  border: 1px solid var(--color-error-border);
}

.media-detail-modal__btn--delete:hover {
  background-color: var(--color-error-bg);
}

[data-theme='dark'] .media-detail-modal {
  background-color: var(--color-background, #1f2937);
}

[data-theme='dark'] .media-detail-modal__close {
  background-color: rgba(31, 41, 55, 0.9);
  color: var(--brand-dark-2, #e5e7eb);
}

[data-theme='dark'] .media-detail-modal__close:hover {
  background-color: rgba(55, 65, 81, 0.9);
}

[data-theme='dark'] .media-detail-modal__image-wrapper {
  background-color: var(--brand-dark-2);
}

[data-theme='dark'] .media-detail-modal__variants {
  background-color: rgba(31, 41, 55, 0.5);
  border-color: rgba(55, 65, 81, 0.5);
}

[data-theme='dark'] .media-detail-modal__variant-suffix {
  color: var(--brand-light-2);
}

[data-theme='dark'] .media-detail-modal__variant-dims {
  color: var(--color-text-muted);
}

[data-theme='dark'] .media-detail-modal__btn--download {
  background-color: var(--color-info);
  color: var(--color-white);
}

[data-theme='dark'] .media-detail-modal__btn--download:hover {
  background-color: var(--color-info);
}

[data-theme='dark'] .media-detail-modal__btn--delete {
  background-color: rgba(220, 38, 38, 0.15);
  color: #fca5a5;
  border-color: rgba(220, 38, 38, 0.3);
}

[data-theme='dark'] .media-detail-modal__btn--delete:hover {
  background-color: rgba(220, 38, 38, 0.25);
}

@media (max-width: 640px) {
  .media-detail-modal {
    max-width: 100%;
    max-height: 100vh;
    border-radius: 0;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .media-detail-modal__image-wrapper {
    max-height: 300px;
    border-radius: 0;
  }

  .media-detail-modal__image {
    max-height: 300px;
  }

  .media-detail-modal__backdrop {
    padding: 0;
  }

  .media-detail-modal__variant-row {
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .media-detail-modal__variant-suffix {
    min-width: auto;
  }
}
</style>
