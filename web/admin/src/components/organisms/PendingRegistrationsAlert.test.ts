import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import PendingRegistrationsAlert from './PendingRegistrationsAlert.vue'

describe('PendingRegistrationsAlert', () => {
  const createRouterWithRoute = () => {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/users', component: { template: '<div>Users</div>' } },
      ],
    })
  }

  const createWrapper = (pendingCount: number) => {
    const router = createRouterWithRoute()

    return mount(PendingRegistrationsAlert, {
      props: { pendingCount },
      global: {
        plugins: [router],
        stubs: {
          RouterLink: {
            template: '<a href="#" class="router-link-stub" v-bind="$attrs"><slot /></a>',
          },
        },
      },
    })
  }

  it('does not render when pendingCount is 0', () => {
    const wrapper = createWrapper(0)

    expect(wrapper.find('.pending-registrations-alert').exists()).toBe(false)
  })

  it('renders alert when pendingCount is greater than 0', () => {
    const wrapper = createWrapper(3)

    expect(wrapper.find('.pending-registrations-alert').exists()).toBe(true)
  })

  it('displays the correct count', () => {
    const wrapper = createWrapper(5)

    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('pending registrations')
  })

  it('uses plural form for multiple registrations', () => {
    const wrapper = createWrapper(3)

    expect(wrapper.text()).toContain('registrations')
  })

  it('uses singular form for single registration', () => {
    const wrapper = createWrapper(1)

    expect(wrapper.text()).toContain('registration')
  })

  it('renders review button with correct route', () => {
    const wrapper = createWrapper(2)

    const button = wrapper.find('.pending-registrations-alert__button')
    expect(button.exists()).toBe(true)
    expect(button.text()).toContain('Review Registrations')
  })

  it('applies warning styling', () => {
    const wrapper = createWrapper(1)

    const alert = wrapper.find('.pending-registrations-alert')
    expect(alert.classes()).toContain('pending-registrations-alert--warning')
  })

  it('has proper accessibility attributes', () => {
    const wrapper = createWrapper(2)

    const alert = wrapper.find('.pending-registrations-alert')
    expect(alert.attributes('role')).toBe('alert')
  })

  it('applies proper CSS classes', () => {
    const wrapper = createWrapper(1)

    expect(wrapper.find('.pending-registrations-alert').exists()).toBe(true)
    expect(wrapper.find('.pending-registrations-alert__content').exists()).toBe(true)
    expect(wrapper.find('.pending-registrations-alert__button').exists()).toBe(true)
  })

  it('displays warning icon', () => {
    const wrapper = createWrapper(1)

    const icon = wrapper.find('.pending-registrations-alert__icon')
    expect(icon.exists()).toBe(true)
  })

  it('has proper link to users page with pending status', () => {
    const wrapper = createWrapper(3)

    const link = wrapper.find('.router-link-stub')
    expect(link.attributes('to')).toBe('/users?status=pending')
  })
})
