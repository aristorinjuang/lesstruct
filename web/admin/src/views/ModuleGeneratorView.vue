<script setup lang="ts">
import { onMounted, ref } from 'vue'
import api from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useModgenStore, firstProblemFromError } from '@/stores/domain/modgen'

const store = useModgenStore()
const description = ref('')
const surfacesInput = ref('routes')
const moduleAvailable = ref(false)
const applyAvailable = ref(false)

const knownSurfaces = [
  'routes',
  'public_routes',
  'migrations',
  'posttypes',
  'roles',
  'themes',
  'admin',
]

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

function splitList(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
}

function toggleSurface(surface: string) {
  const current = new Set(splitList(surfacesInput.value))
  if (current.has(surface)) {
    current.delete(surface)
  } else {
    current.add(surface)
  }
  surfacesInput.value = [...current].join(', ')
}

onMounted(async () => {
  try {
    const response = await api.get<{
      features: { moduleGeneration?: boolean; moduleApply?: boolean }
    }>('/api/health')
    moduleAvailable.value = response.data.features?.moduleGeneration === true
    applyAvailable.value = response.data.features?.moduleApply === true
  } catch {
    moduleAvailable.value = false
    applyAvailable.value = false
  }
})

async function handleGenerate() {
  if (!description.value.trim()) {
    displayToast('Description is required', 'error')
    return
  }
  try {
    await store.generate(description.value, splitList(surfacesInput.value))
    displayToast(`Generated module ${store.project?.name}`)
  } catch {
    const reason = firstProblemFromError(store.error)
    displayToast(
      reason
        ? `Generation failed: ${reason}`
        : store.error?.message || 'Generation failed. Please try again.',
      'error',
    )
  }
}

async function handleDownload() {
  if (!store.project) return
  try {
    await store.download(store.project.name)
    displayToast(`Downloaded ${store.project.name}.zip`)
  } catch {
    displayToast('Download failed. Please try again.', 'error')
  }
}

async function handleDelete() {
  if (!store.project) return
  try {
    await store.remove(store.project.name)
    displayToast(`Deleted ${store.project.name}`)
    store.project = null
  } catch {
    displayToast('Delete failed. Please try again.', 'error')
  }
}

async function handleApply() {
  if (!store.project) return
  try {
    const { apply } = await store.apply(store.project.name)
    if (apply.verified) {
      displayToast(`Applied ${store.project.name} — rebuild ./cmd/lesstruct-custom and redeploy.`)
    } else {
      const failed = apply.steps
        .filter((s) => !s.passed && !s.skipped)
        .map((s) => s.name)
        .join(', ')
      displayToast(
        failed ? `Applied with failing checks: ${failed}` : 'Applied with failing checks.',
        'error',
      )
    }
  } catch {
    displayToast('Apply failed. Please try again.', 'error')
  }
}
</script>

<template>
  <div class="modgen-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Module Generator</h1>
        <p class="page-subtitle">
          Describe a module in plain language. The generator returns a validated Tier 1 Go module
          project.
        </p>
      </div>
    </header>

    <div v-if="!moduleAvailable" class="modgen-notice">
      Module generation is not configured. Set AI_MODGEN_API_KEY or AI_TEXT_GENERATION_API_KEY.
    </div>

    <div v-else class="modgen-grid">
      <div class="modgen-card">
        <label class="modgen-label" for="modgen-description">Brief</label>
        <textarea
          id="modgen-description"
          v-model="description"
          class="modgen-textarea"
          rows="6"
          placeholder="Forum threads with moderation…"
        />
        <span class="modgen-label">Surfaces</span>
        <div class="modgen-chips">
          <Button
            v-for="surface in knownSurfaces"
            :key="surface"
            type="button"
            :variant="splitList(surfacesInput).includes(surface) ? 'primary' : 'neutral'"
            :aria-pressed="splitList(surfacesInput).includes(surface) ? 'true' : 'false'"
            size="small"
            @click="toggleSurface(surface)"
          >
            {{ surface }}
          </Button>
        </div>
        <div class="modgen-actions">
          <Button
            type="button"
            variant="primary"
            :is-loading="store.isLoading"
            @click="handleGenerate"
          >
            {{ store.isLoading ? 'Generating…' : 'Generate' }}
          </Button>
        </div>
      </div>

      <div v-if="store.project" class="modgen-card">
        <h2 class="modgen-title">{{ store.project.name }}</h2>
        <p class="modgen-rationale">{{ store.project.rationale }}</p>
        <p class="modgen-meta">{{ store.project.surfaces.join(', ') }}</p>
        <ul class="modgen-files">
          <li v-for="name in Object.keys(store.project.files)" :key="name">{{ name }}</li>
        </ul>
        <div class="modgen-actions">
          <Button type="button" variant="neutral" @click="handleDownload">Download</Button>
          <Button
            v-if="applyAvailable"
            type="button"
            variant="primary"
            :is-loading="store.isLoading"
            @click="handleApply"
          >
            Apply to checkout
          </Button>
          <Button type="button" variant="danger" @click="handleDelete">Delete</Button>
        </div>
        <p class="modgen-hint">
          Tier 1 modules link at compile time: unzip into <code>modules/{{ store.project.name }}/</code>,
          wire a custom edition, and rebuild — see INSTALL.md in the archive.
        </p>
      </div>
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
.modgen-view {
  max-width: 100%;
}
.modgen-notice {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
}
.modgen-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1.25rem;
}
.modgen-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.modgen-label {
  font-weight: 600;
}
.modgen-textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}
.modgen-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.modgen-actions {
  display: flex;
  gap: 0.5rem;
}
.modgen-title {
  margin: 0;
}
.modgen-rationale {
  margin: 0;
}
.modgen-meta {
  margin: 0;
}
.modgen-files {
  margin: 0;
  padding-left: 1.25rem;
}
.modgen-hint {
  margin: 0;
}
</style>
