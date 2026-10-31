import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { mount, RouterLinkStub } from '@vue/test-utils'
import OrderDetailView from './OrderDetailView.vue'
import { ALLOWED_TRANSITIONS, ORDER_STATUSES } from '@/types/order'
import type { Order, OrderStatus } from '@/types/order'
import type { Payment } from '@/types/payment'

let orderIdParam = '42'

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => ({ params: { id: orderIdParam } }),
  }
})

const currentOrderRef = ref<Order | null>(null)
const isFetchingOneRef = ref(false)
const isTransitioningRef = ref(false)
const isSavingShippingRef = ref(false)
const isFetchingInvoiceRef = ref(false)
const storeErrorRef = ref<Error | null>(null)
const fetchOrderMock = vi.fn()
const transitionMock = vi.fn()
const setShippingMock = vi.fn()
const openInvoiceMock = vi.fn()

vi.mock('@/composables/useOrders', () => ({
  useOrders: () => ({
    currentOrder: currentOrderRef,
    isFetchingOne: isFetchingOneRef,
    isTransitioning: isTransitioningRef,
    isSavingShipping: isSavingShippingRef,
    isFetchingInvoice: isFetchingInvoiceRef,
    error: storeErrorRef,
    fetchOrder: fetchOrderMock,
    transition: transitionMock,
    setShipping: setShippingMock,
    openInvoice: openInvoiceMock,
  }),
}))

const orderPaymentsRef = ref<Payment[]>([])
const isFetchingHistoryRef = ref(false)
const fetchOrderPaymentsMock = vi.fn()

vi.mock('@/composables/usePayments', () => ({
  usePayments: () => ({
    orderPayments: orderPaymentsRef,
    isFetchingHistory: isFetchingHistoryRef,
    fetchOrderPayments: fetchOrderPaymentsMock,
  }),
}))

function buildOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 42,
    orderNumber: 'ORD-000042',
    status: 'pending',
    customerEmail: 'customer@example.com',
    customerName: 'Customer Name',
    customerPhone: '+62 812 000 000',
    shipTo: {
      street: 'Jl. Example No. 1',
      city: 'Jakarta',
      province: 'DKI Jakarta',
      postalCode: '10110',
    },
    currency: 'IDR',
    subtotal: 299000,
    shippingMinor: 0,
    totalMinor: 299000,
    items: [
      {
        id: 1,
        productContentId: 7,
        variantId: null,
        sku: 'SHIRT-001',
        title: 'T-Shirt',
        unitPrice: 299000,
        quantity: 1,
        lineTotal: 299000,
      },
    ],
    itemCount: 1,
    createdAt: '2026-10-18T00:00:00Z',
    updatedAt: '2026-10-18T00:00:00Z',
    ...overrides,
  }
}

