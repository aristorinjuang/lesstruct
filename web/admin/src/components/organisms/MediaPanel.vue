<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useMediaStore, type Media } from '@/stores/domain/media'
import MediaThumbnail from '@/components/molecules/MediaThumbnail.vue'
import DuplicateMediaDialog from '@/components/organisms/DuplicateMediaDialog.vue'
import GenerateImageModal from '@/components/organisms/GenerateImageModal.vue'
import Button from '@/components/atoms/Button.vue'
import api from '@/utils/request'

interface Props {
  isOpen?: boolean
}

interface Emits {
  (e: 'insert-image', media: Media): void
  (e: 'show-toast', message: string, type: string): void
}

const props = withDefaults(defineProps<Props>(), {
  isOpen: false
})

const emit = defineEmits<Emits>()

const mediaStore = useMediaStore()

const aiGenerationAvailable = ref(false)
const showGenerateModal = ref(false)

const altTextInput = ref('')
const fileInput = ref<HTMLInputElement>()
const isUploading = ref(false)
const errorMessage = ref('')

const showDuplicateDialog = ref(false)
const duplicateMedia = ref<Media | null>(null)
const pendingFile = ref<File | null>(null)
const pendingAltText = ref('')

const displayedMedia = computed(() => {
  return mediaStore.media
})

watch(() => props.isOpen, async (isOpen) => {
  if (isOpen) {
    await loadMedia()
  }
})

async function loadMedia() {
  try {
    await mediaStore.fetchMedia()
  } catch (error: unknown) {
    const err = error as { response?: { data?: { error?: { message?: string } } } }
    errorMessage.value = err.response?.data?.error?.message || 'Failed to load media'
  }
}

async function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  if (!file.type.startsWith('image/')) {
    errorMessage.value = 'Please select an image file (JPG, PNG, GIF)'
    return
  }

  if (file.size > 10 * 1024 * 1024) {
    errorMessage.value = 'File size exceeds 10MB limit. Please upload a smaller image'
    return
  }

  if (!altTextInput.value.trim()) {
    errorMessage.value = 'Alt text is required for accessibility'
    return
  }

  isUploading.value = true
  errorMessage.value = ''
  pendingFile.value = file
  pendingAltText.value = altTextInput.value.trim()

  try {
    await mediaStore.upload(file, altTextInput.value.trim())
    clearUploadForm()
  } catch (error: unknown) {
    const err = error as any
    if (err.duplicate && err.existingMedia) {
      duplicateMedia.value = err.existingMedia
      showDuplicateDialog.value = true
    } else {
      errorMessage.value = err.message || 'Failed to upload image'
    }
  } finally {
    isUploading.value = false
  }
}

function handleUseExisting(media: Media) {
  emit('insert-image', media)
  emit('show-toast', 'Using existing image from media library', 'success')
  closeDuplicateDialog()
  clearUploadForm()
}

async function handleUploadAnyway() {
  if (!pendingFile.value || !pendingAltText.value) return

  isUploading.value = true

  try {
    const result = await mediaStore.upload(pendingFile.value, pendingAltText.value, { force: true })
    closeDuplicateDialog()
    clearUploadForm()
    emit('show-toast', `Image uploaded as ${result.originalFilename}`, 'success')
  } catch (error: unknown) {
    const err = error as any
    errorMessage.value = err.message || 'Failed to upload image'
  } finally {
    isUploading.value = false
  }
}

function closeDuplicateDialog() {
  showDuplicateDialog.value = false
  duplicateMedia.value = null
  pendingFile.value = null
  pendingAltText.value = ''
}

