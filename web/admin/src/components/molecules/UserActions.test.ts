import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserActions from './UserActions.vue'

describe('UserActions', () => {
  describe('Pending user status', () => {
    it('should show Approve, Reject, and Mark as Spam buttons', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending' },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons).toHaveLength(3)
      expect(buttons[0].text()).toBe('Approve')
      expect(buttons[1].text()).toBe('Reject')
      expect(buttons[2].text()).toBe('Mark as Spam')
    })

    it('should emit approve event when Approve button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending' },
      })

      const approveButton = wrapper.findAll('button')[0]
      await approveButton.trigger('click')

      expect(wrapper.emitted('approve')).toBeTruthy()
    })

    it('should emit reject event when Reject button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending' },
      })

      const rejectButton = wrapper.findAll('button')[1]
      await rejectButton.trigger('click')

      expect(wrapper.emitted('reject')).toBeTruthy()
    })

    it('should emit markAsSpam event when Mark as Spam button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending' },
      })

      const spamButton = wrapper.findAll('button')[2]
      await spamButton.trigger('click')

      expect(wrapper.emitted('markAsSpam')).toBeTruthy()
    })
  })

  describe('Active user status', () => {
    it('should show Suspend, Soft Delete, and Edit Profile buttons', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Active' },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons).toHaveLength(3)
      expect(buttons[0].text()).toBe('Suspend')
      expect(buttons[1].text()).toBe('Soft Delete')
      expect(buttons[2].text()).toBe('Edit Profile')
    })

    it('should emit suspend event when Suspend button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Active' },
      })

      const suspendButton = wrapper.findAll('button')[0]
      await suspendButton.trigger('click')

      expect(wrapper.emitted('suspend')).toBeTruthy()
    })

    it('should emit softDelete event when Soft Delete button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Active' },
      })

      const deleteButton = wrapper.findAll('button')[1]
      await deleteButton.trigger('click')

      expect(wrapper.emitted('softDelete')).toBeTruthy()
    })

    it('should emit editProfile event when Edit Profile button is clicked', async () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Active' },
      })

      const editButton = wrapper.findAll('button')[2]
      await editButton.trigger('click')

      expect(wrapper.emitted('editProfile')).toBeTruthy()
    })
  })

  describe('Suspended user status', () => {
    it('should show only Soft Delete button', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Suspended' },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons).toHaveLength(1)
      expect(buttons[0].text()).toBe('Soft Delete')
    })
  })

  describe('SoftDeleted user status', () => {
    it('should show no action buttons', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'SoftDeleted' },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons).toHaveLength(0)
    })
  })

  describe('Accessibility', () => {
    it('should have aria-label attributes on all buttons', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending' },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons[0].attributes('aria-label')).toBe('Approve user')
      expect(buttons[1].attributes('aria-label')).toBe('Reject user')
      expect(buttons[2].attributes('aria-label')).toBe('Mark as spam')
    })
  })

  describe('Disabled state', () => {
    it('should disable all buttons when disabled prop is true', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending', disabled: true },
      })

      const buttons = wrapper.findAll('button')
      expect(buttons).toHaveLength(3)
      buttons.forEach((button) => {
        expect(button.attributes('disabled')).toBeDefined()
      })
    })

    it('should not disable buttons when disabled prop is false', () => {
      const wrapper = mount(UserActions, {
        props: { userStatus: 'Pending', disabled: false },
      })

      const buttons = wrapper.findAll('button')
      buttons.forEach((button) => {
        expect(button.attributes('disabled')).toBeUndefined()
      })
    })
  })
})
