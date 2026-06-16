import { storeToRefs } from 'pinia'
import { useApiKeysStore } from '@/stores/domain/apiKeys'

/**
 * UX-DR4 mandates a composable; components import this, not the store directly.
 *
 * `storeToRefs` keeps the returned state reactive when destructured by the
 * consumer (a plain property pass-through of setup-store state would snapshot
 * the value and lose reactivity). Actions are picked directly off the store.
 */
export function useApiKeys() {
  const store = useApiKeysStore()
  const { keys, isLoading, error, isCreating, isRevoking } = storeToRefs(store)
  return {
    keys,
    isLoading,
    error,
    isCreating,
    isRevoking,
    fetchKeys: store.fetchKeys,
    create: store.create,
    revoke: store.revoke,
    clearError: store.clearError,
  }
}
