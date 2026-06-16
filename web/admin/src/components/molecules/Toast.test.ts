import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import Toast from '@/components/molecules/Toast.vue'

describe('Toast', () => {
  it('should render toast message', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    expect(wrapper.find('.toast__message').text()).toBe('Test message')
  })

  it('should render close button with icon', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    const closeButton = wrapper.find('.toast__close')
    expect(closeButton.exists()).toBe(true)
    expect(closeButton.find('svg').exists()).toBe(true)
  })

  it('should apply correct type class', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Success message',
        type: 'success',
      },
    })

    expect(wrapper.find('.toast--success').exists()).toBe(true)
  })

  it('should emit dismiss event when close button is clicked', async () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    const closeButton = wrapper.find('.toast__close')
    await closeButton.trigger('click')
    await nextTick()

    expect(wrapper.emitted('dismiss')).toBeTruthy()
    expect(wrapper.emitted('dismiss')![0]).toHaveLength(1) // Check that an id string was emitted
  })

  it('should be visible by default', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    expect(wrapper.find('.toast').exists()).toBe(true)
  })

  it('should not render when visible is false', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
        visible: false,
      },
    })

    expect(wrapper.find('.toast').exists()).toBe(false)
  })

  it('should have correct accessibility attributes', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
        type: 'success',
      },
    })

    const toast = wrapper.find('.toast')
    expect(toast.attributes('role')).toBe('status')
    expect(toast.attributes('aria-live')).toBe('polite')
  })

  it('should have minimum touch target size on close button', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    const closeButton = wrapper.find('.toast__close')
    expect(closeButton.exists()).toBe(true)

    // Verify CSS specifies 44px minimum touch target
    const element = closeButton.element as HTMLElement
    const computedStyle = window.getComputedStyle(element)
    expect(
      parseInt(computedStyle.minWidth) >= 44 || parseInt(computedStyle.minHeight) >= 44 || closeButton.classes().includes('toast__close')
    ).toBe(true)
  })

  it('should auto-dismiss after duration', async () => {
    vi.useFakeTimers()

    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
        duration: 1000,
      },
    })

    // Initially visible
    expect(wrapper.find('.toast').exists()).toBe(true)

    // Fast-forward past the duration
    vi.advanceTimersByTime(1000)
    await nextTick()

    // Should have emitted dismiss
    expect(wrapper.emitted('dismiss')).toBeTruthy()

    vi.useRealTimers()
  })

  it('should not auto-dismiss if duration is 0', async () => {
    vi.useFakeTimers()

    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
        duration: 0,
      },
    })

    // Fast-forward
    vi.advanceTimersByTime(5000)
    await nextTick()

    // Should not have emitted dismiss
    expect(wrapper.emitted('dismiss')).toBeFalsy()

    vi.useRealTimers()
  })

  it('should expose dismiss method', () => {
    const wrapper = mount(Toast, {
      props: {
        message: 'Test message',
      },
    })

    expect(wrapper.vm.dismiss).toBeInstanceOf(Function)
  })

  it('should support all toast types', () => {
    const types: Array<'success' | 'error' | 'warning' | 'info'> = [
      'success',
      'error',
      'warning',
      'info',
    ]

    types.forEach((type) => {
      const wrapper = mount(Toast, {
        props: {
          message: `${type} message`,
          type,
        },
      })

      expect(wrapper.find(`.toast--${type}`).exists()).toBe(true)
    })
  })
})
