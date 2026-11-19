<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import api from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'
import { usePlugingenStore } from '@/stores/domain/plugingen'

const store = usePlugingenStore()
const pluginBuildAvailable = ref(false)

const installedNames = computed(() => new Set(store.plugins.map((p) => p.name)))

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
    const response = await api.get<{ features: { pluginBuild?: boolean } }>('/api/health')
    pluginBuildAvailable.value = response.data.features?.pluginBuild === true
  } catch {
    pluginBuildAvailable.value = false
  }
  try {
    await store.fetchProjects()
  } catch {
    displayToast('Failed to load plugin projects', 'error')
  }
  try {
    await store.fetchStatus()
  } catch {
    displayToast('Failed to load plugin status', 'error')
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

async function handleDelete(name: string) {
  try {
    await store.remove(name)
    displayToast(`Deleted ${name}`)
  } catch {
    displayToast('Delete failed', 'error')
  }
}

async function handleBuild(name: string) {
  try {
    const requiresRestart = await store.build(name)
    displayToast(
      requiresRestart
        ? 'Plugin installed. Restart to apply.'
        : 'Plugin installed and hot-reloaded.',
    )
    await store.fetchStatus()
  } catch {
    displayToast('Build failed', 'error')
  }
}
</script>

<template>
  <div class="plugins-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Plugins</h1>
        <p class="page-subtitle">Generated projects and loaded plugins.</p>
      </div>
    </header>

    <h2 class="plugins-section">Projects</h2>
    <div class="plugins-grid">
      <div v-for="project in store.projects" :key="project.name" class="plugins-card">
        <h3 class="plugins-title">{{ project.name }}</h3>
        <p class="plugins-meta">
          {{ project.language }} · {{ project.hooks.join(', ') }} · {{ project.source }}
        </p>
        <p v-if="installedNames.has(project.name)" class="plugins-badge">installed</p>
        <div class="plugins-actions">
          <Button type="button" variant="neutral" size="small" @click="handleDownload(project.name)"
            >Download</Button
          >
          <Button
            v-if="pluginBuildAvailable"
            type="button"
            variant="primary"
            size="small"
            @click="handleBuild(project.name)"
            >Build &amp; install</Button
          >
          <Button type="button" variant="danger" size="small" @click="handleDelete(project.name)"
            >Delete</Button
          >
        </div>
      </div>
    </div>

    <h2 class="plugins-section">Loaded plugins</h2>
    <div class="plugins-grid">
      <div v-for="plugin in store.plugins" :key="plugin.name" class="plugins-card">
        <h3 class="plugins-title">{{ plugin.name }}</h3>
        <p class="plugins-meta">{{ plugin.status }} · {{ plugin.hooks.join(', ') }}</p>
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
.plugins-view {
  max-width: 100%;
}
.plugins-section {
  margin: 1rem 0 0.5rem;
}
.plugins-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
}
.plugins-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.plugins-title {
  margin: 0;
}
.plugins-meta {
  margin: 0;
}
.plugins-badge {
  margin: 0;
  font-size: 0.85em;
  color: var(--color-text-muted);
}
.plugins-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.5rem;
}
</style>
