<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useConfig } from '@/composables/useConfig'
import { usePayments } from '@/composables/usePayments'
import Toast from '@/components/molecules/Toast.vue'
import Button from '@/components/atoms/Button.vue'
import Select from '@/components/atoms/Select.vue'
import PaymentStatusBadge from '@/components/atoms/PaymentStatusBadge.vue'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'
import { formatDateTime } from '@/utils/date'
import { formatMoney } from '@/utils/money'
import { PAYMENT_STATUSES, type Payment, type PaymentStatus } from '@/types/payment'
import type { ApiError } from '@/utils/request'

const {
  payments,
  hasMore,
  nextCursor,
  isLoading,
  error,
  isDeciding,
  fetchPayments,
  verify,
  reject,
} = usePayments()
const { orders: ordersConfig, payments: paymentsConfig } = useConfig()

// Proofs carry minor units and no currency of their own; the order currency
// is the display currency for the whole commerce install.
function formatAmount(amount: number): string {
  return formatMoney(ordersConfig.value?.currency ?? '', amount)
}

const statusFilter = ref<string>('')
const isMounted = ref(true)
const decidingId = ref<number | null>(null)

// Both decisions move money, so both ask first.
const confirmAction = ref<'verify' | 'reject' | null>(null)
const confirmPayment = ref<Payment | null>(null)

const confirmCopy = computed(() => {
  if (!confirmPayment.value) return ''
  if (confirmAction.value === 'verify') {
    return `Verifying confirms order ${confirmPayment.value.orderId} as paid and claims its stock. Continue?`
  }
  return `Rejecting keeps the order pending — the customer may correct and resubmit. Continue?`
})

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
  ...PAYMENT_STATUSES.map((status) => ({
    value: status,
    label: status.charAt(0).toUpperCase() + status.slice(1),
  })),
]

function activeStatus(): PaymentStatus | null {
  return statusFilter.value ? (statusFilter.value as PaymentStatus) : null
}

async function loadPayments() {
  try {
    await fetchPayments(activeStatus())
  } catch (err) {
    toastApiError(err, 'Failed to load payments')
  }
}

async function loadMore() {
  if (!nextCursor.value) return
  try {
    await fetchPayments(activeStatus(), nextCursor.value, true)
  } catch (err) {
    toastApiError(err, 'Failed to load more payments')
  }
}

function onDecisionClick(payment: Payment, action: 'verify' | 'reject') {
  confirmPayment.value = payment
  confirmAction.value = action
}

function onCancelConfirm() {
  confirmPayment.value = null
  confirmAction.value = null
}

async function onConfirm() {
  const payment = confirmPayment.value
  const action = confirmAction.value
  if (!payment || !action) return
  decidingId.value = payment.id
  try {
    if (action === 'verify') {
      await verify(payment.id)
      displayToast(`Payment verified — order ${payment.orderId} marked paid`)
    } else {
      await reject(payment.id)
      displayToast('Payment rejected — the order stays pending')
    }
    await loadPayments()
  } catch (err) {
    toastApiError(err, 'Failed to decide the payment')
  } finally {
    decidingId.value = null
    onCancelConfirm()
  }
}

watch(statusFilter, () => {
  void loadPayments()
})

onMounted(() => {
  void loadPayments()
})

onUnmounted(() => {
  isMounted.value = false
})
</script>

