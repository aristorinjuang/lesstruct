import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserRoleBadge from './UserRoleBadge.vue'

describe('UserRoleBadge', () => {
  describe('Admin role', () => {
    it('should display "Admin" text', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Admin' },
      })
      expect(wrapper.text()).toBe('Admin')
    })

    it('should have accent/violet styling for Admin role', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Admin' },
      })
      expect(wrapper.classes()).toContain('user-role-badge--admin')
    })
  })

  describe('Contributor role', () => {
    it('should display "Contributor" text', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Contributor' },
      })
      expect(wrapper.text()).toBe('Contributor')
    })

    it('should have primary styling for Contributor role', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Contributor' },
      })
      expect(wrapper.classes()).toContain('user-role-badge--contributor')
    })
  })

  describe('Commentator role', () => {
    it('should display "Commentator" text', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Commentator' },
      })
      expect(wrapper.text()).toBe('Commentator')
    })

    it('should have blue styling for Commentator role', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Commentator' },
      })
      expect(wrapper.classes()).toContain('user-role-badge--commentator')
    })
  })

  describe('Accessibility', () => {
    it('should have role="status" attribute', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Admin' },
      })
      expect(wrapper.attributes('role')).toBe('status')
    })

    it('should have aria-live="polite" attribute', () => {
      const wrapper = mount(UserRoleBadge, {
        props: { role: 'Admin' },
      })
      expect(wrapper.attributes('aria-live')).toBe('polite')
    })
  })
})
