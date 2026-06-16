<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import Toast from '@/components/molecules/Toast.vue'
import ApiKeyList from '@/components/organisms/ApiKeyList.vue'
import ApiKeyCreateDialog from '@/components/organisms/ApiKeyCreateDialog.vue'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'
import { useApiKeys } from '@/composables/useApiKeys'
import type { ApiError } from '@/utils/request'

const { keys, isLoading, error, fetchKeys, revoke } = useApiKeys()

const showCreateDialog = ref(false)
const revokeTargetId = ref<number | null>(null)
const isRevoking = ref(false)
const revokeAborted = ref(false)
const isMounted = ref(true)

// Toast state (mirrors UserManagementView.vue)
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

function loadApiKeys() {
  fetchKeys().catch(() => {
    if (isMounted.value) displayToast('Failed to load API keys', 'error')
  })
}

function handleRevokeClick(id: number) {
  revokeTargetId.value = id
}

function handleRevokeCancel() {
  // If a revoke is in flight, suppress the eventual success/error toast — the
  // shared request wrapper isn't abortable from here, so the result is ignored.
  if (isRevoking.value) revokeAborted.value = true
  revokeTargetId.value = null
}

async function handleRevokeConfirm() {
  if (isRevoking.value) return
  if (revokeTargetId.value === null) return
  const targetId = revokeTargetId.value
  isRevoking.value = true
  try {
    await revoke(targetId)
    if (!revokeAborted.value && isMounted.value) displayToast('API key revoked')
  } catch (err) {
    if (!revokeAborted.value && isMounted.value) {
      const code = (err as ApiError)?.code
      const msg = code === 'NOT_FOUND' ? 'This key no longer exists' : 'Failed to revoke API key'
      displayToast(msg, 'error')
    }
  } finally {
    isRevoking.value = false
    revokeAborted.value = false
    revokeTargetId.value = null
  }
}

onMounted(() => {
  loadApiKeys()
})

onUnmounted(() => {
  isMounted.value = false
})
</script>

<template>
  <div class="api-keys-view">
    <header class="page-header--stacked">
      <div class="api-keys-view__header-row">
        <div>
          <h1 class="page-title">API Keys</h1>
          <p class="page-subtitle">
            API keys allow programmatic access to your Lesstruct content via the REST API and CLI.
          </p>
        </div>
        <button
          type="button"
          class="api-keys-view__create-button"
          @click="showCreateDialog = true"
        >
          Create Key
        </button>
      </div>
    </header>

    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <!-- Error state (list + dialogs stay mounted regardless, so Create/Revoke remain reachable) -->
    <div v-if="error && keys.length === 0" class="api-keys-view__error">
      <p>Failed to load API keys. Please try again.</p>
      <button type="button" class="api-keys-view__retry-button" :disabled="isLoading" @click="loadApiKeys">
        Retry
      </button>
    </div>

    <!-- Main content (list handles its own loading + empty states) -->
    <ApiKeyList
      v-else
      :keys="keys"
      :is-loading="isLoading"
      @revoke="handleRevokeClick"
      @create="showCreateDialog = true"
    />

    <ApiKeyCreateDialog
      :is-open="showCreateDialog"
      @close="showCreateDialog = false"
      @created="displayToast('API key created')"
    />

    <ConfirmationDialog
      :is-open="revokeTargetId !== null"
      title="Revoke API Key"
      confirm-button-text="Revoke"
      message="Revoking this key immediately disables it. Any agent or tool using it will lose access. This cannot be undone."
      @confirm="handleRevokeConfirm"
      @cancel="handleRevokeCancel"
    />
  </div>
</template>

<style scoped>
.api-keys-view__header-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.api-keys-view__create-button {
  flex-shrink: 0;
  padding: 0.5rem 1rem;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
  transition: background-color 0.2s;
}

.api-keys-view__create-button:hover {
  background-color: var(--color-interactive-hover);
}

.api-keys-view__create-button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.api-keys-view__error {
  padding: 2rem;
  text-align: center;
  color: var(--color-error-dark);
}

.api-keys-view__retry-button {
  margin-top: 1rem;
  padding: 0.625rem 1rem;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
}

.api-keys-view__retry-button:hover:not(:disabled) {
  background-color: var(--color-interactive-hover);
}

.api-keys-view__retry-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 639px) {
  .api-keys-view__header-row {
    flex-direction: column;
    align-items: stretch;
  }

  .api-keys-view__create-button {
    width: 100%;
  }
}
</style>
