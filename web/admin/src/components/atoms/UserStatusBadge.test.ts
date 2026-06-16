import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserStatusBadge from './UserStatusBadge.vue'

describe('UserStatusBadge', () => {
  describe('Pending status', () => {
    it('should display "Pending" text', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Pending' },
      })
      expect(wrapper.text()).toBe('Pending')
    })

    it('should have yellow styling for Pending status', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Pending' },
      })
      expect(wrapper.classes()).toContain('user-status-badge--pending')
    })
  })

  describe('Active status', () => {
    it('should display "Active" text', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Active' },
      })
      expect(wrapper.text()).toBe('Active')
    })

    it('should have green styling for Active status', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Active' },
      })
      expect(wrapper.classes()).toContain('user-status-badge--active')
    })
  })

  describe('Suspended status', () => {
    it('should display "Suspended" text', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Suspended' },
      })
      expect(wrapper.text()).toBe('Suspended')
    })

    it('should have orange styling for Suspended status', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Suspended' },
      })
      expect(wrapper.classes()).toContain('user-status-badge--suspended')
    })
  })

  describe('SoftDeleted status', () => {
    it('should display "Soft Deleted" text', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'SoftDeleted' },
      })
      expect(wrapper.text()).toBe('Soft Deleted')
    })

    it('should have gray styling for SoftDeleted status', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'SoftDeleted' },
      })
      expect(wrapper.classes()).toContain('user-status-badge--soft-deleted')
    })
  })

  describe('Accessibility', () => {
    it('should have role="status" attribute', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Active' },
      })
      expect(wrapper.attributes('role')).toBe('status')
    })

    it('should have aria-live="polite" attribute', () => {
      const wrapper = mount(UserStatusBadge, {
        props: { status: 'Active' },
      })
      expect(wrapper.attributes('aria-live')).toBe('polite')
    })
  })
})
