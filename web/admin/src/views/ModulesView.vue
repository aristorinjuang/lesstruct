<script setup lang="ts">
import { onMounted, ref } from 'vue'
import api from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import DeleteConfirmDialog from '@/components/molecules/DeleteConfirmDialog.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useModgenStore } from '@/stores/domain/modgen'

const store = useModgenStore()
const applyAvailable = ref(false)
const unapplying = ref<string | null>(null)
const replacing = ref<string | null>(null)

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
    const response = await api.get<{ features: { moduleApply?: boolean } }>('/api/health')
    applyAvailable.value = response.data.features?.moduleApply === true
  } catch {
    applyAvailable.value = false
  }
  try {
    await store.fetchProjects()
  } catch {
    displayToast('Failed to load module projects', 'error')
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

async function handleApply(name: string) {
  try {
    const { apply } = await store.apply(name)
    if (apply.verified) {
      displayToast(`Applied ${name} — rebuild ./cmd/lesstruct-custom and redeploy.`)
    } else {
      displayToast('Applied with failing checks.', 'error')
    }
    await store.fetchProjects()
  } catch {
    displayToast('Apply failed', 'error')
  }
}

function requestReplace(name: string) {
  replacing.value = name
}

function cancelReplace() {
  replacing.value = null
}

async function confirmReplace() {
  const name = replacing.value
  if (!name) return
  try {
    const { apply } = await store.apply(name, { replace: true, confirm: true })
    replacing.value = null
    if (apply.verified) {
      displayToast(`Replaced ${name} — rebuild ./cmd/lesstruct-custom and redeploy.`)
    } else {
      displayToast('Replaced with failing checks.', 'error')
    }
    await store.fetchProjects()
  } catch {
    displayToast('Replace failed', 'error')
  }
}

async function handleDelete(name: string) {
  const wasApplied = store.projects.some((p) => p.name === name && p.applied)
  try {
    await store.remove(name)
    if (wasApplied) {
      displayToast(
        `Deleted ${name} (staged copy) — the module stays applied in your checkout.`,
        'error',
      )
    } else {
      displayToast(`Deleted ${name}`)
    }
  } catch {
    displayToast('Delete failed', 'error')
  }
}

function requestUnapply(name: string) {
  unapplying.value = name
}

function cancelUnapply() {
  unapplying.value = null
}

async function confirmUnapply() {
  const name = unapplying.value
  if (!name) return
  try {
    const result = await store.unapply(name)
    unapplying.value = null
    if (result.verified) {
      displayToast(`Removed ${name} from the checkout — rebuild ./cmd/lesstruct-custom and redeploy.`)
    } else {
      displayToast('Removed with failing checks.', 'error')
    }
    await store.fetchProjects()
  } catch {
    displayToast('Remove failed', 'error')
  }
}
</script>

<template>
  <div class="modules-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Modules</h1>
        <p class="page-subtitle">Staged module projects. Copy a tree into a checkout and rebuild.</p>
      </div>
    </header>

    <h2 class="modules-section">Projects</h2>
    <div class="modules-grid">
      <div v-for="project in store.projects" :key="project.name" class="modules-card">
        <h3 class="modules-title">{{ project.name }}</h3>
        <p class="modules-meta">
          {{ project.surfaces.join(', ') }} · {{ project.fileCount }} files · {{ project.source }}
        </p>
        <p v-if="project.applied" class="modules-badge">applied</p>
        <div class="modules-actions">
          <Button type="button" variant="neutral" size="small" @click="handleDownload(project.name)"
            >Download</Button
          >
          <Button
            v-if="applyAvailable && !project.applied"
            type="button"
            variant="primary"
            size="small"
            @click="handleApply(project.name)"
            >Apply to checkout</Button
          >
          <Button
            v-if="applyAvailable && project.applied"
            type="button"
            variant="primary"
            size="small"
            @click="requestReplace(project.name)"
            >Replace applied</Button
          >
          <Button
            v-if="applyAvailable && project.applied"
            type="button"
            variant="danger"
            size="small"
            @click="requestUnapply(project.name)"
            >Remove from checkout</Button
          >
          <Button type="button" variant="danger" size="small" @click="handleDelete(project.name)"
            >Delete</Button
          >
        </div>
      </div>
    </div>

    <DeleteConfirmDialog
      title="Remove from checkout"
      :item-name="unapplying ?? ''"
      :is-open="unapplying !== null"
      :is-loading="store.isLoading"
      warning="This deletes the applied module tree, its registrations, and the custom-edition wiring. The staged copy is untouched."
      @confirm="confirmUnapply"
      @cancel="cancelUnapply"
    />

    <DeleteConfirmDialog
      title="Replace applied module"
      :item-name="replacing ?? ''"
      :is-open="replacing !== null"
      :is-loading="store.isLoading"
      warning="This overwrites the applied module tree with the staged project and re-verifies it. Tracked edits inside the applied tree block the replace."
      @confirm="confirmReplace"
      @cancel="cancelReplace"
    />

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
.modules-view {
  max-width: 100%;
}
.modules-section {
  margin: 1rem 0 0.5rem;
}
.modules-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
}
.modules-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
}
.modules-title {
  margin: 0;
}
.modules-meta {
  margin: 0;
}
.modules-badge {
  margin: 0;
  font-size: 0.85em;
  color: var(--color-text-muted);
}
.modules-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.5rem;
}
</style>
