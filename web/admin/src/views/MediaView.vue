<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useMediaStore, type Media } from '@/stores/domain/media'
import { groupByDate, type DateGroup } from '@/utils/groupByDate'
import MediaDetailModal from '@/components/organisms/MediaDetailModal.vue'
import DuplicateMediaDialog from '@/components/organisms/DuplicateMediaDialog.vue'
import GenerateImageModal from '@/components/organisms/GenerateImageModal.vue'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'
import api from '@/utils/request'

const mediaStore = useMediaStore()

const aiGenerationAvailable = ref(false)
const showGenerateModal = ref(false)

const searchQuery = ref('')
const dateFilter = ref('')
const selectedMedia = ref<Media | null>(null)
const showUploadForm = ref(false)
const uploadFile = ref<File | null>(null)
const uploadAltText = ref('')
const isUploading = ref(false)
const uploadError = ref('')
const fileInput = ref<HTMLInputElement>()

const showDuplicateDialog = ref(false)
const duplicateMedia = ref<Media | null>(null)
const pendingForceFile = ref<File | null>(null)
const pendingForceAltText = ref('')

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

const gridImgErrors = ref<Record<number, boolean>>({})

function onGridImgError(id: number) {
  gridImgErrors.value = { ...gridImgErrors.value, [id]: true }
}

const showNoResults = computed(() => {
  return !mediaStore.isLoading && mediaStore.media.length === 0 && (searchQuery.value || dateFilter.value)
})

const hasActiveFilters = computed(() => {
  return searchQuery.value || dateFilter.value
})

const showDateSeparators = computed(() => {
  return !searchQuery.value && !dateFilter.value && mediaStore.media.length > 1
})

const mediaGroups = computed<DateGroup[]>(() => {
  if (!showDateSeparators.value) {
    return [{ label: '', items: mediaStore.media }]
  }
  return groupByDate(mediaStore.media)
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(searchQuery, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    loadMedia()
  }, 300)
})

watch(dateFilter, () => {
  loadMedia()
})

async function loadMedia() {
  try {
    await mediaStore.fetchMedia({
      search: searchQuery.value || undefined,
      dateFilter: dateFilter.value || undefined
    })
  } catch {
    // error handled by store
  }
}

function openDetail(media: Media) {
  selectedMedia.value = media
}

function closeDetail() {
  selectedMedia.value = null
}

function handleDelete() {
  if (!selectedMedia.value) return
  const mediaToDelete = selectedMedia.value

  if (!confirm(`Are you sure you want to delete "${mediaToDelete.originalFilename}"? This action cannot be undone.`)) {
    return
  }

  mediaStore.deleteMedia(mediaToDelete.id)
    .then(() => {
      closeDetail()
      displayToast('Media deleted successfully', 'success')
    })
    .catch(() => {
      displayToast('Failed to delete media', 'error')
    })
}

function openUploadForm() {
  showUploadForm.value = true
  uploadError.value = ''
}

function closeUploadForm() {
  showUploadForm.value = false
  uploadFile.value = null
  uploadAltText.value = ''
  uploadError.value = ''
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  if (!file.type.startsWith('image/')) {
    uploadError.value = 'Please select an image file (JPG, PNG, GIF)'
    return
  }

  if (file.size > 10 * 1024 * 1024) {
    uploadError.value = 'File size exceeds 10MB limit'
    return
  }

  uploadFile.value = file
  uploadError.value = ''
}

async function handleUpload() {
  if (!uploadFile.value || !uploadAltText.value.trim()) return

  isUploading.value = true
  uploadError.value = ''

  try {
    await mediaStore.upload(uploadFile.value, uploadAltText.value.trim())
    closeUploadForm()
    displayToast('Media uploaded successfully', 'success')
    await loadMedia()
  } catch (error: unknown) {
    const err = error as any
    if (err.duplicate && err.existingMedia) {
      duplicateMedia.value = err.existingMedia
      pendingForceFile.value = uploadFile.value
      pendingForceAltText.value = uploadAltText.value.trim()
      showDuplicateDialog.value = true
    } else {
      uploadError.value = err.message || 'Failed to upload image'
    }
  } finally {
    isUploading.value = false
  }
}

async function handleUploadAnyway() {
  if (!pendingForceFile.value || !pendingForceAltText.value) return

  isUploading.value = true

  try {
    const result = await mediaStore.upload(pendingForceFile.value, pendingForceAltText.value, { force: true })
    closeDuplicateDialog()
    closeUploadForm()
    displayToast(`Image uploaded as ${result.originalFilename}`, 'success')
    await loadMedia()
  } catch (error: unknown) {
    const err = error as any
    uploadError.value = err.message || 'Failed to upload image'
  } finally {
    isUploading.value = false
  }
}

