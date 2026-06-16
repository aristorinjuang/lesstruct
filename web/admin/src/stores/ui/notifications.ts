import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { NotificationCounts } from '@/types/notifications'

export const useNotificationStore = defineStore('notifications', () => {
  // State
  const counts = ref<NotificationCounts>({
    pendingRegistrations: 0,
    pendingComments: 0,
    pendingUpdates: 0,
  })

  // Computed
  const hasPendingNotifications = computed(() =>
    Object.values(counts.value).some((count) => count > 0)
  )

  const pendingRegistrations = computed(() => counts.value.pendingRegistrations)

  // Actions
  function syncFromDashboard(pendingRegistrations: number): void {
    counts.value.pendingRegistrations = Math.max(0, pendingRegistrations)
  }

  function decrementCount(type: keyof NotificationCounts): void {
    if (counts.value[type] > 0) {
      counts.value[type]--
    }
  }

  return {
    // State
    counts,

    // Computed
    hasPendingNotifications,
    pendingRegistrations,

    // Actions
    syncFromDashboard,
    decrementCount,
  }
})