<template>
  <div class="payments-view">
    <header class="page-header--stacked">
      <h1 class="page-title">Payments</h1>
      <p class="page-subtitle">
        Verify manual transfer proofs or follow up gateway settlements. Verifying
        confirms the order as paid and claims its stock.
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

    <div v-if="isLoading && payments.length === 0" class="payments-view__state">
      Loading payments…
    </div>
    <div
      v-else-if="error && payments.length === 0"
      class="payments-view__state payments-view__state--error"
    >
      <p>Failed to load payments.</p>
      <Button type="button" variant="danger" @click="loadPayments">Retry</Button>
    </div>
    <div v-else-if="payments.length === 0" class="payments-view__state">
      <p>No payments yet.</p>
      <p v-if="!paymentsConfig" class="payments-view__hint">
        Payments appear here on commerce installs once guests submit transfer
        proofs or check out through a gateway.
      </p>
    </div>
    <div v-else class="payments-view__content">
      <div class="payments-view__toolbar">
        <Select
          v-model="statusFilter"
          :options="statusOptions"
          placeholder=""
          class="payments-view__status-filter"
          aria-label="Filter payments by status"
        />
      </div>

      <table class="payments-view__table">
        <thead>
          <tr>
            <th scope="col">Payment</th>
            <th scope="col">Order</th>
            <th scope="col">Status</th>
            <th scope="col">Method</th>
            <th scope="col">Reference</th>
            <th scope="col">Amount</th>
            <th scope="col">Submitted</th>
            <th scope="col"><span class="visually-hidden">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="payment in payments" :key="payment.id">
            <td>#{{ payment.id }}</td>
            <td>
              <RouterLink :to="`/orders/${payment.orderId}`" class="payments-view__number">
                Order {{ payment.orderId }}
              </RouterLink>
            </td>
            <td><PaymentStatusBadge :status="payment.status" /></td>
            <td>{{ payment.method }}</td>
            <td>
              <code>{{ payment.reference }}</code>
            </td>
            <td>{{ formatAmount(payment.amount) }}</td>
            <td>{{ formatDateTime(payment.createdAt) }}</td>
            <td>
              <div v-if="payment.status === 'pending'" class="payments-view__decisions">
                <Button
                  type="button"
                  variant="primary"
                  :disabled="isDeciding"
                  @click="onDecisionClick(payment, 'verify')"
                >
                  {{ decidingId === payment.id ? 'Deciding…' : 'Verify' }}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  :disabled="isDeciding"
                  @click="onDecisionClick(payment, 'reject')"
                >
                  Reject
                </Button>
              </div>
              <span v-else class="payments-view__decided">
                {{ payment.verifiedBy ? `by ${payment.verifiedBy}` : 'decided' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="hasMore" class="payments-view__load-more">
        <Button type="button" variant="secondary" :disabled="isLoading" @click="loadMore">
          {{ isLoading ? 'Loading…' : 'Load more' }}
        </Button>
      </div>
    </div>

    <ConfirmationDialog
      :is-open="confirmAction !== null"
      :title="confirmAction === 'verify' ? 'Verify payment' : 'Reject payment'"
      :message="confirmCopy"
      :confirm-button-text="confirmAction === 'verify' ? 'Verify' : 'Reject'"
      cancel-button-text="Keep pending"
      @confirm="onConfirm"
      @cancel="onCancelConfirm"
    />
  </div>
</template>

<style scoped>
.payments-view {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.payments-view__state {
  padding: 2rem;
  border: 1px dashed rgba(148, 163, 184, 0.4);
  border-radius: 8px;
  text-align: center;
  color: var(--color-text-muted);
}

.payments-view__state--error p {
  margin-bottom: 0.75rem;
}

.payments-view__hint {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.payments-view__toolbar {
  display: flex;
  justify-content: flex-end;
}

.payments-view__status-filter {
  max-width: 200px;
}

.payments-view__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.payments-view__table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.3);
  color: var(--color-text-muted);
  font-weight: 600;
}

.payments-view__table td {
  padding: 0.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.payments-view__number {
  font-weight: 600;
  color: var(--color-primary);
  text-decoration: none;
}

.payments-view__number:hover {
  text-decoration: underline;
}

.payments-view__decisions {
  display: flex;
  gap: 0.5rem;
}

.payments-view__decided {
  color: var(--color-text-muted);
  font-size: 0.8125rem;
}

.payments-view__load-more {
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
