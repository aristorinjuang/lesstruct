<script setup lang="ts">
import { onMounted, ref } from 'vue'
import api from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import ThemeSnapshot from '@/components/molecules/ThemeSnapshot.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useThemegenStore, firstProblemFromError } from '@/stores/domain/themegen'

const store = useThemegenStore()
const description = ref('')
const style = ref('')
const templateOverrides = ref(false)
const themeAvailable = ref(false)

const stylePresets = ['', 'dark', 'light', 'minimal', 'playful', 'editorial', 'brutalist']

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

function selectPreset(preset: string) {
  style.value = style.value === preset ? '' : preset
}

onMounted(async () => {
  try {
    const response = await api.get<{ features: { themeGeneration?: boolean } }>('/api/health')
    themeAvailable.value = response.data.features?.themeGeneration === true
  } catch {
    themeAvailable.value = false
  }
  await store.fetchPreviewStatus()
})

async function handleGenerate() {
  if (!description.value.trim()) {
    displayToast('Description is required', 'error')
    return
  }
  try {
    await store.generate(description.value, style.value, templateOverrides.value)
    displayToast(`Generated theme ${store.theme?.name}`)
  } catch {
    const reason = firstProblemFromError(store.error)
    displayToast(
      reason ? `Generation failed: ${reason}` : store.error?.message || 'Generation failed. Please try again.',
      'error',
    )
  }
}

async function handleDownload() {
  if (!store.theme) return
  try {
    await store.download(store.theme.name)
    displayToast(`Downloaded ${store.theme.name}.zip`)
  } catch {
    displayToast('Download failed. Please try again.', 'error')
  }
}

async function handleActivate() {
  if (!store.theme) return
  try {
    const requiresRestart = await store.activate(store.theme.name)
    displayToast(requiresRestart ? 'Theme activated. Restart to apply.' : 'Theme applied.')
  } catch {
    displayToast('Activation failed. Please try again.', 'error')
  }
}

async function handlePreview() {
  if (!store.theme) return
  try {
    await store.preview(store.theme.name)
    window.open('/', '_blank', 'noopener')
    displayToast('Preview opened in a new tab — valid 15 minutes.')
  } catch {
    displayToast('Preview failed. Please try again.', 'error')
  }
}

async function handleStopPreview() {
  try {
    await store.stopPreview()
    displayToast('Preview stopped.')
  } catch {
    displayToast('Could not stop preview.', 'error')
  }
}
</script>

<template>
  <div class="themegen-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Theme Generator</h1>
        <p class="page-subtitle">Describe a theme in plain language. The generator returns a validated theme; page templates are opt-in.</p>
      </div>
    </header>

    <div v-if="!themeAvailable" class="themegen-notice">
      Theme generation is not configured. Set AI_THEMGEN_API_KEY or AI_TEXT_GENERATION_API_KEY.
    </div>

    <div v-else class="themegen-grid">
      <div class="themegen-card">
        <label class="themegen-label" for="themegen-description">Brief</label>
        <textarea
          id="themegen-description"
          v-model="description"
          class="themegen-textarea"
          rows="6"
          placeholder="A dark editorial theme with serif headings and generous whitespace…"
        />
        <div class="themegen-chips">
          <Button
            v-for="preset in stylePresets"
            :key="preset || 'auto'"
            type="button"
            :variant="style === preset ? 'primary' : 'neutral'"
            :aria-pressed="style === preset ? 'true' : 'false'"
            size="small"
            @click="selectPreset(preset)"
          >
            {{ preset || 'auto' }}
          </Button>
        </div>
        <label class="themegen-checkbox" for="themegen-template-overrides">
          <input id="themegen-template-overrides" v-model="templateOverrides" type="checkbox" />
          <span>
            Generate page templates
            <small>Off ships a stylesheet-only theme. On also generates templates, parse-tested and smoke-rendered before install.</small>
          </span>
        </label>
        <div class="themegen-actions">
          <Button type="button" variant="primary" :is-loading="store.isLoading" @click="handleGenerate">
            {{ store.isLoading ? 'Generating…' : 'Generate' }}
          </Button>
        </div>
      </div>

      <div v-if="store.theme" class="themegen-card">
        <ThemeSnapshot :name="store.theme.name" />
        <h2 class="themegen-title">{{ store.theme.name }}</h2>
        <p class="themegen-rationale">{{ store.theme.rationale }}</p>
        <div class="themegen-actions">
          <Button type="button" variant="neutral" @click="handleDownload">Download</Button>
          <Button type="button" variant="neutral" @click="handlePreview">Preview</Button>
          <Button type="button" variant="primary" @click="handleActivate">Activate</Button>
          <Button v-if="store.previewTheme" type="button" variant="subtle" @click="handleStopPreview">Stop preview ({{ store.previewTheme }})</Button>
        </div>
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
.themegen-view {
  max-width: 100%;
}
.themegen-notice {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
}
.themegen-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1.25rem;
}
.themegen-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.themegen-label {
  font-weight: 600;
}
.themegen-textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}
.themegen-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.themegen-checkbox {
  display: flex;
  gap: 0.5rem;
  align-items: flex-start;
}
.themegen-checkbox small {
  display: block;
  color: var(--color-text-muted);
}
.themegen-actions {
  display: flex;
  gap: 0.5rem;
}
.themegen-title {
  margin: 0;
}
.themegen-rationale {
  margin: 0;
}
</style>
