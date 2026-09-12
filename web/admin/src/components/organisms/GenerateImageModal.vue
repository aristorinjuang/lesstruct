<script setup lang="ts">
import { computed, ref, shallowRef, watch, onUnmounted } from 'vue'
import Modal from '@/components/organisms/Modal.vue'
import Button from '@/components/atoms/Button.vue'
import IconXMark from '@/components/icons/IconXMark.vue'
import IconPhoto from '@/components/icons/IconPhoto.vue'
import type { Media, useMediaStore } from '@/stores/domain/media'

interface Props {
  isOpen?: boolean
  supportsReferences?: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'generated', media: Media, meta: { og: boolean }): void
  (e: 'error', message: string): void
}

interface ReferenceItem {
  key: string
  name: string
  objectUrl: string
  file: File
}

type MediaStore = ReturnType<typeof useMediaStore>

const MAX_REFERENCES = 3
const MAX_REFERENCE_SIZE = 10 * 1024 * 1024
const ACCEPTED_TYPES = 'image/jpeg,image/png,image/webp'

const props = withDefaults(defineProps<Props>(), {
  isOpen: false,
  supportsReferences: false,
})

const emit = defineEmits<Emits>()

const prompt = ref('')
const isLoading = ref(false)
const errorMessage = ref('')
const isOG = ref(false)
const references = ref<ReferenceItem[]>([])
const referenceError = ref('')
const isDragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const isPickerOpen = ref(false)
const pickerStore = shallowRef<MediaStore | null>(null)
const pickerSelected = ref<number[]>([])
const pickerError = ref('')
const pickerLoading = ref(false)
const pickerFetching = ref(false)

let nextReferenceKey = 0

const remainingSlots = computed(() => MAX_REFERENCES - references.value.length)
const pickerMedia = computed<Media[]>(() => pickerStore.value?.media ?? [])

watch(
  () => props.isOpen,
  (newVal) => {
    if (newVal) {
      prompt.value = ''
      errorMessage.value = ''
      isLoading.value = false
      isOG.value = false
      clearReferences()
      closeLibraryPicker()
    }
  },
)

onUnmounted(() => {
  clearReferences()
})

function clearReferences() {
  for (const reference of references.value) {
    URL.revokeObjectURL(reference.objectUrl)
  }
  references.value = []
  referenceError.value = ''
  isDragOver.value = false
}

// addReferenceFiles validates and appends files, returning an error message ('' on success).
function addReferenceFiles(files: FileList | File[] | null | undefined): string {
  if (!files) {
    return ''
  }
  for (const file of Array.from(files)) {
    if (references.value.length >= MAX_REFERENCES) {
      return `You can add up to ${MAX_REFERENCES} reference images`
    }
    if (!file.type.startsWith('image/')) {
      return `"${file.name}" is not an image file (JPG, PNG, WebP)`
    }
    if (file.size > MAX_REFERENCE_SIZE) {
      return `"${file.name}" exceeds the 10MB limit`
    }
    references.value.push({
      key: `reference-${nextReferenceKey++}`,
      name: file.name,
      objectUrl: URL.createObjectURL(file),
      file,
    })
  }
  return ''
}

function removeReference(key: string) {
  const index = references.value.findIndex((reference) => reference.key === key)
  if (index === -1) {
    return
  }
  const [removed] = references.value.splice(index, 1)
  if (removed) {
    URL.revokeObjectURL(removed.objectUrl)
  }
}

function triggerFileInput() {
  fileInput.value?.click()
}

function onFileInputChange(event: Event) {
  const target = event.target as HTMLInputElement
  referenceError.value = addReferenceFiles(target.files)
  target.value = ''
}

function onStripDrop(event: DragEvent) {
  isDragOver.value = false
  referenceError.value = addReferenceFiles(event.dataTransfer?.files)
}

async function openLibraryPicker() {
  pickerError.value = ''
  pickerSelected.value = []
  isPickerOpen.value = true
  if (!pickerStore.value) {
    const { useMediaStore } = await import('@/stores/domain/media')
    pickerStore.value = useMediaStore()
  }
  const store = pickerStore.value
  if (store && store.media.length === 0 && !store.isLoading) {
    pickerFetching.value = true
    try {
      await store.fetchMedia()
    } catch {
      pickerError.value = 'Failed to load the media library'
    } finally {
      pickerFetching.value = false
    }
  }
}

function closeLibraryPicker() {
  isPickerOpen.value = false
  pickerSelected.value = []
  pickerError.value = ''
  pickerLoading.value = false
}

function togglePickerSelection(id: number) {
  const index = pickerSelected.value.indexOf(id)
  if (index >= 0) {
    pickerSelected.value.splice(index, 1)
  } else if (pickerSelected.value.length < remainingSlots.value) {
    pickerSelected.value.push(id)
  }
}

