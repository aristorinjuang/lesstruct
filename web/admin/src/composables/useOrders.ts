import { storeToRefs } from 'pinia'
import { useOrdersStore } from '@/stores/domain/orders'

/**
 * UX-DR4 mandates a composable; components import this, not the store
 * directly. `storeToRefs` keeps the destructured state reactive; actions are
 * picked directly off the store.
 */
export function useOrders() {
  const store = useOrdersStore()
  const {
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
  } = storeToRefs(store)

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
    fetchOrders: store.fetchOrders,
    fetchOrder: store.fetchOrder,
    transition: store.transition,
    setShipping: store.setShipping,
    openInvoice: store.openInvoice,
    clearError: store.clearError,
  }
}
