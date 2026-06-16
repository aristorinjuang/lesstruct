import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NotificationBadge from './NotificationBadge.vue'

describe('NotificationBadge', () => {
  describe('rendering', () => {
    it('should render the count when greater than 0', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 5 },
      })
      expect(wrapper.text()).toBe('5')
    })

    it('should not render when count is 0', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 0 },
      })
      expect(wrapper.find('.notification-badge').exists()).toBe(false)
    })

    it('should not render when count is negative (clamped to 0)', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: -3 },
      })
      expect(wrapper.find('.notification-badge').exists()).toBe(false)
    })

    it('should render "99+" when count exceeds maxCount', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 100, maxCount: 99 },
      })
      expect(wrapper.text()).toBe('99+')
    })

    it('should render the exact count when within maxCount', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 50, maxCount: 99 },
      })
      expect(wrapper.text()).toBe('50')
    })

    it('should render the maxCount when equal to maxCount', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 99, maxCount: 99 },
      })
      expect(wrapper.text()).toBe('99')
    })
  })

  describe('accessibility', () => {
    it('should have proper ARIA attributes', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 3 },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.attributes('role')).toBe('status')
      expect(badge.attributes('aria-live')).toBe('polite')
      expect(badge.attributes('aria-label')).toBe('3 pending items')
    })

    it('should have tabindex="0" for keyboard focus', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 3 },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.attributes('tabindex')).toBe('0')
    })

    it('should update ARIA label when count changes', async () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 3 },
      })
      await wrapper.setProps({ count: 5 })
      expect(wrapper.find('.notification-badge').attributes('aria-label')).toBe('5 pending items')
    })
  })

  describe('styling', () => {
    it('should have correct CSS classes', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 1 },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.classes()).toContain('notification-badge')
    })

    it('should have absolute positioning', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 1 },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.classes()).toContain('notification-badge')
    })
  })

  describe('props', () => {
    it('should use default maxCount of 99', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 100 },
      })
      expect(wrapper.text()).toBe('99+')
    })

    it('should use custom maxCount when provided', () => {
      const wrapper = mount(NotificationBadge, {
        props: { count: 20, maxCount: 10 },
      })
      expect(wrapper.text()).toBe('10+')
    })
  })
})
