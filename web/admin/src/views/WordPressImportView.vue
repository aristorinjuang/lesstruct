<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import api, { ApiError } from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'

interface ImportJob {
  state: string
  imported: number
  skipped: number
  usersImported: number
  total: number
  errors?: string[]
  startedAt?: string
  finishedAt?: string
}

const STORAGE_KEY = 'wp_import_job_id'

const router = useRouter()
const selectedFile = ref<File | null>(null)
const fileInput = ref<HTMLInputElement>()
const isUploading = ref(false)
const isImporting = ref(false)
const job = ref<ImportJob | null>(null)
const importError = ref('')
const skipMedia = ref(false)

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

let jobId: string | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null

const canImport = computed(() => selectedFile.value !== null && !isUploading.value && !isImporting.value)
const dropzoneClasses = computed(() => ({
  'dropzone--active': selectedFile.value !== null,
  'dropzone--disabled': isUploading.value || isImporting.value,
}))

const progressLabel = computed(() => {
  if (isUploading.value) return 'Uploading file...'
  if (isImporting.value && job.value) {
    return `Importing... ${job.value.imported.toLocaleString()} / ${job.value.total.toLocaleString()} items`
  }
  if (isImporting.value) return 'Starting import...'
  return ''
})

onMounted(() => {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored) {
    jobId = stored
    isImporting.value = true
    startPolling()
    // Kick off the first poll immediately so progress appears without a 3-second delay.
    pollStatus()
  }
})

onUnmounted(() => {
  stopPolling()
})

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(pollStatus, 3000)
}

async function pollStatus() {
  if (!jobId) return
  try {
    const response = await api.get<{ data: { job: ImportJob } }>(
      `/api/admin/wordpress/import/status/${jobId}`,
      { timeout: 10000 },
    )
    const jobData = response.data.data.job
    job.value = jobData

    if (jobData.state === 'done') {
      stopPolling()
      isImporting.value = false
      localStorage.removeItem(STORAGE_KEY)
      displayToast(
        `Successfully imported ${jobData.imported} item${jobData.imported === 1 ? '' : 's'}` +
          (jobData.skipped > 0 ? `, skipped ${jobData.skipped}` : ''),
        'success',
      )
    } else if (jobData.state === 'failed') {
      stopPolling()
      isImporting.value = false
      localStorage.removeItem(STORAGE_KEY)
      displayToast('Import failed. See details below for more information.', 'error')
    }
  } catch (err) {
    // If the job is no longer on the server (server restart), give up.
    if (err instanceof ApiError && err.statusCode === 404) {
      stopPolling()
      isImporting.value = false
      localStorage.removeItem(STORAGE_KEY)
      job.value = null
      importError.value = 'Import job no longer exists on the server (server may have restarted).'
      return
    }
    // Other poll errors are transient — the next tick will retry.
  }
}

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
  job.value = null
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
  job.value = null
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

  isUploading.value = true
  isImporting.value = false
  importError.value = ''
  job.value = null
  jobId = null

  try {
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    if (skipMedia.value) {
      formData.append('skipMedia', 'true')
    }
    const response = await api.postWithTimeout<{ data: { jobId: string; state: string } }>(
      '/api/admin/wordpress/import',
      formData,
      30 * 60 * 1000,
    )
    jobId = response.data.data.jobId
    localStorage.setItem(STORAGE_KEY, jobId)
    isUploading.value = false
    isImporting.value = true
    startPolling()
  } catch (err) {
    isUploading.value = false
    if (err instanceof ApiError) {
      importError.value = err.message
    } else {
      importError.value = 'Failed to start WordPress import. Please try again.'
    }
    displayToast(importError.value, 'error')
  }
}

function resetForm() {
  stopPolling()
  localStorage.removeItem(STORAGE_KEY)
  selectedFile.value = null
  job.value = null
  importError.value = ''
  jobId = null
  isUploading.value = false
  isImporting.value = false
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
        :disabled="isUploading || isImporting"
        @change="handleFileSelect"
      />

      <div
        class="dropzone"
        :class="dropzoneClasses"
        @click="!isUploading && !isImporting && fileInput?.click()"
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

      <label class="wordpress-import-form__checkbox">
        <input type="checkbox" v-model="skipMedia" :disabled="isUploading || isImporting" />
        <span>Skip media — import text only; images stay linked to the original WordPress URLs</span>
      </label>

      <div class="wordpress-import-form__actions">
        <Button
          type="button"
          variant="primary"
          :is-loading="isUploading || isImporting"
          :disabled="!canImport"
          @click="handleImport"
        >
          {{ isUploading || isImporting ? 'Importing...' : 'Import' }}
        </Button>
        <Button
          v-if="job && !isImporting"
          type="button"
          variant="secondary"
          @click="resetForm"
        >
          Import Another
        </Button>
      </div>

      <div v-if="isUploading || isImporting" class="alert alert-info wordpress-import-form__alert">
        {{ progressLabel }}
      </div>

      <div v-if="importError" class="alert alert-error wordpress-import-form__alert">
        {{ importError }}
      </div>
    </div>

    <div v-if="job" class="card wordpress-import-form__results">
      <h2 class="card-title">Import Summary</h2>
      <div v-if="job.state === 'running'" class="progress-bar">
        <div
          class="progress-bar__fill"
          :style="{ width: (job.total > 0 ? (job.imported / job.total) * 100 : 0) + '%' }"
        />
      </div>
      <div class="stats">
        <div class="stat" :class="job.state === 'failed' ? 'stat--warning' : 'stat--success'">
          <span class="stat-value">{{ job.imported.toLocaleString() }}</span>
          <span class="stat-label">Imported</span>
        </div>
        <div class="stat stat--success">
          <span class="stat-value">{{ job.usersImported }}</span>
          <span class="stat-label">Users Created</span>
        </div>
        <div class="stat stat--warning">
          <span class="stat-value">{{ job.skipped }}</span>
          <span class="stat-label">Skipped</span>
        </div>
      </div>
      <div v-if="job.errors && job.errors.length > 0" class="wordpress-import-form__errors">
        <h3 class="card-title">Issues</h3>
        <ul class="wordpress-import-form__error-list">
          <li v-for="(err, index) in job.errors" :key="index" class="alert alert-error wordpress-import-form__error-item">
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

.wordpress-import-form__checkbox {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin-top: 1rem;
  font-size: 0.875rem;
  color: var(--color-gray-700);
  cursor: pointer;
}

.wordpress-import-form__checkbox input[type="checkbox"] {
  margin-top: 0.15rem;
  flex-shrink: 0;
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

.progress-bar {
  height: 6px;
  background: var(--color-gray-200);
  border-radius: 3px;
  margin-bottom: 1rem;
  overflow: hidden;
}

.progress-bar__fill {
  height: 100%;
  background: var(--brand-primary, #2563eb);
  border-radius: 3px;
  transition: width 0.5s ease;
}
</style>