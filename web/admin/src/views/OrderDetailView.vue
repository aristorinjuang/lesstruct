<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useOrders } from '@/composables/useOrders'
import { usePayments } from '@/composables/usePayments'
import Toast from '@/components/molecules/Toast.vue'
import Button from '@/components/atoms/Button.vue'
import OrderStatusBadge from '@/components/atoms/OrderStatusBadge.vue'
import PaymentStatusBadge from '@/components/atoms/PaymentStatusBadge.vue'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'
import { formatDateTime } from '@/utils/date'
import { ALLOWED_TRANSITIONS, CONFIRMED_TRANSITIONS, type OrderStatus } from '@/types/order'
import { formatMoney } from '@/utils/money'
import { ApiError } from '@/utils/request'

const route = useRoute()
const {
  currentOrder: order,
  isFetchingOne,
  isTransitioning,
  isSavingShipping,
  isFetchingInvoice,
  error: storeError,
  fetchOrder,
  transition,
  setShipping,
  openInvoice,
} = useOrders()
const {
  orderPayments: payments,
  isFetchingHistory,
  fetchOrderPayments,
} = usePayments()

const notFound = ref(false)
const failed = ref(false)

// The status in flight, for button disabled state and copy.
const pendingTarget = ref<OrderStatus | null>(null)

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

const availableTransitions = computed<readonly OrderStatus[]>(() => {
  if (!order.value) return []
  return ALLOWED_TRANSITIONS[order.value.status] ?? []
})

// Destructive targets ask first; the copy names what the move does.
const confirmTarget = ref<OrderStatus | null>(null)

const confirmCopy = computed(() => {
  const target = confirmTarget.value
  if (target === 'refunded') {
    return 'Refunding marks this order refunded. Only the payment reconciliation outside this screen can return the money. Continue?'
  }
  if (target === 'expired') {
    return 'Expiring closes this unpaid order permanently — payment can no longer be recorded against it. Continue?'
  }
  return 'Cancelling closes this order permanently and returns nothing to the customer automatically. Continue?'
})

const confirmLabel = computed(() => {
  const target = confirmTarget.value
  return target ? target.charAt(0).toUpperCase() + target.slice(1) : ''
})

const transitionLabel: Partial<Record<OrderStatus, string>> = {
  paid: 'Mark paid',
  packed: 'Mark packed',
  shipped: 'Mark shipped',
  delivered: 'Mark delivered',
  cancelled: 'Cancel order',
  refunded: 'Refund',
  expired: 'Expire',
}

async function loadOrder() {
  const id = Number(route.params.id)
  if (!Number.isFinite(id)) {
    notFound.value = true
    return
  }
  const found = await fetchOrder(id)
  notFound.value = false
  failed.value = false
  if (found) {
    void fetchOrderPayments(id)
    return
  }
  notFound.value = (storeError.value as ApiError | null)?.statusCode === 404
  failed.value = !notFound.value
}

function targetCopy(target: OrderStatus): string {
  const labels: Partial<Record<OrderStatus, string>> = {
    paid: 'paid',
    packed: 'packed',
    shipped: 'shipped',
    delivered: 'delivered',
    cancelled: 'cancelled',
    refunded: 'refunded',
    expired: 'expired',
  }
  return labels[target] ?? target
}

async function applyTransition(target: OrderStatus) {
  if (!order.value) return
  pendingTarget.value = target
  try {
    const updated = await transition(order.value.id, target)
    if (updated) {
      displayToast(`Order marked ${targetCopy(target)}`)
    }
  } catch (err) {
    displayToast((err as ApiError)?.message || 'Failed to update order', 'error')
  } finally {
    pendingTarget.value = null
    confirmTarget.value = null
  }
}

function onTransitionClick(target: OrderStatus) {
  if (CONFIRMED_TRANSITIONS.has(target)) {
    confirmTarget.value = target
    return
  }
  void applyTransition(target)
}

function onConfirm() {
  if (confirmTarget.value) {
    void applyTransition(confirmTarget.value)
  }
}

function onCancelConfirm() {
  confirmTarget.value = null
}

// Shipping editor (pending-gated): weigh the parcel, then charge it. Past
// pending the input disables; 4xx surfaces as a toast.
const shippingAmount = ref('')
const isInvoiceUnavailable = ref(false)

async function loadShippingDraft() {
  if (order.value) {
    shippingAmount.value = String(order.value.shippingMinor ?? 0)
  }
}

async function applyShipping() {
  if (!order.value) return
  // v-model on a number input assigns numbers at runtime (Vue auto-casts
  // type="number"), so normalize before validating: an emptied field must
  // not silently write free shipping.
  const raw = shippingAmount.value as unknown as string | number
  if (raw === '' || (typeof raw === 'number' && Number.isNaN(raw))) {
    displayToast('Shipping must be a non-negative whole number', 'error')
    return
  }
  const amount = Number(raw)
  if (!Number.isInteger(amount) || amount < 0) {
    displayToast('Shipping must be a non-negative whole number', 'error')
    return
  }
  try {
    const updated = await setShipping(order.value.id, amount)
    if (updated) {
      displayToast('Shipping updated')
    }
  } catch (err) {
    displayToast((err as ApiError)?.message || 'Failed to update shipping', 'error')
  }
}

