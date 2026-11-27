<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import ThemeSnapshot from '@/components/molecules/ThemeSnapshot.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useConfig } from '@/composables/useConfig'
import { useThemegenStore } from '@/stores/domain/themegen'

const store = useThemegenStore()
// The AI key gates generation, not management: the Themes page lists,
// downloads, activates, and deletes without one, so the preview action —
// whose endpoint stays key-gated — hides when generation is off.
const { themeGeneration } = useConfig()

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

onMounted(async () => {
  try {
    await store.fetchThemes()
  } catch {
    displayToast('Failed to load themes', 'error')
  }
  // The preview status probe stays key-gated server-side; skip it when
  // generation is off instead of logging a 404 per mount.
  if (themeGeneration.value) {
    await store.fetchPreviewStatus()
  }
})

async function handleDownload(name: string) {
  try {
    await store.download(name)
    displayToast(`Downloaded ${name}.zip`)
  } catch {
    displayToast('Download failed', 'error')
  }
}

async function handleActivate(name: string) {
  try {
    const requiresRestart = await store.activate(name)
    displayToast(requiresRestart ? 'Theme activated. Restart to apply.' : 'Theme applied.')
  } catch {
    displayToast('Activation failed', 'error')
  }
}

async function handleDeactivate() {
  try {
    const requiresRestart = await store.deactivate()
    displayToast(requiresRestart ? 'Theme deactivated. Restart to apply.' : 'Embedded theme applied.')
  } catch {
    displayToast('Deactivation failed', 'error')
  }
}

async function handleDelete(name: string) {
  try {
    await store.remove(name)
    displayToast(`Deleted ${name}`)
  } catch {
    displayToast('Delete failed (active themes cannot be deleted)', 'error')
  }
}

async function handlePreview(name: string) {
  try {
    await store.preview(name)
    window.open('/', '_blank', 'noopener')
    displayToast('Preview opened in a new tab — valid 15 minutes.')
  } catch {
    displayToast('Preview failed', 'error')
  }
}

async function handleStopPreview() {
  try {
    await store.stopPreview()
    displayToast('Preview stopped.')
  } catch {
    displayToast('Could not stop preview', 'error')
  }
}
</script>

<template>
  <div class="themes-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Themes</h1>
        <p class="page-subtitle">Installed themes. Activation applies immediately.</p>
      </div>
    </header>

    <div v-if="store.previewTheme" class="themes-actions">
      <Button type="button" variant="subtle" size="small" @click="handleStopPreview">Stop preview ({{ store.previewTheme }})</Button>
    </div>

    <div class="themes-grid">
      <div v-for="theme in store.themes" :key="theme.name" class="themes-card">
        <ThemeSnapshot :name="theme.name" />
        <h2 class="themes-title">{{ theme.name }}</h2>
        <p class="themes-meta">{{ theme.edition }} · {{ theme.source }} · {{ theme.active ? 'active' : 'inactive' }}</p>
        <p v-if="store.previewTheme === theme.name" class="themes-preview-badge">Previewing in this browser</p>
        <div class="themes-actions">
          <Button type="button" variant="neutral" size="small" @click="handleDownload(theme.name)">Download</Button>
          <Button v-if="themeGeneration" type="button" variant="neutral" size="small" @click="handlePreview(theme.name)">Preview</Button>
          <Button v-if="!theme.active" type="button" variant="primary" size="small" @click="handleActivate(theme.name)">Activate</Button>
          <Button v-if="theme.active" type="button" variant="neutral" size="small" @click="handleDeactivate">Deactivate</Button>
          <Button v-if="!theme.active" type="button" variant="danger" size="small" @click="handleDelete(theme.name)">Delete</Button>
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
.themes-view {
  max-width: 100%;
}
.themes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
}
.themes-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.themes-title {
  margin: 0;
}
.themes-meta {
  margin: 0;
}
.themes-preview-badge {
  margin: 0;
  font-size: 0.85em;
  color: var(--color-text-muted);
}
.themes-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: .5rem;
}
</style>
