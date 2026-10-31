import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { Payment, PaymentDetail, PaymentListMeta, PaymentStatus } from '@/types/payment'

interface PaymentListResponse {
  data: Payment[]
  meta?: PaymentListMeta
}

interface PaymentDetailResponse {
  data: PaymentDetail
}

interface PaymentResponse {
  data: Payment
}

export const usePaymentsStore = defineStore('payments', () => {
  const payments = ref<Payment[]>([])
  const hasMore = ref(false)
  const nextCursor = ref<string | null>(null)
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  const currentPayment = ref<PaymentDetail | null>(null)
  const isFetchingOne = ref(false)
  const isDeciding = ref(false)

  const orderPayments = ref<Payment[]>([])
  const isFetchingHistory = ref(false)

  // Monotonic token used to ignore stale list responses — e.g. when a newer
  // fetch supersedes an older one, or the view unmounts mid-request.
  let lastFetchId = 0

  /**
   * Fetches the newest-first admin payment list. The status filter narrows
   * the page; pass a cursor to key off the previous page. `append` is the
   * load-more path: the fetched page was produced with the previous page's
   * cursor, so it joins the list instead of replacing it.
   */
  async function fetchPayments(
    status: PaymentStatus | null = null,
    cursor: string | null = null,
    append = false,
  ): Promise<Payment[]> {
    isLoading.value = true
    error.value = null
    const requestId = ++lastFetchId
    try {
      const params: Record<string, string> = {}
      if (status) params.status = status
      if (cursor) params.cursor = cursor
      const response = await api.get<PaymentListResponse>('/api/v1/payments', { params })
      if (requestId !== lastFetchId) return payments.value
      const data = response.data?.data
      const page = Array.isArray(data) ? data : []
      payments.value = append ? [...payments.value, ...page] : page
      hasMore.value = Boolean(response.data?.meta?.pagination?.hasMore)
      nextCursor.value = response.data?.meta?.pagination?.nextCursor ?? null
      return payments.value
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

  async function fetchPayment(id: number): Promise<PaymentDetail | null> {
    isFetchingOne.value = true
    error.value = null
    try {
      const response = await api.get<PaymentDetailResponse>(`/api/v1/payments/${id}`)
      currentPayment.value = response.data?.data ?? null
      return currentPayment.value
    } catch (err) {
      currentPayment.value = null
      error.value = err as Error
      return null
    } finally {
      isFetchingOne.value = false
    }
  }

  /**
   * Fetches one order's payment history, newest first. The read is
   * unpaginated server-side — histories are a handful of proofs — so the
   * list replaces unconditionally.
   */
  async function fetchOrderPayments(orderId: number): Promise<Payment[]> {
    isFetchingHistory.value = true
    try {
      const response = await api.get<PaymentListResponse>('/api/v1/payments', {
        params: { orderId: String(orderId) },
      })
      const data = response.data?.data
      orderPayments.value = Array.isArray(data) ? data : []
      return orderPayments.value
    } finally {
      isFetchingHistory.value = false
    }
  }

  async function verify(id: number): Promise<Payment | null> {
    isDeciding.value = true
    try {
      const response = await api.post<PaymentResponse>(`/api/v1/payments/${id}/verify`, {})
      return response.data?.data ?? null
    } finally {
      isDeciding.value = false
    }
  }

  async function reject(id: number): Promise<Payment | null> {
    isDeciding.value = true
    try {
      const response = await api.post<PaymentResponse>(`/api/v1/payments/${id}/reject`, {})
      return response.data?.data ?? null
    } finally {
      isDeciding.value = false
    }
  }

  function clearError(): void {
    error.value = null
  }

  return {
    payments,
    hasMore,
    nextCursor,
    isLoading,
    error,
    currentPayment,
    isFetchingOne,
    isDeciding,
    orderPayments,
    isFetchingHistory,
    fetchPayments,
    fetchPayment,
    fetchOrderPayments,
    verify,
    reject,
    clearError,
  }
})
