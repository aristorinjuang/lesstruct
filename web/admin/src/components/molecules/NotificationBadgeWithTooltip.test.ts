import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NotificationBadgeWithTooltip from './NotificationBadgeWithTooltip.vue'

describe('NotificationBadgeWithTooltip', () => {
  describe('rendering', () => {
    it('should render NotificationBadge with correct props', () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.exists()).toBe(true)
      expect(badge.text()).toBe('5')
    })

    it('should not render NotificationBadge when count is 0', () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 0,
          tooltipText: 'No pending items',
        },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.exists()).toBe(false)
    })

    it('should render tooltip with correct text on hover', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 3,
          tooltipText: '3 pending user registrations',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('mouseenter')
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(true)
      expect(wrapper.find('.notification-badge-tooltip').text()).toBe('3 pending user registrations')
    })

    it('should not render tooltip when badge is hidden (count 0)', () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 0,
          tooltipText: 'No pending items',
        },
      })
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(false)
    })
  })

  describe('tooltip visibility', () => {
    it('should show tooltip on mouse enter', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('mouseenter')
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(true)
    })

    it('should hide tooltip on mouse leave', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('mouseenter')
      await wrapper.vm.$nextTick()
      await container.trigger('mouseleave')
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(false)
    })

    it('should show tooltip on focusin (keyboard)', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('focusin')
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(true)
    })

    it('should hide tooltip on focusout (keyboard)', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('focusin')
      await wrapper.vm.$nextTick()
      await container.trigger('focusout')
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.notification-badge-tooltip').exists()).toBe(false)
    })
  })

  describe('accessibility', () => {
    it('should have tooltip with role="tooltip"', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      await container.trigger('mouseenter')
      await wrapper.vm.$nextTick()
      const tooltip = wrapper.find('.notification-badge-tooltip')
      expect(tooltip.attributes('role')).toBe('tooltip')
    })

    it('should allow keyboard access via badge tabindex', async () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const badge = wrapper.find('.notification-badge')
      expect(badge.attributes('tabindex')).toBe('0')
    })
  })

  describe('props', () => {
    it('should pass maxCount to NotificationBadge', () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 100,
          maxCount: 50,
          tooltipText: 'Many pending items',
        },
      })
      expect(wrapper.find('.notification-badge').text()).toBe('50+')
    })
  })

  describe('positioning', () => {
    it('should have correct container positioning', () => {
      const wrapper = mount(NotificationBadgeWithTooltip, {
        props: {
          count: 5,
          tooltipText: '5 pending items',
        },
      })
      const container = wrapper.find('.notification-badge-with-tooltip')
      expect(container.classes()).toContain('notification-badge-with-tooltip')
    })
  })
})
