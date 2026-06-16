import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/utils/request'
import { useNotificationStore } from '@/stores/ui/notifications'
import type { User, UserApiResponse } from '@/types/user'

export const useUserStore = defineStore('user', () => {
  // State
  const users = ref<User[]>([])
  const pendingUsers = ref<User[]>([])
  const isUsersLoading = ref(false)
  const isPendingUsersLoading = ref(false)
  const usersError = ref<Error | null>(null)
  const pendingUsersError = ref<Error | null>(null)
  const isProcessing = ref(false)

  // Computed
  const pendingCount = computed(() => pendingUsers.value.length)

  const isLoading = computed(
    () => isUsersLoading.value || isPendingUsersLoading.value
  )

  const error = computed(() => usersError.value || pendingUsersError.value)

  // Actions
  async function fetchUsers(): Promise<void> {
    isUsersLoading.value = true
    usersError.value = null

    try {
      const response = await api.get<any>('/api/admin/users')
      // Backend response structure: { data: { data: [...], meta: {...} } }
      users.value = response.data.data?.data || []
    } catch (err) {
      usersError.value = err as Error
      throw err
    } finally {
      isUsersLoading.value = false
    }
  }

  async function fetchPendingUsers(): Promise<void> {
    isPendingUsersLoading.value = true
    pendingUsersError.value = null

    try {
      const response = await api.get<any>('/api/admin/pending-users')
      // Backend response structure: { data: { data: [...], meta: {...} } }
      pendingUsers.value = response.data.data?.data || []
    } catch (err) {
      pendingUsersError.value = err as Error
      throw err
    } finally {
      isPendingUsersLoading.value = false
    }
  }

  async function approveUser(userId: string): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.post(`/api/admin/users/${userId}/approve`, {})

      // Decrement before refresh to ensure badge stays in sync
      const notificationStore = useNotificationStore()
      notificationStore.decrementCount('pendingRegistrations')

      // Refresh both lists
      await Promise.all([fetchUsers(), fetchPendingUsers()])
    } finally {
      isProcessing.value = false
    }
  }

  async function rejectUser(userId: string): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.post(`/api/admin/users/${userId}/reject`, { confirmed: true })

      // Decrement before refresh
      const notificationStore = useNotificationStore()
      notificationStore.decrementCount('pendingRegistrations')

      // Refresh pending users
      await fetchPendingUsers()
    } finally {
      isProcessing.value = false
    }
  }

  async function markAsSpam(userId: string): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.post(`/api/admin/users/${userId}/mark-spam`, { confirmed: true })

      // Decrement before refresh
      const notificationStore = useNotificationStore()
      notificationStore.decrementCount('pendingRegistrations')

      // Refresh pending users
      await fetchPendingUsers()
    } finally {
      isProcessing.value = false
    }
  }

  async function suspendUser(userId: string): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.post(`/api/admin/users/${userId}/suspend`, {})
      await fetchUsers()
    } finally {
      isProcessing.value = false
    }
  }

  async function softDeleteUser(userId: string): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.post(`/api/admin/users/${userId}/soft-delete`, { confirmed: true })
      await fetchUsers()
    } finally {
      isProcessing.value = false
    }
  }

  async function createUser(data: { username: string; name: string; email: string; role: string; customFields?: Record<string, any> }): Promise<string> {
    if (isProcessing.value) return ''
    isProcessing.value = true

    try {
      const response = await api.post<{ data: { password: string } }>('/api/admin/users', data)
      await fetchUsers()
      return response.data.data.password
    } finally {
      isProcessing.value = false
    }
  }

  async function updateUser(userId: string, data: { name?: string; email?: string; role?: string; customFields?: Record<string, any> }): Promise<void> {
    if (isProcessing.value) return
    isProcessing.value = true

    try {
      await api.put(`/api/admin/users/${userId}`, data as Record<string, unknown>)
      try {
        await fetchUsers()
      } catch {
        // Update succeeded — list refresh failure is non-critical
      }
    } finally {
      isProcessing.value = false
    }
  }

  function clearError(): void {
    usersError.value = null
    pendingUsersError.value = null
  }

  return {
    // State
    users,
    pendingUsers,
    isUsersLoading,
    isPendingUsersLoading,
    usersError,
    pendingUsersError,
    isProcessing,

    // Computed
    pendingCount,
    isLoading,
    error,

    // Actions
    fetchUsers,
    fetchPendingUsers,
    approveUser,
    rejectUser,
    markAsSpam,
    suspendUser,
    softDeleteUser,
    createUser,
    updateUser,
    clearError,
  }
})
