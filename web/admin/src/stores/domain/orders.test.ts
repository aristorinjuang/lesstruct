import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useOrdersStore } from './orders'
import type { Order } from '@/types/order'
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

function buildOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 42,
    orderNumber: 'ORD-000042',
    status: 'pending',
    customerEmail: 'customer@example.com',
    customerName: 'Customer Name',
    customerPhone: '',
    shipTo: {
      street: 'Jl. Example No. 1',
      city: 'Jakarta',
      province: 'DKI Jakarta',
      postalCode: '10110',
    },
    currency: 'IDR',
    subtotal: 299000,
    shippingMinor: 0,
    totalMinor: 299000,
    items: [
      {
        id: 1,
        productContentId: 7,
        variantId: null,
        sku: 'SHIRT-001',
        title: 'Shirt',
        unitPrice: 299000,
        quantity: 1,
        lineTotal: 299000,
      },
    ],
    createdAt: '2026-10-18T00:00:00Z',
    updatedAt: '2026-10-18T00:00:00Z',
    ...overrides,
  }
}

describe('Orders Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchOrders', () => {
    it('success — populates orders, cursor, and hasMore', async () => {
      const pageOrders = [buildOrder(), buildOrder({ id: 43 })]
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: pageOrders,
          meta: { pagination: { nextCursor: '42', hasMore: true } },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useOrdersStore()
      const result = await store.fetchOrders()

      expect(api.get).toHaveBeenCalledWith('/api/v1/orders', { params: {} })
      expect(result).toEqual(pageOrders)
      expect(store.orders).toEqual(pageOrders)
      expect(store.nextCursor).toBe('42')
      expect(store.hasMore).toBe(true)
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })

    it('passes the status filter and cursor as params', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: { data: [], meta: undefined },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useOrdersStore()
      await store.fetchOrders('paid', '17')

      expect(api.get).toHaveBeenCalledWith('/api/v1/orders', {
        params: { status: 'paid', cursor: '17' },
      })
      expect(store.orders).toEqual([])
      expect(store.nextCursor).toBe(null)
      expect(store.hasMore).toBe(false)
    })

    it('appends the fetched page when append is set', async () => {
      const firstPage = [buildOrder()]
      const secondPage = [buildOrder({ id: 43, orderNumber: 'ORD-000043' })]

      const store = useOrdersStore()
      vi.mocked(api.get).mockResolvedValue({
        data: { data: firstPage, meta: { pagination: { nextCursor: '42', hasMore: true } } },
      } as unknown as Awaited<ReturnType<typeof api.get>>)
      await store.fetchOrders()

      vi.mocked(api.get).mockResolvedValue({
        data: { data: secondPage, meta: { pagination: { hasMore: false } } },
      } as unknown as Awaited<ReturnType<typeof api.get>>)
      await store.fetchOrders(null, '42', true)

      expect(store.orders).toHaveLength(2)
      expect(store.hasMore).toBe(false)
      expect(store.nextCursor).toBe(null)
    })

    it('error — records error and rethrows', async () => {
      const err = apiError('Boom', 500, 'INTERNAL')
      vi.mocked(api.get).mockRejectedValue(err)

      const store = useOrdersStore()

      await expect(store.fetchOrders()).rejects.toThrow('Boom')
      expect(store.error).toBe(err)
      expect(store.isLoading).toBe(false)
    })

    it('ignores stale responses after a newer fetch supersedes them', async () => {
      const newer = [buildOrder()]
      let resolveOld: (value: unknown) => void = () => {}
      const oldRequest = new Promise((resolve) => {
        resolveOld = resolve
      })
      vi.mocked(api.get)
        .mockImplementationOnce(() => oldRequest as never)
        .mockResolvedValueOnce({
          data: { data: newer, meta: undefined },
        } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useOrdersStore()
      const oldCall = store.fetchOrders()
      const newCall = store.fetchOrders()
      resolveOld({ data: { data: [buildOrder({ id: 7 })], meta: undefined } })
      await oldCall
      await newCall

      expect(store.orders).toEqual(newer)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('fetchOrder', () => {
    it('success — sets currentOrder and returns it', async () => {
      const order = buildOrder()
      vi.mocked(api.get).mockResolvedValue({
        data: { data: order },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useOrdersStore()
      const result = await store.fetchOrder(42)

      expect(api.get).toHaveBeenCalledWith('/api/v1/orders/42')
      expect(result).toEqual(order)
      expect(store.currentOrder).toEqual(order)
      expect(store.isFetchingOne).toBe(false)
    })

    it('not-found — clears currentOrder and returns null without throwing', async () => {
      vi.mocked(api.get).mockRejectedValue(apiError('Not found', 404, 'NOT_FOUND'))

      const store = useOrdersStore()
      const result = await store.fetchOrder(999)

      expect(result).toBe(null)
      expect(store.currentOrder).toBe(null)
      expect(store.error?.message).toBe('Not found')
    })
  })

  describe('transition', () => {
    it('success — posts the status and stores the updated order', async () => {
      const updated = buildOrder({ status: 'paid', shipTo: buildOrder().shipTo })
      vi.mocked(api.post).mockResolvedValue({
        data: { data: updated },
      } as unknown as Awaited<ReturnType<typeof api.post>>)

      const store = useOrdersStore()
      const result = await store.transition(42, 'paid')

      expect(api.post).toHaveBeenCalledWith('/api/v1/orders/42/status', { status: 'paid' })
      expect(result).toEqual(updated)
      expect(store.currentOrder).toEqual(updated)
      expect(store.isTransitioning).toBe(false)
    })

    it('conflict — rethrows the structured error and resets the flag', async () => {
      vi.mocked(api.post).mockRejectedValue(apiError('Invalid transition', 409, 'CONFLICT'))

      const store = useOrdersStore()

      await expect(store.transition(42, 'cancelled')).rejects.toThrow('Invalid transition')
      expect(store.isTransitioning).toBe(false)
    })
  })

  describe('clearError', () => {
    it('clears a stored error', async () => {
      vi.mocked(api.get).mockRejectedValue(apiError('Boom', 500, 'INTERNAL'))
      const store = useOrdersStore()
      await expect(store.fetchOrders()).rejects.toThrow('Boom')
      expect(store.error).not.toBe(null)

      store.clearError()
      expect(store.error).toBe(null)
    })
  })
})