async function openInvoiceTab() {
  if (!order.value) return
  // Open the tab synchronously in the click gesture: awaiting the fetch
  // first would lose transient activation and trip popup blockers.
  const tab = window.open('', '_blank', 'noopener')
  try {
    const url = await openInvoice(order.value.id)
    isInvoiceUnavailable.value = false
    if (!url) {
      if (tab && !tab.closed) tab.close()
      return
    }
    if (tab && !tab.closed) {
      tab.location.href = url
    } else {
      // Popup blocked: a programmatic anchor click needs no gesture.
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.target = '_blank'
      anchor.rel = 'noopener'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
    }
  } catch (err) {
    if (tab && !tab.closed) tab.close()
    const apiErr = err as ApiError
    if (apiErr?.statusCode === 404 || apiErr?.code === 'NOT_FOUND') {
      isInvoiceUnavailable.value = true
      return
    }
    if (apiErr?.statusCode === 503 || apiErr?.code === 'INVOICE_UNAVAILABLE') {
      isInvoiceUnavailable.value = true
      displayToast('Invoice rendering is not configured', 'error')
      return
    }
    displayToast(apiErr?.message || 'Failed to load invoice', 'error')
  }
}

onMounted(() => {
  void loadOrder().then(() => void loadShippingDraft())
})
</script>

