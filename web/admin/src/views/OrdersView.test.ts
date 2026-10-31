import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import OrdersView from './OrdersView.vue'
import type { Order } from '@/types/order'

const ordersRef = ref<Order[]>([])
const hasMoreRef = ref(false)
const nextCursorRef = ref<string | null>(null)
const isLoadingRef = ref(false)
const errorRef = ref<Error | null>(null)
const fetchOrdersMock = vi.fn()

vi.mock('@/composables/useOrders', () => ({
  useOrders: () => ({
    orders: ordersRef,
    hasMore: hasMoreRef,
    nextCursor: nextCursorRef,
    isLoading: isLoadingRef,
    error: errorRef,
    fetchOrders: fetchOrdersMock,
  }),
}))

const ordersConfigRef = ref<{ currency: string } | null>(null)
vi.mock('@/composables/useConfig', () => ({
  useConfig: () => ({
    orders: ordersConfigRef,
  }),
}))

function buildOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 42,
    orderNumber: 'ORD-000042',
    status: 'pending',
    customerEmail: 'customer@example.com',
    customerName: 'Customer Name',
    shipTo: { street: 'Jl. Example', city: 'Jakarta', province: 'DKI', postalCode: '10110' },
    currency: 'IDR',
    subtotal: 299000,
    shippingMinor: 0,
    totalMinor: 299000,
    items: [
      {
        id: 1,
        productContentId: 7,
        variantId: null,
        sku: 'T-1',
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

let router: Router

function mountView() {
  return mount(OrdersView, {
    global: {
      plugins: [router],
      stubs: {
        RouterLink: RouterLinkStub,
      },
    },
  })
}

function flushPromises(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0))
}

describe('OrdersView', () => {
  beforeEach(() => {
    ordersRef.value = []
    hasMoreRef.value = false
    nextCursorRef.value = null
    isLoadingRef.value = false
    errorRef.value = null
    ordersConfigRef.value = null
    vi.clearAllMocks()
    fetchOrdersMock.mockResolvedValue([])

    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/orders', component: { template: '<div />' } },
        { path: '/orders/:id', component: { template: '<div />' } },
      ],
    })
  })

  it('renders the loading state while fetching', () => {
    isLoadingRef.value = true

    const wrapper = mountView()

    expect(wrapper.text()).toContain('Loading orders…')
  })

  it('renders the empty state with the commerce hint on a non-commerce install', () => {
    const wrapper = mountView()

    expect(wrapper.text()).toContain('No orders yet.')
    expect(wrapper.text()).toContain('commerce installs')
  })

  it('renders the empty state without the commerce hint on a commerce install', () => {
    ordersConfigRef.value = { currency: 'IDR' }

    const wrapper = mountView()

    expect(wrapper.text()).toContain('No orders yet.')
    expect(wrapper.text()).not.toContain('commerce installs')
  })

  it('fetches orders on mount', async () => {
    mountView()
    await flushPromises()

    expect(fetchOrdersMock).toHaveBeenCalledWith(null)
  })

  it('renders the order table with money formatted from each order currency', () => {
    ordersRef.value = [
      buildOrder(),
      buildOrder({ id: 43, orderNumber: 'ORD-000043', status: 'paid' }),
    ]

    const wrapper = mountView()

    expect(wrapper.text()).toContain('ORD-000042')
    expect(wrapper.text()).toContain('ORD-000043')
    expect(wrapper.text()).toContain('customer@example.com')
    expect(wrapper.text()).toContain('IDR 299,000')
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
  })

  it('renders the read-time item count instead of the loaded items length', () => {
    ordersRef.value = [
      buildOrder({ items: [], itemCount: 3 }),
    ]

    const wrapper = mountView()
    const cells = wrapper.findAll('tbody tr td').map((td) => td.text())

    expect(cells).toContain('3')
    expect(cells).not.toContain('0')
  })

  it('falls back to the loaded items length when itemCount is absent', () => {
    const legacy = buildOrder() as unknown as Record<string, unknown>
    delete legacy.itemCount
    ordersRef.value = [legacy as unknown as Order]

    const wrapper = mountView()
    const cells = wrapper.findAll('tbody tr td').map((td) => td.text())

    expect(cells).toContain('1')
  })

  it('links each row to the detail route', () => {
    ordersRef.value = [buildOrder()]

    const wrapper = mountView()
    const links = wrapper.findAllComponents(RouterLinkStub)

    expect(links).toHaveLength(2)
    expect(links[0].props('to')).toBe('/orders/42')
    expect(links[1].props('to')).toBe('/orders/42')
  })

  it('re-fetches with a status when the filter changes', async () => {
    ordersRef.value = [buildOrder()]
    const wrapper = mountView()
    await flushPromises()

    fetchOrdersMock.mockClear()
    await wrapper.find('select').setValue('paid')
    await flushPromises()

    expect(fetchOrdersMock).toHaveBeenCalledWith('paid')
  })

  it('shows the load-more button and fetches the next page appending', async () => {
    ordersRef.value = [buildOrder()]
    hasMoreRef.value = true
    nextCursorRef.value = '42'

    const wrapper = mountView()
    await flushPromises()

    const loadMoreButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Load more'))
    expect(loadMoreButton).toBeDefined()

    fetchOrdersMock.mockClear()
    await loadMoreButton!.trigger('click')
    await flushPromises()

    expect(fetchOrdersMock).toHaveBeenCalledWith(null, '42', true)
  })

  it('hides the load-more button when the cursor is exhausted', () => {
    ordersRef.value = [buildOrder()]

    const wrapper = mountView()

    const loadMoreButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Load more'))
    expect(loadMoreButton).toBeUndefined()
  })

  it('renders the error state with a retry button before any orders load', () => {
    errorRef.value = new Error('Boom')

    const wrapper = mountView()

    expect(wrapper.text()).toContain('Failed to load orders.')
    expect(wrapper.text()).toContain('Retry')
  })
})
