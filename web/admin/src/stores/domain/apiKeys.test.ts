import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useApiKeysStore } from './apiKeys'
import type { ApiKey, CreateApiKeyResponse } from '@/types/apiKey'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

function apiError(message: string, statusCode: number, code: string): Error {
  const err = new Error(message)
  return Object.assign(err, { statusCode, code })
}

describe('API Keys Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchKeys', () => {
    it('success — populates keys from response.data.data', async () => {
      const mockKeys: ApiKey[] = [
        {
          id: 1,
          name: 'Production key',
          prefix: 'lesstruct_1••••',
          createdAt: '2026-06-14T00:00:00Z',
          lastUsedAt: null,
          expiresAt: null,
          revokedAt: null,
        },
        {
          id: 2,
          name: 'CLI key',
          prefix: 'lesstruct_2••••',
          createdAt: '2026-06-14T00:00:00Z',
          lastUsedAt: '2026-06-14T12:00:00Z',
          expiresAt: null,
          revokedAt: null,
        },
      ]

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: mockKeys,
          error: null,
          meta: { timestamp: '2026-06-14T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useApiKeysStore()

      const result = await store.fetchKeys()

      expect(api.get).toHaveBeenCalledWith('/api/admin/api-keys')
      expect(result).toEqual(mockKeys)
      expect(store.keys).toEqual(mockKeys)
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })

    it('empty — null data coerces to []', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: null,
          error: null,
          meta: { timestamp: '2026-06-14T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useApiKeysStore()

      const result = await store.fetchKeys()

      expect(result).toEqual([])
      expect(store.keys).toEqual([])
      expect(store.isLoading).toBe(false)
    })

    it('empty — non-array data coerces to []', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: { not: 'an array' },
          error: null,
          meta: { timestamp: '2026-06-14T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useApiKeysStore()

      const result = await store.fetchKeys()

      expect(result).toEqual([])
      expect(store.keys).toEqual([])
      expect(store.isLoading).toBe(false)
    })

    it('error — network failure sets error and re-throws', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Network error'))

      const store = useApiKeysStore()

      await expect(store.fetchKeys()).rejects.toThrow('Network error')

      expect(store.error).toBeInstanceOf(Error)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('create', () => {
    it('success — returns the full key payload and refreshes the list', async () => {
      const createPayload: CreateApiKeyResponse = {
        key: 'lesstruct_3_abcsecret',
        id: 3,
        name: 'New key',
        keyPrefix: 'lesstruct_3',
        createdAt: '2026-06-14T00:00:00Z',
      }
      const refreshedKeys: ApiKey[] = [
        {
          id: 3,
          name: 'New key',
          prefix: 'lesstruct_3••••',
          createdAt: '2026-06-14T00:00:00Z',
          lastUsedAt: null,
          expiresAt: null,
          revokedAt: null,
        },
      ]

      vi.mocked(api.post).mockResolvedValue({
        data: {
          data: createPayload,
          error: null,
          meta: { timestamp: '2026-06-14T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.post>>)
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: refreshedKeys,
          error: null,
          meta: { timestamp: '2026-06-14T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useApiKeysStore()

      const result = await store.create('New key')

      expect(api.post).toHaveBeenCalledWith('/api/admin/api-keys', { name: 'New key' })
      // create() refreshes the list via silentRefresh()
      expect(api.get).toHaveBeenCalledWith('/api/admin/api-keys')
      expect(result).toEqual(createPayload)
      expect(store.keys).toEqual(refreshedKeys)
      expect(store.isCreating).toBe(false)
    })

    it('error — VALIDATION_ERROR re-thrown with code', async () => {
      vi.mocked(api.post).mockRejectedValue(apiError('Invalid key name', 400, 'VALIDATION_ERROR'))

      const store = useApiKeysStore()

      await expect(store.create('dup')).rejects.toMatchObject({ code: 'VALIDATION_ERROR' })

      expect(store.isCreating).toBe(false)
    })

    it('blocks double-submit via isCreating guard', async () => {
      const store = useApiKeysStore()
      store.isCreating = true

      await expect(store.create('x')).rejects.toThrow('Already processing')

      // Guard short-circuits before any API call.
      expect(api.post).not.toHaveBeenCalled()
    })
  })

  describe('revoke', () => {
    it('success — calls DELETE and refreshes the list', async () => {
      const refreshedKeys: ApiKey[] = [
        {
          id: 1,
          name: 'Revoked key',
          prefix: 'lesstruct_1••••',
          createdAt: '2026-06-14T00:00:00Z',
          lastUsedAt: null,
          expiresAt: null,
          revokedAt: '2026-06-14T12:00:00Z',
        },
      ]

      vi.mocked(api.delete).mockResolvedValue({
        data: {
          data: { id: 1, revokedAt: '2026-06-14T12:00:00Z' },
          error: null,
          meta: { timestamp: '2026-06-14T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.delete>>)
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: refreshedKeys,
          error: null,
          meta: { timestamp: '2026-06-14T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useApiKeysStore()

      await store.revoke(1)

      expect(api.delete).toHaveBeenCalledWith('/api/admin/api-keys/1')
      // revoke() refreshes the list via silentRefresh()
      expect(api.get).toHaveBeenCalledWith('/api/admin/api-keys')
      expect(store.keys).toEqual(refreshedKeys)
      expect(store.isRevoking).toBe(false)
    })

    it('error — NOT_FOUND re-thrown with code', async () => {
      vi.mocked(api.delete).mockRejectedValue(apiError('Not found', 404, 'NOT_FOUND'))

      const store = useApiKeysStore()

      await expect(store.revoke(999)).rejects.toMatchObject({ code: 'NOT_FOUND' })

      expect(store.isRevoking).toBe(false)
    })

    it('blocks double-submit via isRevoking guard (throws, never silently no-ops)', async () => {
      const store = useApiKeysStore()
      store.isRevoking = true

      await expect(store.revoke(1)).rejects.toThrow('Already processing')

      // Guard short-circuits before any API call.
      expect(api.delete).not.toHaveBeenCalled()
    })
  })

  describe('clearError', () => {
    it('clears the stored error', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Network error'))

      const store = useApiKeysStore()

      await expect(store.fetchKeys()).rejects.toThrow('Network error')
      expect(store.error).toBeInstanceOf(Error)

      store.clearError()

      expect(store.error).toBe(null)
    })
  })
})