<template>
  <div class="order-detail-view">
    <header class="page-header--stacked">
      <RouterLink to="/orders" class="order-detail-view__back">← All orders</RouterLink>
      <h1 v-if="order" class="page-title order-detail-view__title">
        {{ order.orderNumber }}
        <OrderStatusBadge :status="order.status" />
      </h1>
      <h1 v-else class="page-title">Order</h1>
      <p v-if="order" class="page-subtitle">
        Placed {{ formatDateTime(order.createdAt) }} · Last moved
        {{ formatDateTime(order.updatedAt) }}
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

    <div v-if="isFetchingOne && !order" class="order-detail-view__state">Loading order…</div>
    <div v-else-if="notFound" class="order-detail-view__state">
      <p>Order not found. It may have been created on a different install.</p>
      <RouterLink to="/orders" class="order-detail-view__back-link">Back to orders</RouterLink>
    </div>
    <div v-else-if="failed" class="order-detail-view__state">
      <p>Failed to load the order.</p>
      <Button type="button" variant="danger" @click="loadOrder">Retry</Button>
    </div>

    <template v-else-if="order">
      <div class="order-detail-view__grid">
        <section class="order-detail-view__card">
          <h2 class="card-title">Customer</h2>
          <dl class="order-detail-view__meta">
            <div>
              <dt>Name</dt>
              <dd>{{ order.customerName }}</dd>
            </div>
            <div>
              <dt>Email</dt>
              <dd>{{ order.customerEmail }}</dd>
            </div>
            <div v-if="order.customerPhone">
              <dt>Phone</dt>
              <dd>{{ order.customerPhone }}</dd>
            </div>
          </dl>
        </section>

        <section class="order-detail-view__card">
          <h2 class="card-title">Ship to</h2>
          <address class="order-detail-view__address">
            {{ order.shipTo.street }}<br />
            {{ order.shipTo.city }}, {{ order.shipTo.province }} {{ order.shipTo.postalCode }}
          </address>
        </section>
      </div>

      <section class="order-detail-view__card">
        <h2 class="card-title">Items</h2>
        <table class="order-detail-view__table">
          <thead>
            <tr>
              <th scope="col">Product</th>
              <th scope="col">SKU</th>
              <th scope="col">Qty</th>
              <th scope="col">Unit price</th>
              <th scope="col">Line total</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in order.items" :key="item.id">
              <td>{{ item.title }}</td>
              <td>
                <code v-if="item.sku">{{ item.sku }}</code
                ><span v-else>—</span>
              </td>
              <td>{{ item.quantity }}</td>
              <td>{{ formatMoney(order.currency, item.unitPrice) }}</td>
              <td>{{ formatMoney(order.currency, item.lineTotal) }}</td>
            </tr>
          </tbody>
          <tfoot>
            <tr>
              <td colspan="4">Subtotal</td>
              <td>{{ formatMoney(order.currency, order.subtotal) }}</td>
            </tr>
            <tr>
              <td colspan="4">Shipping</td>
              <td>{{ formatMoney(order.currency, order.shippingMinor) }}</td>
            </tr>
            <tr>
              <td colspan="4">Total</td>
              <td>{{ formatMoney(order.currency, order.totalMinor) }}</td>
            </tr>
          </tfoot>
        </table>
      </section>

      <section class="order-detail-view__card">
        <h2 class="card-title">Payments</h2>
        <div v-if="isFetchingHistory && payments.length === 0" class="order-detail-view__state">
          Loading payments…
        </div>
        <p v-else-if="payments.length === 0" class="order-detail-view__muted">
          No payment proofs yet — the customer submits one against the order
          number and checkout email.
        </p>
        <table v-else class="order-detail-view__table">
          <thead>
            <tr>
              <th scope="col">Payment</th>
              <th scope="col">Status</th>
              <th scope="col">Method</th>
              <th scope="col">Reference</th>
              <th scope="col">Amount</th>
              <th scope="col">Decided by</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="payment in payments" :key="payment.id">
              <td>#{{ payment.id }}</td>
              <td><PaymentStatusBadge :status="payment.status" /></td>
              <td>{{ payment.method }}</td>
              <td>
                <code>{{ payment.reference }}</code>
              </td>
              <td>{{ formatMoney(order.currency, payment.amount) }}</td>
              <td>{{ payment.verifiedBy || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="order-detail-view__card">
        <h2 class="card-title">Shipping</h2>
        <p class="order-detail-view__muted">
          Weigh the parcel, then charge it. Only a pending order carries a shipping line.
        </p>
        <div class="order-detail-view__shipping">
          <input
            v-model="shippingAmount"
            type="number"
            min="0"
            step="1"
            class="order-detail-view__shipping-input"
            aria-label="Shipping amount in minor units"
            :disabled="order.status !== 'pending' || isSavingShipping"
          />
          <Button
            type="button"
            variant="primary"
            :disabled="order.status !== 'pending' || isSavingShipping"
            @click="applyShipping"
          >
            {{ isSavingShipping ? 'Saving…' : 'Set shipping' }}
          </Button>
        </div>
      </section>

      <section class="order-detail-view__card">
        <h2 class="card-title">Invoice</h2>
        <p class="order-detail-view__muted">Derived on demand from the stored record.</p>
        <div class="order-detail-view__actions">
          <Button
            v-if="!isInvoiceUnavailable"
            type="button"
            variant="secondary"
            :disabled="isFetchingInvoice"
            @click="openInvoiceTab"
          >
            {{ isFetchingInvoice ? 'Loading…' : 'Open invoice (PDF)' }}
          </Button>
          <p v-else class="order-detail-view__muted">Invoice is not available for this order.</p>
        </div>
      </section>

      <section class="order-detail-view__card">
        <h2 class="card-title">Actions</h2>
        <p v-if="isTransitioning" class="order-detail-view__busy">Updating order…</p>
        <div v-if="availableTransitions.length === 0" class="order-detail-view__terminal">
          This order is {{ targetCopy(order.status) }} — a closed status with no further moves.
        </div>
        <div v-else class="order-detail-view__actions">
          <Button
            v-for="target in availableTransitions"
            :key="target"
            type="button"
            :variant="CONFIRMED_TRANSITIONS.has(target) ? 'secondary' : 'primary'"
            :disabled="isTransitioning"
            @click="onTransitionClick(target)"
          >
            {{ pendingTarget === target ? 'Updating…' : (transitionLabel[target] ?? target) }}
          </Button>
        </div>
      </section>
    </template>

    <ConfirmationDialog
      :is-open="confirmTarget !== null"
      :title="confirmLabel ? `${confirmLabel} order` : 'Confirm move'"
      :message="confirmCopy"
      :confirm-button-text="confirmLabel"
      cancel-button-text="Keep order"
      @confirm="onConfirm"
      @cancel="onCancelConfirm"
    />
  </div>
</template>

<style scoped>
.order-detail-view {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.order-detail-view__back {
  color: var(--color-text-muted);
  text-decoration: none;
  font-size: 0.875rem;
}

.order-detail-view__back:hover,
.order-detail-view__back-link:hover {
  text-decoration: underline;
}

.order-detail-view__back-link {
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 600;
}

.order-detail-view__title {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.order-detail-view__state {
  padding: 2rem;
  border: 1px dashed rgba(148, 163, 184, 0.4);
  border-radius: 8px;
  text-align: center;
  color: var(--color-text-muted);
}

.order-detail-view__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 1rem;
}

.order-detail-view__card {
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 8px;
  padding: 1rem 1.25rem;
}

.card-title {
  font-size: 0.9375rem;
  font-weight: 600;
  margin-bottom: 0.75rem;
}

.order-detail-view__meta {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0;
}

.order-detail-view__meta dt {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-muted);
}

.order-detail-view__meta dd {
  margin: 0;
  font-size: 0.9375rem;
}

.order-detail-view__address {
  font-style: normal;
  line-height: 1.5;
}

.order-detail-view__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.order-detail-view__table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
  color: var(--color-text-muted);
  font-weight: 600;
}

.order-detail-view__table td {
  padding: 0.625rem 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.order-detail-view__table tfoot td {
  font-weight: 600;
  border-bottom: none;
}

.order-detail-view__table tfoot td:last-child {
  text-align: right;
}

.order-detail-view__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.order-detail-view__shipping {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin-top: 0.75rem;
}

.order-detail-view__shipping-input {
  max-width: 200px;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 6px;
  font-size: 0.875rem;
}

.order-detail-view__busy {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.order-detail-view__terminal {
  color: var(--color-text-muted);
  font-size: 0.9375rem;
}

.order-detail-view__muted {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
