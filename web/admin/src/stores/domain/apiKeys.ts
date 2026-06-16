import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { ApiKey, CreateApiKeyResponse } from '@/types/apiKey'

export const useApiKeysStore = defineStore('apiKeys', () => {
  const keys = ref<ApiKey[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)
  const isCreating = ref(false)
  const isRevoking = ref(false)

  // Monotonic token used to ignore stale list responses — e.g. when a newer
  // fetch supersedes an older one, or the view unmounts mid-request.
  let lastFetchId = 0

  async function fetchKeys(): Promise<ApiKey[]> {
    isLoading.value = true
    error.value = null
    const requestId = ++lastFetchId
    try {
      const response = await api.get<{ data: ApiKey[] }>('/api/admin/api-keys')
      if (requestId !== lastFetchId) return keys.value
      const data = response.data?.data
      keys.value = Array.isArray(data) ? data : []
      return keys.value
    } catch (err) {
      if (requestId === lastFetchId) {
        error.value = err as Error
      }
      throw err
    } finally {
      if (requestId === lastFetchId) {
        isLoading.value = false
      }
    }
  }

  // Background refresh used after create/revoke. It deliberately does NOT touch
  // `isLoading` (so the list doesn't flicker mid-action) and does NOT surface
  // failures on the load-error banner (the create/revoke itself succeeded).
  // A successful refresh clears any stale load error since the list is fresh.
  async function silentRefresh(): Promise<void> {
    const requestId = ++lastFetchId
    try {
      const response = await api.get<{ data: ApiKey[] }>('/api/admin/api-keys')
      if (requestId !== lastFetchId) return
      const data = response.data?.data
      keys.value = Array.isArray(data) ? data : []
      error.value = null
    } catch {
      // Best-effort refresh — failures here are non-critical.
    }
  }

  async function create(name: string): Promise<CreateApiKeyResponse> {
    if (isCreating.value) {
      throw new Error('Already processing')
    }
    isCreating.value = true
    try {
      const response = await api.post<{ data: CreateApiKeyResponse }>('/api/admin/api-keys', { name })
      const payload = response.data?.data
      if (!payload) {
        throw new Error('Malformed response from server')
      }
      // Refresh the list so the new key appears with its masked prefix. The
      // create response omits `prefix`/`lastUsedAt`, so the list is re-fetched.
      await silentRefresh()
      return payload
    } finally {
      isCreating.value = false
    }
  }

  async function revoke(id: number): Promise<void> {
    if (isRevoking.value) {
      throw new Error('Already processing')
    }
    isRevoking.value = true
    try {
      await api.delete(`/api/admin/api-keys/${id}`)
      await silentRefresh()
    } finally {
      isRevoking.value = false
    }
  }

  function clearError(): void {
    error.value = null
  }

  return {
    keys,
    isLoading,
    error,
    isCreating,
    isRevoking,
    fetchKeys,
    create,
    revoke,
    clearError,
  }
})