function pickerThumbnail(item: Media): string {
  return item.variants?._thumb?.url || item.url
}

async function confirmLibrarySelection() {
  const store = pickerStore.value
  if (!store || pickerSelected.value.length === 0) {
    return
  }
  pickerError.value = ''
  pickerLoading.value = true
  try {
    for (const id of pickerSelected.value) {
      if (references.value.length >= MAX_REFERENCES) {
        break
      }
      const item = store.media.find((media) => media.id === id)
      if (!item) {
        continue
      }
      let file: File
      try {
        const response = await fetch(item.url)
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`)
        }
        const blob = await response.blob()
        file = new File([blob], item.originalFilename || `reference-${item.id}`, {
          type: blob.type || 'image/webp',
        })
      } catch {
        pickerError.value = `Could not load "${item.originalFilename}" from the library`
        return
      }
      if (addReferenceFiles([file])) {
        break
      }
    }
    referenceError.value = ''
    closeLibraryPicker()
  } finally {
    pickerLoading.value = false
  }
}

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
    const media = await mediaStore.generateImage(
      trimmed,
      references.value.map((reference) => reference.file),
      { og: isOG.value },
    )
    emit('generated', media, { og: isOG.value })
    emit('close')
  } catch (err: unknown) {
    const error = err as {
      message?: string
      response?: { data?: { error?: { message?: string } } }
    }
    errorMessage.value =
      error.response?.data?.error?.message || error.message || 'Failed to generate image'
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
    :title="isPickerOpen ? 'Choose from library' : 'Generate with AI'"
    :size="isPickerOpen ? 'full' : 'md'"
    :close-on-overlay-click="!isLoading && !pickerLoading"
    :close-on-escape="!isLoading && !pickerLoading"
    @close="handleCancel"
  >
    <div v-if="isPickerOpen" class="generate-modal__picker">
      <p v-if="pickerError" class="generate-modal__error">
        {{ pickerError }}
      </p>
      <p v-else-if="pickerFetching" class="generate-modal__hint">Loading media library…</p>
      <p v-else-if="pickerMedia.length === 0" class="generate-modal__hint">
        No images in the media library yet.
      </p>
      <div v-else class="generate-modal__grid">
        <button
          v-for="item in pickerMedia"
          :key="item.id"
          type="button"
          class="generate-modal__grid-item"
          :class="{ 'generate-modal__grid-item--selected': pickerSelected.includes(item.id) }"
          :disabled="
            pickerLoading ||
            (!pickerSelected.includes(item.id) && pickerSelected.length >= remainingSlots)
          "
          :aria-pressed="pickerSelected.includes(item.id)"
          :title="item.originalFilename"
          @click="togglePickerSelection(item.id)"
        >
          <img
            :src="pickerThumbnail(item)"
            :alt="item.altText || item.originalFilename"
            loading="lazy"
          />
          <span
            v-if="pickerSelected.includes(item.id)"
            class="generate-modal__grid-check"
            aria-hidden="true"
            >✓</span
          >
        </button>
      </div>
    </div>

    <div v-else class="generate-modal__body">
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
      <p class="generate-modal__char-count">{{ prompt.length }}/1000</p>

      <div v-if="supportsReferences" class="generate-modal__refs">
        <div class="generate-modal__refs-header">
          <span class="generate-modal__label">
            Image references <span class="generate-modal__optional">· optional</span>
          </span>
          <span class="generate-modal__count">{{ references.length }}/{{ MAX_REFERENCES }}</span>
        </div>
        <div
          class="generate-modal__strip"
          :class="{ 'generate-modal__strip--dragover': isDragOver }"
          @dragover.prevent="isDragOver = true"
          @dragleave="isDragOver = false"
          @drop.prevent="onStripDrop"
        >
          <div
            v-for="reference in references"
            :key="reference.key"
            class="generate-modal__thumb"
            :title="reference.name"
          >
            <img :src="reference.objectUrl" :alt="reference.name" />
            <button
              type="button"
              class="generate-modal__remove"
              :disabled="isLoading"
              :aria-label="`Remove ${reference.name}`"
              @click="removeReference(reference.key)"
            >
              <IconXMark class="generate-modal__remove-icon" />
            </button>
          </div>
          <button
            v-if="remainingSlots > 0"
            type="button"
            class="generate-modal__add"
            :disabled="isLoading"
            @click="triggerFileInput"
          >
            <span class="generate-modal__add-plus" aria-hidden="true">+</span>
            <span>Add</span>
          </button>
          <button
            type="button"
            class="generate-modal__library"
            :disabled="isLoading || remainingSlots === 0"
            @click="openLibraryPicker"
          >
            <IconPhoto class="generate-modal__library-icon" />
            <span>Library</span>
          </button>
        </div>
        <p class="generate-modal__hint">
          Optional: add up to 3 images to guide the style, subject, or composition (JPG, PNG, WebP —
          10 MB max each). You can also drag &amp; drop files here.
        </p>
        <p v-if="referenceError" class="generate-modal__error">
          {{ referenceError }}
        </p>
        <input
          ref="fileInput"
          type="file"
          class="generate-modal__file-input"
          :accept="ACCEPTED_TYPES"
          multiple
          @change="onFileInputChange"
        />
      </div>

      <label class="generate-modal__check">
        <input v-model="isOG" type="checkbox" :disabled="isLoading" />
        <span>
          Open Graph image (1200 × 630)
          <span class="generate-modal__optional">— cropped for social previews</span>
        </span>
      </label>

      <div v-if="errorMessage" class="generate-modal__error">
        {{ errorMessage }}
      </div>
    </div>

    <template #footer>
      <div class="generate-modal__footer">
        <template v-if="isPickerOpen">
          <Button
            type="button"
            variant="neutral"
            :disabled="pickerLoading"
            @click="closeLibraryPicker"
          >
            Back
          </Button>
          <Button
            type="button"
            variant="primary"
            :is-loading="pickerLoading"
            :disabled="pickerLoading || pickerSelected.length === 0"
            @click="confirmLibrarySelection"
          >
            Add{{
              pickerSelected.length > 0
                ? ` ${pickerSelected.length} image${pickerSelected.length > 1 ? 's' : ''}`
                : ''
            }}
          </Button>
        </template>
        <template v-else>
          <Button type="button" variant="neutral" :disabled="isLoading" @click="handleCancel">
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
        </template>
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

.generate-modal__refs {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.generate-modal__refs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.generate-modal__optional {
  font-weight: 400;
  color: var(--brand-dark-2, #9ca3af);
}

.generate-modal__count {
  font-size: 0.75rem;
  color: var(--brand-dark-2, #9ca3af);
}

.generate-modal__strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.5rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  background-color: var(--color-background, #fff);
}

.generate-modal__strip--dragover {
  border-color: var(--color-info);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.generate-modal__thumb {
  position: relative;
  width: 5.5rem;
  height: 5.5rem;
  border-radius: 0.375rem;
  overflow: hidden;
  border: 1px solid var(--brand-light-2, #d1d5db);
  background-color: var(--color-bg-muted);
  flex-shrink: 0;
}

.generate-modal__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.generate-modal__remove {
  position: absolute;
  top: 0.125rem;
  right: 0.125rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  border: none;
  border-radius: 9999px;
  background-color: rgba(0, 0, 0, 0.65);
  color: #fff;
  cursor: pointer;
}

.generate-modal__remove:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.generate-modal__remove-icon {
  width: 0.75rem;
  height: 0.75rem;
}

.generate-modal__add,
.generate-modal__library {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.125rem;
  width: 5.5rem;
  height: 5.5rem;
  border: 1px dashed var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  background-color: transparent;
  color: var(--brand-dark-2, #9ca3af);
  font-size: 0.75rem;
  cursor: pointer;
  flex-shrink: 0;
}

.generate-modal__add:hover:not(:disabled),
.generate-modal__library:hover:not(:disabled) {
  border-color: var(--color-info);
  color: var(--color-info);
}

.generate-modal__add:disabled,
.generate-modal__library:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.generate-modal__add-plus {
  font-size: 1.25rem;
  line-height: 1;
}

.generate-modal__library-icon {
  width: 1.25rem;
  height: 1.25rem;
}

.generate-modal__hint {
  font-size: 0.75rem;
  color: var(--brand-dark-2, #9ca3af);
  margin: 0;
}

.generate-modal__file-input {
  display: none;
}

.generate-modal__check {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  cursor: pointer;
}

.generate-modal__check input {
  margin-top: 0.125rem;
  accent-color: var(--color-info);
}

.generate-modal__picker {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
  min-height: 0;
}

.generate-modal__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr));
  gap: 0.75rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0.125rem;
}

.generate-modal__grid-item {
  position: relative;
  aspect-ratio: 1;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 0.375rem;
  overflow: hidden;
  background-color: var(--color-bg-muted);
  cursor: pointer;
}

.generate-modal__grid-item:hover:not(:disabled) {
  border-color: var(--brand-light-2, #d1d5db);
}

.generate-modal__grid-item--selected {
  border-color: var(--color-info);
}

.generate-modal__grid-item:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.generate-modal__grid-item img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.generate-modal__grid-check {
  position: absolute;
  top: 0.125rem;
  right: 0.125rem;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 9999px;
  background-color: var(--color-info);
  color: #fff;
  font-size: 0.75rem;
  line-height: 1;
}
</style>
