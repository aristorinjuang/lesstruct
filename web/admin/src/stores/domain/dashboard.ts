import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/utils/request'
import type { DashboardStats, ActivityItem } from '@/types/dashboard'

export const useDashboardStore = defineStore('dashboard', () => {
  // State
  const stats = ref<DashboardStats | null>(null)
  const activities = ref<ActivityItem[]>([])
  const loadingCount = ref(0)
  const statsError = ref<Error | null>(null)
  const activityError = ref<Error | null>(null)

  // Computed
  const isLoading = computed(() => loadingCount.value > 0)

  const error = computed(() => statsError.value ?? activityError.value)

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

  async function fetchRecentActivity(): Promise<void> {
    loadingCount.value++
    activityError.value = null

    try {
      // Mock data for now - backend endpoint may not exist yet
      // const response = await api.get<{ data: { activities: ActivityItem[] }; error: null; meta: { timestamp: string; requestId: string } }>('/api/v1/dashboard/recent-activity')
      // activities.value = response.data.data.activities

      // Mock data
      activities.value = []
    } catch (err) {
      activityError.value = err as Error
      throw err
    } finally {
      loadingCount.value--
    }
  }

  function fetchAll(): Promise<void> {
    return Promise.all([
      fetchDashboardStats(),
      fetchRecentActivity(),
    ]) as unknown as Promise<void>
  }

  function clearError(): void {
    statsError.value = null
    activityError.value = null
  }

  return {
    // State
    stats,
    activities,
    isLoading,
    error,

    // Computed
    hasPendingRegistrations,

    // Actions
    fetchDashboardStats,
    fetchRecentActivity,
    fetchAll,
    clearError,
  }
})
