import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePaymentsStore } from './payments'
import type { Payment } from '@/types/payment'
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

function buildPayment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: 7,
    orderId: 42,
    method: 'manual',
    status: 'pending',
    reference: 'BR-12345',
    amount: 522500,
    createdAt: '2026-10-18T00:00:00Z',
    updatedAt: '2026-10-18T00:00:00Z',
    ...overrides,
  }
}

const mockedGet = api.get as unknown as ReturnType<typeof vi.fn>
const mockedPost = api.post as unknown as ReturnType<typeof vi.fn>

describe('Payments Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetches the newest-first list with the status filter', async () => {
    const store = usePaymentsStore()
    mockedGet.mockResolvedValue({
      data: { data: [buildPayment()], meta: { pagination: { hasMore: false } } },
    })

    const payments = await store.fetchPayments('pending')

    expect(mockedGet).toHaveBeenCalledWith('/api/v1/payments', {
      params: { status: 'pending' },
    })
    expect(payments).toHaveLength(1)
    expect(store.hasMore).toBe(false)
    expect(store.nextCursor).toBeNull()
    expect(store.isLoading).toBe(false)
  })

  it('appends load-more pages off the cursor', async () => {
    const store = usePaymentsStore()
    mockedGet
      .mockResolvedValueOnce({
        data: {
          data: [buildPayment({ id: 7 })],
          meta: { pagination: { nextCursor: 'Ng', hasMore: true } },
        },
      })
      .mockResolvedValueOnce({
        data: { data: [buildPayment({ id: 6 })], meta: { pagination: {} } },
      })

    await store.fetchPayments(null)
    await store.fetchPayments(null, 'Ng', true)

    expect(mockedGet).toHaveBeenLastCalledWith('/api/v1/payments', {
      params: { cursor: 'Ng' },
    })
    expect(store.payments.map((payment) => payment.id)).toEqual([7, 6])
  })

  it('records list failures', async () => {
    const store = usePaymentsStore()
    mockedGet.mockRejectedValue(apiError('boom', 500, 'INTERNAL_ERROR'))

    await expect(store.fetchPayments(null)).rejects.toThrow('boom')
    expect(store.error?.message).toBe('boom')
  })

  it('fetches one payment with its order context', async () => {
    const store = usePaymentsStore()
    const detail = {
      ...buildPayment(),
      order: { orderNumber: 'ORD-000042', status: 'pending', totalMinor: 522500 },
    }
    mockedGet.mockResolvedValue({ data: { data: detail } })

    const found = await store.fetchPayment(7)

    expect(mockedGet).toHaveBeenCalledWith('/api/v1/payments/7')
    expect(found?.order.orderNumber).toBe('ORD-000042')
    expect(store.currentPayment?.order.totalMinor).toBe(522500)
  })

  it('fetches one order history scoped by order id', async () => {
    const store = usePaymentsStore()
    mockedGet.mockResolvedValue({ data: { data: [buildPayment()] } })

    const history = await store.fetchOrderPayments(42)

    expect(mockedGet).toHaveBeenCalledWith('/api/v1/payments', {
      params: { orderId: '42' },
    })
    expect(history).toHaveLength(1)
    expect(store.orderPayments).toHaveLength(1)
    expect(store.isFetchingHistory).toBe(false)
  })

  it('verifies a pending proof', async () => {
    const store = usePaymentsStore()
    mockedPost.mockResolvedValue({
      data: { data: buildPayment({ status: 'verified', verifiedBy: 'admin' }) },
    })

    const decided = await store.verify(7)

    expect(mockedPost).toHaveBeenCalledWith('/api/v1/payments/7/verify', {})
    expect(decided?.status).toBe('verified')
    expect(store.isDeciding).toBe(false)
  })

  it('rejects a pending proof', async () => {
    const store = usePaymentsStore()
    mockedPost.mockResolvedValue({ data: { data: buildPayment({ status: 'rejected' }) } })

    const decided = await store.reject(7)

    expect(mockedPost).toHaveBeenCalledWith('/api/v1/payments/7/reject', {})
    expect(decided?.status).toBe('rejected')
    expect(store.isDeciding).toBe(false)
  })

  it('clears the error', async () => {
    const store = usePaymentsStore()
    mockedGet.mockRejectedValue(apiError('boom', 500, 'INTERNAL_ERROR'))

    await expect(store.fetchPayments(null)).rejects.toThrow('boom')
    store.clearError()
    expect(store.error).toBeNull()
  })
})
