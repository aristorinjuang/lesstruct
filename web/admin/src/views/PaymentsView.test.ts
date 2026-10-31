import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import PaymentsView from './PaymentsView.vue'
import type { Payment } from '@/types/payment'

const paymentsRef = ref<Payment[]>([])
const hasMoreRef = ref(false)
const nextCursorRef = ref<string | null>(null)
const isLoadingRef = ref(false)
const errorRef = ref<Error | null>(null)
const isDecidingRef = ref(false)
const fetchPaymentsMock = vi.fn()
const verifyMock = vi.fn()
const rejectMock = vi.fn()

vi.mock('@/composables/usePayments', () => ({
  usePayments: () => ({
    payments: paymentsRef,
    hasMore: hasMoreRef,
    nextCursor: nextCursorRef,
    isLoading: isLoadingRef,
    error: errorRef,
    isDeciding: isDecidingRef,
    fetchPayments: fetchPaymentsMock,
    verify: verifyMock,
    reject: rejectMock,
  }),
}))

const ordersConfigRef = ref<{ currency: string } | null>(null)
const paymentsConfigRef = ref<{ methods: string[]; gateway: boolean } | null>(null)
vi.mock('@/composables/useConfig', () => ({
  useConfig: () => ({
    orders: ordersConfigRef,
    payments: paymentsConfigRef,
  }),
}))

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

let router: Router

function mountView() {
  return mount(PaymentsView, {
    global: {
      plugins: [router],
      stubs: {
        RouterLink: RouterLinkStub,
        ConfirmationDialog: {
          name: 'ConfirmationDialog',
          template:
            '<div class="confirmation-dialog-stub"><button class="confirm-stub" @click="$emit(\'confirm\')">confirm-stub</button></div>',
          props: ['isOpen', 'title', 'message', 'confirmButtonText', 'cancelButtonText'],
        },
      },
    },
  })
}

function flushPromises(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0))
}

function findButton(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(label))
}

describe('PaymentsView', () => {
  beforeEach(() => {
    paymentsRef.value = []
    hasMoreRef.value = false
    nextCursorRef.value = null
    isLoadingRef.value = false
    errorRef.value = null
    isDecidingRef.value = false
    ordersConfigRef.value = { currency: 'IDR' }
    paymentsConfigRef.value = null
    vi.clearAllMocks()
    fetchPaymentsMock.mockResolvedValue([])

    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/payments', component: { template: '<div />' } },
        { path: '/orders/:id', component: { template: '<div />' } },
      ],
    })
  })

  it('renders the loading state while fetching', () => {
    isLoadingRef.value = true

    const wrapper = mountView()

    expect(wrapper.text()).toContain('Loading payments…')
  })

  it('renders the empty state with the commerce hint on a non-commerce install', () => {
    const wrapper = mountView()

    expect(wrapper.text()).toContain('No payments yet.')
    expect(wrapper.text()).toContain('commerce installs')
  })

  it('fetches payments on mount', async () => {
    mountView()
    await flushPromises()

    expect(fetchPaymentsMock).toHaveBeenCalledWith(null)
  })

  it('renders the payment table with the proof and its order link', () => {
    paymentsRef.value = [buildPayment()]

    const wrapper = mountView()

    expect(wrapper.text()).toContain('BR-12345')
    expect(wrapper.text()).toContain('manual')
    expect(wrapper.text()).toContain('IDR 522,500')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links[0]?.props('to')).toBe('/orders/42')
  })

  it('offers verify and reject on pending proofs only', () => {
    paymentsRef.value = [
      buildPayment(),
      buildPayment({ id: 8, status: 'verified', verifiedBy: 'admin' }),
    ]

    const wrapper = mountView()

    expect(findButton(wrapper, 'Verify')).toBeDefined()
    expect(findButton(wrapper, 'Reject')).toBeDefined()
    expect(wrapper.text()).toContain('by admin')
  })

  it('verifies through the confirmation dialog and reloads', async () => {
    paymentsRef.value = [buildPayment()]
    verifyMock.mockResolvedValue(buildPayment({ status: 'verified' }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Verify')!.trigger('click')
    await flushPromises()

    expect(wrapper.find('.confirmation-dialog-stub').exists()).toBe(true)
    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    expect(verifyMock).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('marked paid')
    expect(fetchPaymentsMock).toHaveBeenCalledTimes(2)
  })

  it('rejects through the confirmation dialog and reloads', async () => {
    paymentsRef.value = [buildPayment()]
    rejectMock.mockResolvedValue(buildPayment({ status: 'rejected' }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Reject')!.trigger('click')
    await flushPromises()

    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    expect(rejectMock).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('stays pending')
    expect(fetchPaymentsMock).toHaveBeenCalledTimes(2)
  })

  it('surfaces decision failures', async () => {
    paymentsRef.value = [buildPayment()]
    verifyMock.mockRejectedValue(new Error('Order no longer pending'))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Verify')!.trigger('click')
    await flushPromises()

    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Order no longer pending')
  })

  it('re-fetches with a status when the filter changes', async () => {
    paymentsRef.value = [buildPayment()]
    const wrapper = mountView()
    await flushPromises()

    fetchPaymentsMock.mockClear()
    await wrapper.find('select').setValue('verified')
    await flushPromises()

    expect(fetchPaymentsMock).toHaveBeenCalledWith('verified')
  })

  it('renders the error state with a retry button before any payments load', async () => {
    errorRef.value = new Error('boom')
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to load payments.')
    await findButton(wrapper, 'Retry')!.trigger('click')
    await flushPromises()

    expect(fetchPaymentsMock).toHaveBeenCalled()
  })
})
