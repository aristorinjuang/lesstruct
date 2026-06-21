import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/utils/request'
import type { DashboardStats } from '@/types/dashboard'

export const useDashboardStore = defineStore('dashboard', () => {
  // State
  const stats = ref<DashboardStats | null>(null)
  const loadingCount = ref(0)
  const statsError = ref<Error | null>(null)

  // Computed
  const isLoading = computed(() => loadingCount.value > 0)

  const error = computed(() => statsError.value)

  const hasPendingRegistrations = computed(() =>
    (stats.value?.pendingRegistrations ?? 0) > 0
  )

  // Actions
  async function fetchDashboardStats(): Promise<void> {
    loadingCount.value++
    statsError.value = null

    try {
      const response = await api.get<{ data: DashboardStats; error: null; meta: { timestamp: string; requestId: string } }>('/api/v1/dashboard/stats')
      stats.value = response.data.data
    } catch (err) {
      statsError.value = err as Error
      throw err
    } finally {
      loadingCount.value--
    }
  }

  function fetchAll(): Promise<void> {
    return fetchDashboardStats()
  }

  function clearError(): void {
    statsError.value = null
  }

  return {
    // State
    stats,
    isLoading,
    error,

    // Computed
    hasPendingRegistrations,

    // Actions
    fetchDashboardStats,
    fetchAll,
    clearError,
  }
})