function mountView() {
  return mount(OrderDetailView, {
    global: {
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

/** Sets the order a mount sees both on the ref and from the fetch mock — the
 * real store sets currentOrder inside fetchOrder. */
function seedOrder(order: Order) {
  currentOrderRef.value = order
  fetchOrderMock.mockResolvedValue(order)
}

function findButton(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(label))
}

describe('OrderDetailView', () => {
  beforeEach(() => {
    currentOrderRef.value = null
    isFetchingOneRef.value = false
    isTransitioningRef.value = false
    isSavingShippingRef.value = false
    isFetchingInvoiceRef.value = false
    storeErrorRef.value = null
    orderPaymentsRef.value = []
    isFetchingHistoryRef.value = false
    orderIdParam = '42'
    vi.clearAllMocks()
    fetchOrderMock.mockResolvedValue(null)
    transitionMock.mockResolvedValue(null)
    setShippingMock.mockResolvedValue(null)
    openInvoiceMock.mockResolvedValue('blob:mock-pdf')
    fetchOrderPaymentsMock.mockResolvedValue([])
  })

  it('renders the loading state while fetching', () => {
    isFetchingOneRef.value = true

    const wrapper = mountView()

    expect(wrapper.text()).toContain('Loading order…')
  })

  it('renders the not-found state when the order does not resolve', async () => {
    storeErrorRef.value = Object.assign(new Error('Not found'), {
      statusCode: 404,
      code: 'NOT_FOUND',
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Order not found')
  })

  it('renders the retry state when the order fetch fails for other reasons', async () => {
    storeErrorRef.value = Object.assign(new Error('Connection reset'), {
      statusCode: 500,
      code: 'INTERNAL',
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to load the order.')
    expect(wrapper.text()).toContain('Retry')
  })

  it('renders the not-found state for a non-numeric route param', async () => {
    orderIdParam = 'abc'

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Order not found')
  })

  it('renders the customer, ship-to, items, and subtotal', async () => {
    const order = buildOrder()
    seedOrder(order)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('ORD-000042')
    expect(wrapper.text()).toContain('Customer Name')
    expect(wrapper.text()).toContain('customer@example.com')
    expect(wrapper.text()).toContain('+62 812 000 000')
    expect(wrapper.text()).toContain('Jakarta')
    expect(wrapper.text()).toContain('T-Shirt')
    expect(wrapper.text()).toContain('SHIRT-001')
    expect(wrapper.text()).toContain('IDR 299,000')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
  })

  it('renders shipping and total lines in the items footer', async () => {
    const order = buildOrder({ shippingMinor: 15000, totalMinor: 314000 })
    seedOrder(order)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Shipping')
    expect(wrapper.text()).toContain('IDR 15,000')
    expect(wrapper.text()).toContain('Total')
    expect(wrapper.text()).toContain('IDR 314,000')
  })

  it('fetches and renders the order payment history', async () => {
    const order = buildOrder()
    seedOrder(order)
    orderPaymentsRef.value = [
      {
        id: 7,
        orderId: 42,
        method: 'manual',
        status: 'pending',
        reference: 'BR-12345',
        amount: 299000,
        createdAt: '2026-10-18T00:00:00Z',
        updatedAt: '2026-10-18T00:00:00Z',
      } satisfies Payment,
    ]

    const wrapper = mountView()
    await flushPromises()

    expect(fetchOrderPaymentsMock).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('Payments')
    expect(wrapper.text()).toContain('BR-12345')
    expect(wrapper.text()).toContain('manual')
  })

  it('renders the empty payment history hint', async () => {
    const order = buildOrder()
    seedOrder(order)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('No payment proofs yet')
  })

  it('offers the pending move set without a paid target', async () => {
    const order = buildOrder()
    seedOrder(order)

    const wrapper = mountView()
    await flushPromises()

    const labels = wrapper.findAll('button').map((button) => button.text())
    expect(labels).not.toContain('Mark paid')
    expect(labels).toContain('Cancel order')
    expect(labels).toContain('Expire')
  })

  it('applies progress moves directly without confirmation', async () => {
    const order = buildOrder({ status: 'paid' })
    const packed = buildOrder({ status: 'packed' })
    seedOrder(order)
    transitionMock.mockResolvedValue(packed)

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Mark packed')!.trigger('click')
    await flushPromises()

    expect(transitionMock).toHaveBeenCalledWith(42, 'packed')
    expect(wrapper.text()).toContain('Order marked packed')
  })

  it('asks for confirmation before destructive moves', async () => {
    const order = buildOrder()
    seedOrder(order)
    transitionMock.mockResolvedValue(buildOrder({ status: 'cancelled' }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.confirmation-dialog-stub').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ConfirmationDialog' }).props('isOpen')).toBe(false)
    expect(transitionMock).not.toHaveBeenCalled()

    await findButton(wrapper, 'Cancel order')!.trigger('click')
    await flushPromises()

    const dialog = wrapper.findComponent({ name: 'ConfirmationDialog' })
    expect(dialog.props('isOpen')).toBe(true)
    expect(dialog.props('confirmButtonText')).toBe('Cancelled')
    expect(transitionMock).not.toHaveBeenCalled()

    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    expect(transitionMock).toHaveBeenCalledWith(42, 'cancelled')
  })

  it('surfaces the transition error from the API', async () => {
    const order = buildOrder()
    seedOrder(order)
    const err = Object.assign(new Error('Invalid transition'), {
      statusCode: 409,
      code: 'CONFLICT',
    })
    transitionMock.mockRejectedValue(err)

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Cancel order')!.trigger('click')
    await flushPromises()

    const dialog = wrapper.findComponent({ name: 'ConfirmationDialog' })
    expect(dialog.props('isOpen')).toBe(true)

    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    expect(transitionMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Invalid transition')
  })

  it('shows a closed-status note when no outgoing moves exist', async () => {
    const order = buildOrder({ status: 'expired' })
    seedOrder(order)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('.order-detail-view__terminal').exists()).toBe(true)
    expect(wrapper.text()).toContain('closed status')
    expect(findButton(wrapper, 'Cancel order')).toBeUndefined()
    expect(findButton(wrapper, 'Expire')).toBeUndefined()
  })

  it('disables the action buttons while a transition is in flight', async () => {
    const order = buildOrder()
    seedOrder(order)
    isTransitioningRef.value = true

    const wrapper = mountView()
    await flushPromises()

    const cancelButton = findButton(wrapper, 'Cancel order')
    const expireButton = findButton(wrapper, 'Expire')
    expect(cancelButton).toBeDefined()
    expect(expireButton).toBeDefined()
    expect(cancelButton!.attributes('disabled')).toBeDefined()
    expect(expireButton!.attributes('disabled')).toBeDefined()
  })

  it('covers every status with the matrix contract', () => {
    // The matrix must never strand an admin on a non-terminal status, and
    // every terminal one leaves no outgoing moves (mirrors the domain
    // lifecycle in modules/orders/domain/status.go).
    const nonTerminal: OrderStatus[] = ['pending', 'paid', 'packed', 'shipped', 'delivered']
    const terminal = ORDER_STATUSES.filter((status) => !nonTerminal.includes(status))
    expect(Object.keys(ALLOWED_TRANSITIONS)).toEqual([...ORDER_STATUSES])
    for (const status of nonTerminal) {
      expect(ALLOWED_TRANSITIONS[status].length).toBeGreaterThan(0)
    }
    expect(terminal.map((status) => ALLOWED_TRANSITIONS[status])).toEqual([[], [], []])
  })

  it('never offers paid as a transition target', () => {
    // Payment confirmation is the payments module's proof-gated path
    // (manual verify or gateway webhook); no admin surface mints a
    // payment (the domain refuses it outright).
    for (const status of ORDER_STATUSES) {
      expect(ALLOWED_TRANSITIONS[status]).not.toContain('paid')
    }
  })

  it('submits the shipping amount for a pending order', async () => {
    seedOrder(buildOrder({ status: 'pending', shippingMinor: 0 }))
    setShippingMock.mockResolvedValue({ ...buildOrder({ status: 'pending' }), shippingMinor: 15000 })

    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.find('.order-detail-view__shipping-input')
    expect(input.exists()).toBe(true)
    await input.setValue('15000')
    await findButton(wrapper, 'Set shipping')!.trigger('click')
    await flushPromises()

    expect(setShippingMock).toHaveBeenCalledWith(42, 15000)
  })

  it('shows a validation error for a negative shipping amount', async () => {
    seedOrder(buildOrder({ status: 'pending', shippingMinor: 0 }))

    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.find('.order-detail-view__shipping-input')
    await input.setValue('-5')
    await findButton(wrapper, 'Set shipping')!.trigger('click')
    await flushPromises()

    expect(setShippingMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('non-negative whole number')
  })

  it('surfaces a server-side shipping error as a toast', async () => {
    seedOrder(buildOrder({ status: 'pending', shippingMinor: 0 }))
    setShippingMock.mockRejectedValueOnce(
      Object.assign(new Error('Shipping exceeds the parcel limit'), { statusCode: 400 })
    )

    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.find('.order-detail-view__shipping-input')
    await input.setValue('999999')
    await findButton(wrapper, 'Set shipping')!.trigger('click')
    await flushPromises()

    expect(setShippingMock).toHaveBeenCalledWith(42, 999999)
    expect(wrapper.text()).toContain('Shipping exceeds the parcel limit')
  })

  it('rejects an emptied shipping field instead of writing free shipping', async () => {
    seedOrder(buildOrder({ status: 'pending', shippingMinor: 15000 }))

    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.find('.order-detail-view__shipping-input')
    await input.setValue('')
    await findButton(wrapper, 'Set shipping')!.trigger('click')
    await flushPromises()

    expect(setShippingMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('non-negative whole number')
  })

  it('disables the shipping editor once the order leaves pending', async () => {
    seedOrder(buildOrder({ status: 'shipped', shippingMinor: 15000 }))

    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.find('.order-detail-view__shipping-input')
    expect((input.element as HTMLInputElement).disabled).toBe(true)
    expect(findButton(wrapper, 'Set shipping')!.attributes('disabled')).toBeDefined()
  })

  it('opens the invoice in a new tab on demand', async () => {
    seedOrder(buildOrder({ status: 'paid' }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Open invoice')!.trigger('click')
    await flushPromises()

    expect(openInvoiceMock).toHaveBeenCalledWith(42)
  })

  it('hides the invoice action when rendering is unavailable', async () => {
    seedOrder(buildOrder({ status: 'paid' }))
    openInvoiceMock.mockRejectedValueOnce(Object.assign(new Error('gone'), { statusCode: 404 }))

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Open invoice')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Invoice is not available for this order')
    expect(findButton(wrapper, 'Open invoice')).toBeUndefined()
  })

  it('explains an unconfigured invoice renderer instead of failing silently', async () => {
    seedOrder(buildOrder({ status: 'paid' }))
    openInvoiceMock.mockRejectedValueOnce(
      Object.assign(new Error('no renderer'), { statusCode: 503, code: 'INVOICE_UNAVAILABLE' })
    )

    const wrapper = mountView()
    await flushPromises()

    await findButton(wrapper, 'Open invoice')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Invoice rendering is not configured')
    expect(findButton(wrapper, 'Open invoice')).toBeUndefined()
  })
})
