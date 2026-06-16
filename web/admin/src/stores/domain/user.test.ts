import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUserStore } from './user'
import api from '@/utils/request'
import type { User } from '@/types/user'

// Mock the API module
vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    postWithTimeout: vi.fn(),
  },
}))

// Mock the notification store
vi.mock('@/stores/ui/notifications', () => ({
  useNotificationStore: vi.fn(() => ({
    decrementCount: vi.fn(),
  })),
}))

const mockUsers: User[] = [
  {
    id: '1',
    username: 'johndoe',
    email: 'john@example.com',
    role: 'Contributor',
    status: 'Active',
    createdAt: '2026-03-26T14:30:00Z',
  },
  {
    id: '2',
    username: 'janedoe',
    email: 'jane@example.com',
    role: 'Commentator',
    status: 'Pending',
    createdAt: '2026-03-27T10:00:00Z',
  },
  {
    id: '3',
    username: 'bobsmith',
    email: 'bob@example.com',
    role: 'Contributor',
    status: 'Suspended',
    createdAt: '2026-03-25T08:00:00Z',
  },
]

describe('useUserStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('should have empty initial state', () => {
      const store = useUserStore()

      expect(store.users).toEqual([])
      expect(store.pendingUsers).toEqual([])
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })
  })

  describe('computed properties', () => {
    it('should compute pending count correctly', () => {
      const store = useUserStore()
      store.pendingUsers = [mockUsers[1]]

      expect(store.pendingCount).toBe(1)
    })

    it('should compute isLoading from both fetch flags', () => {
      const store = useUserStore()
      store.isUsersLoading = true

      expect(store.isLoading).toBe(true)

      store.isUsersLoading = false
      store.isPendingUsersLoading = true

      expect(store.isLoading).toBe(true)

      store.isPendingUsersLoading = false

      expect(store.isLoading).toBe(false)
    })

    it('should compute error from both error refs', () => {
      const store = useUserStore()
      store.usersError = new Error('users failed')

      expect(store.error).toEqual(new Error('users failed'))

      store.usersError = null
      store.pendingUsersError = new Error('pending failed')

      expect(store.error).toEqual(new Error('pending failed'))

      store.pendingUsersError = null

      expect(store.error).toBe(null)
    })
  })

  describe('fetchUsers', () => {
    it('should fetch users successfully', async () => {
      const store = useUserStore()
      vi.mocked(api.get).mockResolvedValue({
        data: { data: { data: mockUsers, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } } },
      })

      await store.fetchUsers()

      expect(store.users).toEqual(mockUsers)
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })

    it('should handle fetch errors', async () => {
      const store = useUserStore()
      const error = new Error('Failed to fetch')
      vi.mocked(api.get).mockRejectedValue(error)

      await expect(store.fetchUsers()).rejects.toThrow('Failed to fetch')

      expect(store.error).toEqual(error)
      expect(store.isLoading).toBe(false)
    })

    it('should set loading state during fetch', async () => {
      const store = useUserStore()
      let resolveFetch: (value: any) => void
      const fetchPromise = new Promise((resolve) => {
        resolveFetch = resolve
      })
      vi.mocked(api.get).mockReturnValue(fetchPromise as any)

      const fetchCall = store.fetchUsers()
      expect(store.isLoading).toBe(true)

      resolveFetch!({ data: { data: { data: mockUsers, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } } } })
      await fetchCall

      expect(store.isLoading).toBe(false)
    })
  })

  describe('fetchPendingUsers', () => {
    it('should fetch pending users successfully', async () => {
      const store = useUserStore()
      const pendingUsers = [mockUsers[1]]
      vi.mocked(api.get).mockResolvedValue({
        data: { data: { data: pendingUsers, meta: { timestamp: '2026-03-27T10:00:00Z', requestId: '124' } } },
      })

      await store.fetchPendingUsers()

      expect(store.pendingUsers).toEqual(pendingUsers)
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })

    it('should handle fetch errors', async () => {
      const store = useUserStore()
      const error = new Error('Failed to fetch pending')
      vi.mocked(api.get).mockRejectedValue(error)

      await expect(store.fetchPendingUsers()).rejects.toThrow('Failed to fetch pending')

      expect(store.error).toEqual(error)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('approveUser', () => {
    it('should approve user and refresh lists', async () => {
      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: mockUsers, error: null, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } },
      })

      await store.approveUser('1')

      expect(api.post).toHaveBeenCalledWith('/api/admin/users/1/approve', {})
      expect(api.get).toHaveBeenCalledTimes(2) // fetchUsers and fetchPendingUsers
    })

    it('should decrement notification count after approval', async () => {
      const { useNotificationStore } = await import('@/stores/ui/notifications')
      const mockDecrementCount = vi.fn()
      vi.mocked(useNotificationStore).mockReturnValue({
        decrementCount: mockDecrementCount,
      } as any)

      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: mockUsers, error: null, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } },
      })

      await store.approveUser('1')

      expect(mockDecrementCount).toHaveBeenCalledWith('pendingRegistrations')
    })
  })

  describe('rejectUser', () => {
    it('should reject user and refresh pending users', async () => {
      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: [mockUsers[1]], error: null, meta: { timestamp: '2026-03-27T10:00:00Z', requestId: '124' } },
      })

      await store.rejectUser('1')

      expect(api.post).toHaveBeenCalledWith('/api/admin/users/1/reject', { confirmed: true })
      expect(api.get).toHaveBeenCalledTimes(1)
    })

    it('should decrement notification count after rejection', async () => {
      const { useNotificationStore } = await import('@/stores/ui/notifications')
      const mockDecrementCount = vi.fn()
      vi.mocked(useNotificationStore).mockReturnValue({
        decrementCount: mockDecrementCount,
      } as any)

      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: [], error: null, meta: { timestamp: '2026-03-27T10:00:00Z', requestId: '124' } },
      })

      await store.rejectUser('1')

      expect(mockDecrementCount).toHaveBeenCalledWith('pendingRegistrations')
    })
  })

  describe('markAsSpam', () => {
    it('should mark user as spam and refresh pending users', async () => {
      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: [], error: null, meta: { timestamp: '2026-03-27T10:00:00Z', requestId: '124' } },
      })

      await store.markAsSpam('1')

      expect(api.post).toHaveBeenCalledWith('/api/admin/users/1/mark-spam', { confirmed: true })
      expect(api.get).toHaveBeenCalledTimes(1)
    })

    it('should decrement notification count after marking as spam', async () => {
      const { useNotificationStore } = await import('@/stores/ui/notifications')
      const mockDecrementCount = vi.fn()
      vi.mocked(useNotificationStore).mockReturnValue({
        decrementCount: mockDecrementCount,
      } as any)

      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: [], error: null, meta: { timestamp: '2026-03-27T10:00:00Z', requestId: '124' } },
      })

      await store.markAsSpam('1')

      expect(mockDecrementCount).toHaveBeenCalledWith('pendingRegistrations')
    })
  })

  describe('suspendUser', () => {
    it('should suspend user and refresh users', async () => {
      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: mockUsers, error: null, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } },
      })

      await store.suspendUser('1')

      expect(api.post).toHaveBeenCalledWith('/api/admin/users/1/suspend', {})
      expect(api.get).toHaveBeenCalledTimes(1)
    })
  })

  describe('softDeleteUser', () => {
    it('should soft delete user and refresh users', async () => {
      const store = useUserStore()
      vi.mocked(api.post).mockResolvedValue({ data: {} as any })
      vi.mocked(api.get).mockResolvedValue({
        data: { data: mockUsers, error: null, meta: { timestamp: '2026-03-26T14:30:00Z', requestId: '123' } },
      })

      await store.softDeleteUser('1')

      expect(api.post).toHaveBeenCalledWith('/api/admin/users/1/soft-delete', { confirmed: true })
      expect(api.get).toHaveBeenCalledTimes(1)
    })
  })

  describe('clearError', () => {
    it('should clear both error states', () => {
      const store = useUserStore()
      store.usersError = new Error('Test error')
      store.pendingUsersError = new Error('Pending error')

      store.clearError()

      expect(store.usersError).toBe(null)
      expect(store.pendingUsersError).toBe(null)
    })
  })
})
