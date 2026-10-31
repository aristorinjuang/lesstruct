<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useConfig } from '@/composables/useConfig'
import { useOrders } from '@/composables/useOrders'
import Toast from '@/components/molecules/Toast.vue'
import Button from '@/components/atoms/Button.vue'
import Select from '@/components/atoms/Select.vue'
import OrderStatusBadge from '@/components/atoms/OrderStatusBadge.vue'
import { formatDateTime } from '@/utils/date'
import { ORDER_STATUSES, type OrderStatus } from '@/types/order'
import { formatMoney } from '@/utils/money'
import type { ApiError } from '@/utils/request'

const { orders, hasMore, nextCursor, isLoading, error, fetchOrders } = useOrders()
const { orders: ordersConfig } = useConfig()

const statusFilter = ref<string>('')
const isMounted = ref(true)

// Toast state (mirrors ApiKeys.vue)
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

function toastApiError(err: unknown, fallback: string) {
  if (isMounted.value) {
    displayToast((err as ApiError)?.message || fallback, 'error')
  }
}

const statusOptions = [
  { value: '', label: 'All statuses' },
  ...ORDER_STATUSES.map((status) => ({
    value: status,
    label: status.charAt(0).toUpperCase() + status.slice(1),
  })),
]

function activeStatus(): OrderStatus | null {
  return statusFilter.value ? (statusFilter.value as OrderStatus) : null
}

async function loadOrders() {
  try {
    await fetchOrders(activeStatus())
  } catch (err) {
    toastApiError(err, 'Failed to load orders')
  }
}

async function loadMore() {
  if (!nextCursor.value) return
  try {
    await fetchOrders(activeStatus(), nextCursor.value, true)
  } catch (err) {
    toastApiError(err, 'Failed to load more orders')
  }
}

watch(statusFilter, () => {
  void loadOrders()
})

onMounted(() => {
  void loadOrders()
})

onUnmounted(() => {
  isMounted.value = false
})
</script>

<template>
  <div class="orders-view">
    <header class="page-header--stacked">
      <h1 class="page-title">Orders</h1>
      <p class="page-subtitle">
        Fulfillment and lifecycle for placed orders. New orders arrive from the public checkout.
      </p>
    </header>

    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <div v-if="isLoading && orders.length === 0" class="orders-view__state">Loading orders…</div>
    <div
      v-else-if="error && orders.length === 0"
      class="orders-view__state orders-view__state--error"
    >
      <p>Failed to load orders.</p>
      <Button type="button" variant="danger" @click="loadOrders">Retry</Button>
    </div>
    <div v-else-if="orders.length === 0" class="orders-view__state">
      <p>No orders yet.</p>
      <p v-if="!ordersConfig" class="orders-view__hint">
        Orders appear here on commerce installs once the public checkout is used.
      </p>
    </div>
    <div v-else class="orders-view__content">
      <div class="orders-view__toolbar">
        <Select
          v-model="statusFilter"
          :options="statusOptions"
          placeholder=""
          class="orders-view__status-filter"
          aria-label="Filter orders by status"
        />
      </div>

      <table class="orders-view__table">
        <thead>
          <tr>
            <th scope="col">Order</th>
            <th scope="col">Status</th>
            <th scope="col">Customer</th>
            <th scope="col">Items</th>
            <th scope="col">Subtotal</th>
            <th scope="col">Placed</th>
            <th scope="col"><span class="visually-hidden">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="order in orders" :key="order.id">
            <td>
              <RouterLink :to="`/orders/${order.id}`" class="orders-view__number">
                {{ order.orderNumber }}
              </RouterLink>
            </td>
            <td><OrderStatusBadge :status="order.status" /></td>
            <td>
              <div class="orders-view__customer">
                <span>{{ order.customerName }}</span>
                <span class="orders-view__muted">{{ order.customerEmail }}</span>
              </div>
            </td>
            <td>{{ order.itemCount ?? order.items.length }}</td>
            <td>{{ formatMoney(order.currency, order.subtotal) }}</td>
            <td>{{ formatDateTime(order.createdAt) }}</td>
            <td>
              <RouterLink :to="`/orders/${order.id}`" class="orders-view__action-link">
                View
              </RouterLink>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="hasMore" class="orders-view__load-more">
        <Button type="button" variant="secondary" :disabled="isLoading" @click="loadMore">
          {{ isLoading ? 'Loading…' : 'Load more' }}
        </Button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.orders-view {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.orders-view__state {
  padding: 2rem;
  border: 1px dashed rgba(148, 163, 184, 0.4);
  border-radius: 8px;
  text-align: center;
  color: var(--color-text-muted);
}

.orders-view__state--error p {
  margin-bottom: 0.75rem;
}

.orders-view__hint {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.orders-view__toolbar {
  display: flex;
  justify-content: flex-end;
}

.orders-view__status-filter {
  max-width: 200px;
}

.orders-view__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.orders-view__table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
  color: var(--color-text-muted);
  font-weight: 600;
}

.orders-view__table td {
  padding: 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.orders-view__number,
.orders-view__action-link {
  font-weight: 600;
  color: var(--color-primary);
  text-decoration: none;
}

.orders-view__number:hover,
.orders-view__action-link:hover {
  text-decoration: underline;
}

.orders-view__customer {
  display: flex;
  flex-direction: column;
}

.orders-view__muted {
  color: var(--color-text-muted);
  font-size: 0.8125rem;
}

.orders-view__load-more {
  display: flex;
  justify-content: center;
  gap: 0.75rem;
  padding: 0.75rem 0;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
</style>
