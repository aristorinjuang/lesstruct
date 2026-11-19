<script setup lang="ts">
import { onMounted, ref } from 'vue'
import api from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'
import { usePlugingenStore, firstProblemFromError } from '@/stores/domain/plugingen'

const store = usePlugingenStore()
const description = ref('')
const hooksInput = ref('before_save')
const capabilitiesInput = ref('log_info')
const pluginAvailable = ref(false)
const pluginBuildAvailable = ref(false)

const knownHooks = [
  'before_save',
  'after_create',
  'after_publish',
  'before_delete',
  'after_unpublish',
  'on_plugin_loaded',
]
const knownCapabilities = ['http_get', 'http_post', 'db_query', 'db_exec', 'log_info', 'log_error']

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

function toggleHook(hook: string) {
  const current = new Set(splitList(hooksInput.value))
  if (current.has(hook)) {
    current.delete(hook)
  } else {
    current.add(hook)
  }
  hooksInput.value = [...current].join(', ')
}

function toggleCapability(capability: string) {
  const current = new Set(splitList(capabilitiesInput.value))
  if (current.has(capability)) {
    current.delete(capability)
  } else {
    current.add(capability)
  }
  capabilitiesInput.value = [...current].join(', ')
}

onMounted(async () => {
  try {
    const response = await api.get<{
      features: { pluginGeneration?: boolean; pluginBuild?: boolean }
    }>('/api/health')
    pluginAvailable.value = response.data.features?.pluginGeneration === true
    pluginBuildAvailable.value = response.data.features?.pluginBuild === true
  } catch {
    pluginAvailable.value = false
    pluginBuildAvailable.value = false
  }
})

async function handleGenerate() {
  if (!description.value.trim()) {
    displayToast('Description is required', 'error')
    return
  }
  const hooks = splitList(hooksInput.value)
  if (hooks.length === 0) {
    displayToast('At least one hook is required', 'error')
    return
  }
  try {
    await store.generate(description.value, hooks, splitList(capabilitiesInput.value))
    displayToast(`Generated plugin ${store.project?.name}`)
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

async function handleBuild() {
  if (!store.project) return
  try {
    const requiresRestart = await store.build(store.project.name)
    displayToast(
      requiresRestart
        ? 'Plugin installed. Restart to apply.'
        : 'Plugin installed and hot-reloaded.',
    )
  } catch {
    displayToast('Build failed. Please try again.', 'error')
  }
}
</script>

<template>
  <div class="plugingen-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Plugin Generator</h1>
        <p class="page-subtitle">
          Describe a plugin in plain language. The generator returns a validated WASM plugin
          project.
        </p>
      </div>
    </header>

    <div v-if="!pluginAvailable" class="plugingen-notice">
      Plugin generation is not configured. Set AI_PLUGGEN_API_KEY or AI_TEXT_GENERATION_API_KEY.
    </div>

    <div v-else class="plugingen-grid">
      <div class="plugingen-card">
        <label class="plugingen-label" for="plugingen-description">Brief</label>
        <textarea
          id="plugingen-description"
          v-model="description"
          class="plugingen-textarea"
          rows="6"
          placeholder="Uppercase post titles on save…"
        />
        <span class="plugingen-label">Hooks</span>
        <div class="plugingen-chips">
          <Button
            v-for="hook in knownHooks"
            :key="hook"
            type="button"
            :variant="splitList(hooksInput).includes(hook) ? 'primary' : 'neutral'"
            :aria-pressed="splitList(hooksInput).includes(hook) ? 'true' : 'false'"
            size="small"
            @click="toggleHook(hook)"
          >
            {{ hook }}
          </Button>
        </div>
        <span class="plugingen-label">Capabilities</span>
        <div class="plugingen-chips">
          <Button
            v-for="capability in knownCapabilities"
            :key="capability"
            type="button"
            :variant="splitList(capabilitiesInput).includes(capability) ? 'primary' : 'neutral'"
            :aria-pressed="splitList(capabilitiesInput).includes(capability) ? 'true' : 'false'"
            size="small"
            @click="toggleCapability(capability)"
          >
            {{ capability }}
          </Button>
        </div>
        <div class="plugingen-actions">
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

      <div v-if="store.project" class="plugingen-card">
        <h2 class="plugingen-title">{{ store.project.name }}</h2>
        <p class="plugingen-rationale">{{ store.project.rationale }}</p>
        <p class="plugingen-meta">
          {{ store.project.language }} · {{ store.project.hooks.join(', ') }}
        </p>
        <ul class="plugingen-files">
          <li v-for="name in Object.keys(store.project.files)" :key="name">{{ name }}</li>
        </ul>
        <div class="plugingen-actions">
          <Button type="button" variant="neutral" @click="handleDownload">Download</Button>
          <Button
            v-if="pluginBuildAvailable"
            type="button"
            variant="primary"
            :is-loading="store.isLoading"
            @click="handleBuild"
          >
            Build &amp; install
          </Button>
          <Button type="button" variant="danger" @click="handleDelete">Delete</Button>
        </div>
        <p class="plugingen-hint">
          Compile with TinyGo (<code
            >tinygo build -o {{ store.project.name }}.wasm -target=wasi main.go</code
          >) and drop the .wasm into <code>plugins/</code> — or build on the server when enabled.
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
.plugingen-view {
  max-width: 100%;
}
.plugingen-notice {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
}
.plugingen-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1.25rem;
}
.plugingen-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.plugingen-label {
  font-weight: 600;
}
.plugingen-textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}
.plugingen-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.plugingen-actions {
  display: flex;
  gap: 0.5rem;
}
.plugingen-title {
  margin: 0;
}
.plugingen-rationale {
  margin: 0;
}
.plugingen-meta {
  margin: 0;
}
.plugingen-files {
  margin: 0;
  padding-left: 1.25rem;
}
.plugingen-hint {
  margin: 0;
}
</style>
