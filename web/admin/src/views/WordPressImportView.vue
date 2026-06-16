<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import api, { ApiError } from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'

interface ImportResult {
  imported: number
  skipped: number
  errors?: string[]
}

const router = useRouter()
const selectedFile = ref<File | null>(null)
const fileInput = ref<HTMLInputElement>()
const isImporting = ref(false)
const result = ref<ImportResult | null>(null)
const importError = ref('')

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

const canImport = computed(() => selectedFile.value !== null && !isImporting.value)
const dropzoneClasses = computed(() => ({
  'dropzone--active': selectedFile.value !== null,
  'dropzone--disabled': isImporting.value,
}))

function goBack() {
  router.push('/import')
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) {
    selectedFile.value = null
    return
  }
  if (!file.name.toLowerCase().endsWith('.xml')) {
    displayToast('Please select a WordPress export file (.xml)', 'error')
    target.value = ''
    selectedFile.value = null
    return
  }
  selectedFile.value = file
  result.value = null
  importError.value = ''
}

function handleDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0]
  if (!file) {
    return
  }
  if (!file.name.toLowerCase().endsWith('.xml')) {
    displayToast('Please select a WordPress export file (.xml)', 'error')
    return
  }
  selectedFile.value = file
  result.value = null
  importError.value = ''
}

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

async function handleImport() {
  if (!selectedFile.value) {
    return
  }

  isImporting.value = true
  importError.value = ''
  result.value = null

  try {
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    const response = await api.postWithTimeout<{ data: ImportResult }>(
      '/api/admin/wordpress/import',
      formData,
      5 * 60 * 1000,
    )
    result.value = response.data.data
    if (result.value.skipped > 0) {
      displayToast(
        `Imported ${result.value.imported}, skipped ${result.value.skipped}`,
        'error',
      )
    } else {
      displayToast(`Successfully imported ${result.value.imported} item${result.value.imported === 1 ? '' : 's'}`, 'success')
    }
  } catch (err) {
    if (err instanceof ApiError) {
      importError.value = err.message
    } else {
      importError.value = 'Failed to import WordPress file. Please try again.'
    }
    displayToast(importError.value, 'error')
  } finally {
    isImporting.value = false
  }
}

function resetForm() {
  selectedFile.value = null
  result.value = null
  importError.value = ''
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}
</script>

<template>
  <div class="wordpress-import-form">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Import from WordPress</h1>
        <p class="page-subtitle">
          Import posts and pages from a WordPress eXtended RSS (WXR) export file.
        </p>
      </div>
    </header>

    <div class="card">
      <input
        ref="fileInput"
        type="file"
        accept=".xml,application/xml,text/xml"
        class="wordpress-import-form__file-input"
        :disabled="isImporting"
        @change="handleFileSelect"
      />

      <div
        class="dropzone"
        :class="dropzoneClasses"
        @click="!isImporting && fileInput?.click()"
        @dragover.prevent
        @drop.prevent="handleDrop"
      >
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="dropzone__icon">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
        </svg>
        <p v-if="selectedFile" class="dropzone__text">
          {{ selectedFile.name }}
        </p>
        <template v-else>
          <p class="dropzone__text">
            Click to choose a file or drag and drop
          </p>
          <p class="dropzone__hint">WordPress export (.xml)</p>
        </template>
      </div>

      <div class="wordpress-import-form__actions">
        <Button
          type="button"
          variant="primary"
          :is-loading="isImporting"
          :disabled="!canImport"
          @click="handleImport"
        >
          {{ isImporting ? 'Importing...' : 'Import' }}
        </Button>
        <Button
          v-if="result"
          type="button"
          variant="secondary"
          :disabled="isImporting"
          @click="resetForm"
        >
          Import Another
        </Button>
      </div>

      <div v-if="isImporting" class="alert alert-info wordpress-import-form__alert">
        Importing content. This may take a few minutes for large files or many images.
      </div>

      <div v-if="importError" class="alert alert-error wordpress-import-form__alert">
        {{ importError }}
      </div>
    </div>

    <div v-if="result" class="card wordpress-import-form__results">
      <h2 class="card-title">Import Summary</h2>
      <div class="stats">
        <div class="stat stat--success">
          <span class="stat-value">{{ result.imported }}</span>
          <span class="stat-label">Imported</span>
        </div>
        <div class="stat stat--warning">
          <span class="stat-value">{{ result.skipped }}</span>
          <span class="stat-label">Skipped</span>
        </div>
      </div>
      <div v-if="result.errors && result.errors.length > 0" class="wordpress-import-form__errors">
        <h3 class="card-title">Issues</h3>
        <ul class="wordpress-import-form__error-list">
          <li v-for="(err, index) in result.errors" :key="index" class="alert alert-error wordpress-import-form__error-item">
            {{ err }}
          </li>
        </ul>
      </div>
    </div>

    <div class="wordpress-import-form__back">
      <button type="button" class="wordpress-import-form__back-link" @click="goBack">
        ← Back to Import
      </button>
    </div>

    <Toast
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @update:visible="toastVisible = $event"
    />
  </div>
</template>

<style scoped>
.wordpress-import-form {
  max-width: 640px;
}

.wordpress-import-form__file-input {
  display: none;
}

.wordpress-import-form__actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.5rem;
}

.wordpress-import-form__alert {
  margin-top: 1rem;
}

.wordpress-import-form__results {
  margin-top: 1.5rem;
}

.wordpress-import-form__errors {
  margin-top: 1.25rem;
}

.wordpress-import-form__errors .card-title {
  font-size: 0.95rem;
  margin-bottom: 0.5rem;
}

.wordpress-import-form__error-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 200px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.wordpress-import-form__error-item {
  word-break: break-word;
}

.wordpress-import-form__back {
  margin-top: 1.5rem;
}

.wordpress-import-form__back-link {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--brand-dark-2);
  font-size: 0.9rem;
  padding: 0;
}

.wordpress-import-form__back-link:hover {
  color: var(--brand-primary);
}
</style>