function closeDuplicateDialog() {
  showDuplicateDialog.value = false
  duplicateMedia.value = null
  pendingForceFile.value = null
  pendingForceAltText.value = ''
}

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

async function onAIImageGenerated() {
  displayToast('Image generated successfully', 'success')
  showGenerateModal.value = false
  await loadMedia()
}

function onAIError(message: string) {
  displayToast(message, 'error')
}

function clearSearch() {
  searchQuery.value = ''
}

function clearFilters() {
  searchQuery.value = ''
  dateFilter.value = ''
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString()
}

onMounted(async () => {
  loadMedia()
  try {
    const health = await api.get<{ features?: { imageGeneration?: boolean } }>('/api/health')
    aiGenerationAvailable.value = health.data.features?.imageGeneration === true
  } catch {
    aiGenerationAvailable.value = false
  }
})

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<template>
  <div class="media-view">
    <!-- Header -->
    <div class="page-header">
      <h1 class="page-title">Media Library</h1>
      <div class="media-view__header-actions">
        <Button
          v-if="aiGenerationAvailable"
          type="button"
          variant="secondary"
          @click="showGenerateModal = true"
        >
          Generate with AI
        </Button>
        <Button
          type="button"
          variant="primary"
          @click="openUploadForm"
        >
          Upload Media
        </Button>
      </div>
    </div>

    <!-- Upload Form -->
    <div v-if="showUploadForm" class="media-view__upload-form">
      <div class="media-view__upload-form-header">
        <h3 class="media-view__upload-form-title">Upload New Media</h3>
        <button
          type="button"
          class="media-view__upload-form-close"
          aria-label="Close upload form"
          @click="closeUploadForm"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>

      <div class="media-view__upload-form-body">
        <div class="media-view__upload-inputs">
          <input
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png,image/gif"
            class="media-view__file-input"
            :disabled="isUploading"
            @change="handleFileSelect"
          />
          <button
            type="button"
            class="media-view__choose-file-btn"
            :disabled="isUploading"
            @click="fileInput?.click()"
          >
            {{ uploadFile ? uploadFile.name : 'Choose File' }}
          </button>
          <input
            v-model="uploadAltText"
            type="text"
            placeholder="Alt text (required)"
            class="media-view__alt-input"
            :disabled="isUploading"
          />
          <Button
            type="button"
            variant="primary"
            :is-loading="isUploading"
            :disabled="isUploading || !uploadFile || !uploadAltText.trim()"
            @click="handleUpload"
          >
            Upload
          </Button>
        </div>

        <div v-if="uploadError" class="media-view__upload-error">
          {{ uploadError }}
          <button
            type="button"
            class="media-view__upload-error-close"
            @click="uploadError = ''"
            aria-label="Close error"
          >
            &times;
          </button>
        </div>
      </div>
    </div>

    <!-- Search and Filter -->
    <div class="media-view__controls">
      <div class="media-view__search">
        <div class="search-wrapper">
          <svg class="search-wrapper__icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
            <path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clip-rule="evenodd" />
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search by filename..."
            class="search-wrapper__input"
          />
          <button
            v-if="searchQuery"
            type="button"
            class="search-wrapper__clear"
            aria-label="Clear search"
            @click="clearSearch"
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="14" height="14">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
            </svg>
          </button>
        </div>
      </div>

      <div class="media-view__filter">
        <select
          v-model="dateFilter"
          class="media-view__filter-select"
        >
          <option value="">All Media</option>
          <option value="today">Today</option>
          <option value="this_week">This Week</option>
          <option value="this_month">This Month</option>
        </select>
      </div>

      <button
        v-if="hasActiveFilters"
        type="button"
        class="media-view__clear-filters"
        @click="clearFilters"
      >
        Clear filters
      </button>
    </div>

    <!-- Content -->
    <div class="media-view__content">
      <!-- Loading State -->
      <div v-if="mediaStore.isLoading && mediaStore.media.length === 0" class="media-view__loading">
        <div class="media-view__skeleton-grid">
          <div v-for="i in 8" :key="i" class="media-view__skeleton-item">
            <div class="media-view__skeleton-image" />
            <div class="media-view__skeleton-text" />
          </div>
        </div>
      </div>

      <!-- No Results -->
      <div v-else-if="showNoResults" class="state-empty">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="state-empty__icon">
          <path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
        </svg>
        <h2 class="state-empty__title">No results found</h2>
        <p class="state-empty__description">
          Try adjusting your search or filter criteria.
        </p>
        <button
          type="button"
          class="media-view__empty-action"
          @click="clearFilters"
        >
          Clear all filters
        </button>
      </div>

      <!-- Empty State -->
      <div v-else-if="mediaStore.media.length === 0" class="state-empty">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="state-empty__icon">
          <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
        </svg>
        <h2 class="state-empty__title">No media files yet</h2>
        <p class="state-empty__description">
          Upload your first image to get started with the media library.
        </p>
        <Button
          type="button"
          variant="primary"
          @click="openUploadForm"
        >
          Upload Media
        </Button>
      </div>

      <!-- Grouped Grid -->
      <template v-else>
        <div v-for="group in mediaGroups" :key="group.label" class="media-view__group">
          <div v-if="group.label" class="media-view__date-separator">
            {{ group.label }}
          </div>
          <div class="media-view__grid">
            <div
              v-for="item in group.items"
              :key="item.id"
              class="media-view__grid-item"
              tabindex="0"
              role="button"
              :aria-label="`View details for ${item.originalFilename}`"
              @click="openDetail(item)"
              @keydown.enter="openDetail(item)"
            >
              <div class="media-view__grid-image-wrapper">
                <img
                  :src="gridImgErrors[item.id] ? item.url : (item.variants?._thumb?.url || item.url)"
                  :alt="item.altText"
                  class="media-view__grid-image"
                  loading="lazy"
                  @error="onGridImgError(item.id)"
                />
                <div class="media-view__tooltip" :aria-describedby="`tooltip-${item.id}`">
                  <span :id="`tooltip-${item.id}`" role="tooltip">
                    <span class="media-view__tooltip-filename">{{ item.originalFilename }}</span>
                    <span class="media-view__tooltip-date">{{ formatDate(item.createdAt) }}</span>
                  </span>
                </div>
              </div>
              <div class="media-view__grid-info">
                <p class="media-view__grid-filename" :title="item.originalFilename">
                  {{ item.originalFilename }}
                </p>
                <div class="media-view__grid-meta">
                  <span class="media-view__grid-date">{{ formatDate(item.createdAt) }}</span>
                  <span class="media-view__grid-size">{{ formatFileSize(item.fileSize) }}</span>
                </div>
                <span v-if="item.uploadedBy" class="media-view__grid-uploader">by {{ item.uploadedBy }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Detail Modal -->
    <MediaDetailModal
      v-if="selectedMedia"
      :media="selectedMedia"
      @close="closeDetail"
      @delete="handleDelete"
    />

    <!-- Toast -->
    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <!-- Duplicate Media Dialog -->
    <DuplicateMediaDialog
      :visible="showDuplicateDialog"
      :existing-media="duplicateMedia"
      :show-use-existing="false"
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
.media-view__header-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

/* Upload Form */
.media-view__upload-form {
  border: 1px solid var(--brand-light-2, #e5e7eb);
  border-radius: 0.5rem;
  margin-bottom: 1.5rem;
  background-color: var(--color-background, #fff);
}

.media-view__upload-form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--brand-light-2, #e5e7eb);
}

.media-view__upload-form-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin: 0;
  color: var(--brand-dark-1);
}

.media-view__upload-form-close {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--brand-dark-2, #6b7280);
  padding: 0.25rem;
  display: flex;
  align-items: center;
  border-radius: 0.25rem;
}

.media-view__upload-form-close:hover {
  background-color: var(--color-bg-muted);
}

.media-view__upload-form-body {
  padding: 1rem;
}

.media-view__upload-inputs {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.media-view__file-input {
  display: none;
}

.media-view__choose-file-btn {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  background-color: var(--color-background, #fff);
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  cursor: pointer;
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.media-view__choose-file-btn:hover {
  background-color: var(--brand-light-1);
}

.media-view__choose-file-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.media-view__alt-input {
  flex: 1;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background, #fff);
}

.media-view__alt-input:focus {
  outline: none;
  border-color: var(--color-info);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.media-view__alt-input:disabled {
  background-color: var(--color-bg-muted);
  cursor: not-allowed;
}

.media-view__upload-error {
  position: relative;
  margin-top: 0.5rem;
  padding: 0.625rem 0.75rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
  font-size: 0.8125rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.media-view__upload-error-close {
  background: none;
  border: none;
  font-size: 1.125rem;
  color: var(--color-error-dark);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

/* Search and Filter Controls */
.media-view__controls {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
}

.media-view__search {
  flex: 1;
  min-width: 200px;
}

.media-view__filter {
  flex-shrink: 0;
}

.media-view__filter-select {
  padding: 0.5rem 2rem 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background, #fff);
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 20 20' fill='%236b7280'%3E%3Cpath fill-rule='evenodd' d='M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z' clip-rule='evenodd'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 0.5rem center;
  background-size: 1.25rem;
}

.media-view__clear-filters {
  padding: 0.375rem 0.75rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  background-color: var(--color-background, #fff);
  font-size: 0.8125rem;
  color: var(--brand-dark-2, #6b7280);
  cursor: pointer;
  white-space: nowrap;
}

.media-view__clear-filters:hover {
  background-color: var(--brand-light-1);
  color: var(--brand-dark-1);
}

/* Content Area */
.media-view__content {
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  min-height: 400px;
}

/* Loading Skeleton */
.media-view__skeleton-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  padding: 1rem;
}

.media-view__skeleton-item {
  border: 1px solid var(--brand-light-2, #e5e7eb);
  border-radius: 0.5rem;
  overflow: hidden;
}

.media-view__skeleton-image {
  aspect-ratio: 16 / 9;
  background: linear-gradient(90deg, var(--color-bg-muted) 25%, var(--brand-light-2) 50%, var(--color-bg-muted) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

.media-view__skeleton-text {
  height: 0.75rem;
  margin: 0.5rem;
  border-radius: 0.25rem;
  background: linear-gradient(90deg, var(--color-bg-muted) 25%, var(--brand-light-2) 50%, var(--color-bg-muted) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

/* Empty States */
.media-view__empty-action {
  padding: 0.5rem 1rem;
  border: 1px solid var(--brand-light-2, #d1d5db);
  border-radius: 0.375rem;
  background-color: var(--color-background, #fff);
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  cursor: pointer;
}

/* Date Separator */
.media-view__date-separator {
  padding: 0.5rem 1rem 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--brand-dark-2, #6b7280);
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.media-view__group + .media-view__group .media-view__date-separator {
  padding-top: 1rem;
  border-top: 1px solid var(--brand-light-2, #f3f4f6);
}

/* Grid */
.media-view__grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  padding: 1rem;
}

.media-view__grid-item {
  border: 1px solid var(--brand-light-2, #e5e7eb);
  border-radius: 0.5rem;
  overflow: hidden;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  background-color: var(--color-background, #fff);
}

.media-view__grid-item:hover {
  border-color: var(--color-border-strong);
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.media-view__grid-item:focus {
  outline: 2px solid var(--color-info);
  outline-offset: 2px;
}

.media-view__grid-image-wrapper {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background-color: var(--color-bg-muted);
}

.media-view__grid-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* Tooltip */
.media-view__tooltip {
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  margin-bottom: 0.5rem;
  padding: 0.5rem;
  background-color: var(--color-bg-inverse);
  border-radius: 0.375rem;
  color: var(--brand-light-1);
  font-size: 0.75rem;
  line-height: 1.4;
  white-space: nowrap;
  max-width: 12rem;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
  z-index: 10;
}

.media-view__tooltip span {
  display: block;
}

.media-view__tooltip::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 0.375rem solid transparent;
  border-top-color: var(--color-bg-inverse);
}

.media-view__grid-item:hover .media-view__tooltip {
  opacity: 1;
  transition-delay: 0.3s;
}

.media-view__grid-info {
  padding: 0.5rem;
}

.media-view__grid-filename {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-view__grid-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 0.25rem;
}

.media-view__grid-date {
  font-size: 0.75rem;
  color: var(--brand-dark-2, #9ca3af);
}

.media-view__grid-size {
  font-size: 0.75rem;
  color: var(--brand-dark-2, #9ca3af);
}

.media-view__grid-uploader {
  display: block;
  font-size: 0.6875rem;
  color: var(--brand-dark-2, #9ca3af);
  margin-top: 0.125rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (min-width: 768px) {
  .media-view__grid {
    grid-template-columns: repeat(4, 1fr);
  }

  .media-view__skeleton-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (max-width: 640px) {
  .media-view__upload-inputs {
    flex-direction: column;
    align-items: stretch;
  }

  .media-view__grid-image-wrapper {
    aspect-ratio: 1 / 1;
  }

  .media-view__tooltip {
    display: none;
  }
}
</style>
