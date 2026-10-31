import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { Order, OrderListMeta, OrderStatus } from '@/types/order'

interface OrderListResponse {
  data: Order[]
  meta?: OrderListMeta
}

interface OrderResponse {
  data: Order
}

export const useOrdersStore = defineStore('orders', () => {
  const orders = ref<Order[]>([])
  const hasMore = ref(false)
  const nextCursor = ref<string | null>(null)
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  const currentOrder = ref<Order | null>(null)
  const isFetchingOne = ref(false)
  const isTransitioning = ref(false)
  const isSavingShipping = ref(false)
  const isFetchingInvoice = ref(false)

  // Monotonic token used to ignore stale list responses — e.g. when a newer
  // fetch supersedes an older one, or the view unmounts mid-request.
  let lastFetchId = 0

  /**
   * Fetches the newest-first admin order list. The status filter narrows the
   * page; pass a cursor to key off the previous page. `append` is the
   * load-more path: the fetched page was produced with the previous page's
   * cursor, so it joins the list instead of replacing it.
   */
  async function fetchOrders(
    status: OrderStatus | null = null,
    cursor: string | null = null,
    append = false,
  ): Promise<Order[]> {
    isLoading.value = true
    error.value = null
    const requestId = ++lastFetchId
    try {
      const params: Record<string, string> = {}
      if (status) params.status = status
      if (cursor) params.cursor = cursor
      const response = await api.get<OrderListResponse>('/api/v1/orders', { params })
      if (requestId !== lastFetchId) return orders.value
      const data = response.data?.data
      const page = Array.isArray(data) ? data : []
      orders.value = append ? [...orders.value, ...page] : page
      hasMore.value = Boolean(response.data?.meta?.pagination?.hasMore)
      nextCursor.value = response.data?.meta?.pagination?.nextCursor ?? null
      return orders.value
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

  async function fetchOrder(id: number): Promise<Order | null> {
    isFetchingOne.value = true
    error.value = null
    try {
      const response = await api.get<OrderResponse>(`/api/v1/orders/${id}`)
      currentOrder.value = response.data?.data ?? null
      return currentOrder.value
    } catch (err) {
      currentOrder.value = null
      error.value = err as Error
      return null
    } finally {
      isFetchingOne.value = false
    }
  }

  async function transition(id: number, status: OrderStatus): Promise<Order | null> {
    isTransitioning.value = true
    try {
      const response = await api.post<OrderResponse>(`/api/v1/orders/${id}/status`, { status })
      currentOrder.value = response.data?.data ?? null
      return currentOrder.value
    } finally {
      isTransitioning.value = false
    }
  }

  async function setShipping(id: number, amount: number): Promise<Order | null> {
    isSavingShipping.value = true
    try {
      const response = await api.post<OrderResponse>(`/api/v1/orders/${id}/shipping`, { amount })
      currentOrder.value = response.data?.data ?? null
      const idx = orders.value.findIndex((o) => o.id === id)
      if (idx >= 0 && currentOrder.value) {
        orders.value[idx] = currentOrder.value
      }
      return currentOrder.value
    } finally {
      isSavingShipping.value = false
    }
  }

  async function openInvoice(id: number): Promise<string> {
    isFetchingInvoice.value = true
    try {
      const headers: Record<string, string> = { Accept: 'application/pdf' }
      const base = import.meta.env.VITE_API_BASE_URL || ''
      const response = await fetch(`${base}/api/v1/orders/${id}/invoice`, {
        headers,
        credentials: 'include',
      })
      if (!response.ok) {
        let code = ''
        let message = response.statusText
        try {
          const body = await response.clone().json()
          if (body?.error?.code) code = body.error.code
          if (body?.error?.message) message = body.error.message
        } catch {
          /* non-JSON error body (e.g. empty 404): keep status text */
        }
        const err = new Error(message || 'Failed to load invoice') as Error & {
          statusCode?: number
          code?: string
        }
        err.statusCode = response.status
        if (code) err.code = code
        throw err
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      setTimeout(() => URL.revokeObjectURL(url), 60_000)
      return url
    } finally {
      isFetchingInvoice.value = false
    }
  }

  function clearError(): void {
    error.value = null
  }

  return {
    orders,
    hasMore,
    nextCursor,
    isLoading,
    error,
    currentOrder,
    isFetchingOne,
    isTransitioning,
    isSavingShipping,
    isFetchingInvoice,
    fetchOrders,
    fetchOrder,
    transition,
    setShipping,
    openInvoice,
    clearError,
  }
})
