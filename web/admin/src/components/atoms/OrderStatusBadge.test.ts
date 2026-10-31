import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import OrderStatusBadge from './OrderStatusBadge.vue'

const CASES = [
  { status: 'pending', label: 'Pending', cls: 'order-status-badge--pending' },
  { status: 'paid', label: 'Paid', cls: 'order-status-badge--paid' },
  { status: 'packed', label: 'Packed', cls: 'order-status-badge--packed' },
  { status: 'shipped', label: 'Shipped', cls: 'order-status-badge--shipped' },
  { status: 'delivered', label: 'Delivered', cls: 'order-status-badge--delivered' },
  { status: 'cancelled', label: 'Cancelled', cls: 'order-status-badge--cancelled' },
  { status: 'refunded', label: 'Refunded', cls: 'order-status-badge--refunded' },
  { status: 'expired', label: 'Expired', cls: 'order-status-badge--expired' },
]

describe('OrderStatusBadge', () => {
  it.each(CASES)('renders $label for $status with its own styling', ({ status, label, cls }) => {
    const wrapper = mount(OrderStatusBadge, {
      props: { status },
    })
    expect(wrapper.text()).toBe(label)
    expect(wrapper.classes()).toContain(cls)
  })

  it('falls back to the raw status for unknown values', () => {
    const wrapper = mount(OrderStatusBadge, {
      props: { status: 'mystery' },
    })
    expect(wrapper.text()).toBe('mystery')
    expect(wrapper.classes()).toContain('order-status-badge--unknown')
  })
})
