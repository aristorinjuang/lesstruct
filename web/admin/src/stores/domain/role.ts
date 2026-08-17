import { ref } from 'vue'
import { defineStore } from 'pinia'
import api from '@/utils/request'
import type { MeCapabilities, Role, RolesResponse } from '@/types/role'

// useRoleStore loads the config-driven role catalog (GET /api/v1/roles) once.
// It exposes the assignable roles for the user management modals and the
// caller's own capabilities for navigation/content surfacing. When the endpoint
// is unavailable (legacy deployment without a role service → 404
// roles_unavailable) `capabilitiesLoaded` stays false and callers fall back to
// the historical hardcoded behavior.
export const useRoleStore = defineStore('role', () => {
  // State
  const roles = ref<Role[]>([])
  const me = ref<MeCapabilities | null>(null)
  const capabilitiesLoaded = ref(false)
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  // Actions
  async function load(): Promise<void> {
    if (capabilitiesLoaded.value || isLoading.value) return
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<RolesResponse>('/api/v1/roles')
      roles.value = response.data.data?.roles ?? []
      me.value = response.data.data?.me ?? null
      capabilitiesLoaded.value = true
    } catch (err) {
      error.value = err as Error
      capabilitiesLoaded.value = false
      me.value = null
      roles.value = []
    } finally {
      isLoading.value = false
    }
  }

  return {
    // State
    roles,
    me,
    capabilitiesLoaded,
    isLoading,
    error,

    // Actions
    load,
  }
})