function clearUploadForm() {
  altTextInput.value = ''
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

function triggerFileInput() {
  fileInput.value?.click()
}

function handleInsert(media: Media) {
  emit('insert-image', media)
}

async function onAIImageGenerated() {
  showGenerateModal.value = false
  await loadMedia()
  emit('show-toast', 'Image generated successfully', 'success')
}

function onAIError(message: string) {
  emit('show-toast', message, 'error')
}

onMounted(async () => {
  if (props.isOpen) {
    loadMedia()
  }
  try {
    const health = await api.get<{ features?: { imageGeneration?: boolean } }>('/api/health')
    aiGenerationAvailable.value = health.data.features?.imageGeneration === true
  } catch {
    aiGenerationAvailable.value = false
  }
})
</script>

<template>
  <div v-if="isOpen" class="media-panel">
    <!-- Upload Section -->
    <div class="media-panel__upload">
      <div class="media-panel__input-group">
        <input
          ref="fileInput"
          type="file"
          accept="image/jpeg,image/png,image/gif"
          class="media-panel__file-input"
          :disabled="isUploading"
          @change="handleFileSelect"
        />
        <input
          v-model="altTextInput"
          type="text"
          placeholder="Alt text (required for accessibility)"
          class="media-panel__alt-input"
          :disabled="isUploading"
          @keydown.enter="triggerFileInput"
        />
        <Button
          type="button"
          variant="primary"
          :is-loading="isUploading"
          :disabled="isUploading || !altTextInput.trim()"
          @click="triggerFileInput"
        >
          Upload Image
        </Button>
        <Button
          v-if="aiGenerationAvailable"
          type="button"
          variant="secondary"
          @click="showGenerateModal = true"
        >
          Generate with AI
        </Button>
      </div>

      <!-- Error Message -->
      <div v-if="errorMessage" class="media-panel__error">
        {{ errorMessage }}
        <button
          type="button"
          class="media-panel__error-close"
          @click="errorMessage = ''"
          aria-label="Close error"
        >
          ×
        </button>
      </div>
    </div>

    <!-- Media Grid -->
    <div v-if="displayedMedia.length > 0" class="media-panel__grid">
      <MediaThumbnail
        v-for="item in displayedMedia"
        :key="item.id"
        :media="item"
        @insert="handleInsert"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="media-panel__empty">
      <svg
        class="media-panel__empty-icon"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
        />
      </svg>
      <p class="media-panel__empty-text">No images uploaded yet</p>
      <p class="media-panel__empty-hint">
        Upload an image to get started
      </p>
    </div>

    <!-- Duplicate Media Dialog -->
    <DuplicateMediaDialog
      :visible="showDuplicateDialog"
      :existing-media="duplicateMedia"
      :show-use-existing="true"
      @use-existing="handleUseExisting"
      @upload-anyway="handleUploadAnyway"
      @close="closeDuplicateDialog"
    />

    <!-- Generate with AI Modal -->
    <GenerateImageModal
      :is-open="showGenerateModal"
      @close="showGenerateModal = false"
      @generated="onAIImageGenerated"
      @error="onAIError"
    />
  </div>
</template>

<style scoped>
.media-panel {
  border-bottom: 1px solid var(--brand-light-2, #e5e7eb);
  padding: 1rem;
  background-color: var(--color-background-soft, #f9fafb);
}

.media-panel__upload {
  margin-bottom: 1rem;
}

.media-panel__input-group {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.media-panel__file-input {
  display: none;
}

.media-panel__alt-input {
  flex: 1;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  background-color: var(--color-background, #fff);
  color: var(--color-text, #111827);
  transition: border-color 0.15s ease-in-out;
}

.media-panel__alt-input:focus {
  outline: none;
  border-color: var(--color-info);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.media-panel__alt-input:disabled {
  background-color: var(--brand-light-2, #f3f4f6);
  cursor: not-allowed;
}

.media-panel__error {
  position: relative;
  margin-top: 0.5rem;
  padding: 0.75rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.media-panel__error-close {
  background: none;
  border: none;
  font-size: 1.25rem;
  color: var(--color-error-dark);
  cursor: pointer;
  padding: 0;
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.25rem;
}

.media-panel__error-close:hover {
  background-color: rgba(0, 0, 0, 0.05);
}

.media-panel__grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}

.media-panel__empty {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--color-text-muted);
}

.media-panel__empty-icon {
  width: 4rem;
  height: 4rem;
  margin: 0 auto 1rem;
  color: var(--color-border-strong);
}

.media-panel__empty-text {
  font-size: 1rem;
  font-weight: 500;
  margin: 0 0 0.5rem;
  color: var(--brand-dark-1, #374151);
}

.media-panel__empty-hint {
  font-size: 0.875rem;
  margin: 0;
  color: var(--brand-dark-2, #6b7280);
}

[data-theme='dark'] .media-panel__alt-input {
  border-color: var(--brand-light-2, #374151);
  background-color: var(--color-background-soft, #1e293b);
  color: var(--color-text, #e5e7eb);
}

[data-theme='dark'] .media-panel__alt-input:disabled {
  background-color: var(--brand-light-2, #111827);
}

[data-theme='dark'] .media-panel__empty-icon {
  color: var(--color-text-secondary);
}

@media (max-width: 768px) {
  .media-panel__input-group {
    flex-direction: column;
    align-items: stretch;
  }

  .media-panel__grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
  }

  .media-panel__empty {
    padding: 2rem 1rem;
  }
}
</style>
