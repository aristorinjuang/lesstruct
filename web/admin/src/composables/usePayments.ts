import { storeToRefs } from 'pinia'
import { usePaymentsStore } from '@/stores/domain/payments'

/**
 * UX-DR4 mandates a composable; components import this, not the store
 * directly. `storeToRefs` keeps the destructured state reactive; actions are
 * picked directly off the store.
 */
export function usePayments() {
  const store = usePaymentsStore()
  const {
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
  } = storeToRefs(store)

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
    fetchPayments: store.fetchPayments,
    fetchPayment: store.fetchPayment,
    fetchOrderPayments: store.fetchOrderPayments,
    verify: store.verify,
    reject: store.reject,
    clearError: store.clearError,
  }
}
