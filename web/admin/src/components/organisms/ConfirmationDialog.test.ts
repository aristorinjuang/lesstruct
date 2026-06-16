import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfirmationDialog from './ConfirmationDialog.vue'

describe('ConfirmationDialog', () => {
  it('should not render when isOpen is false', () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: false,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    expect(wrapper.find('.modal__overlay').exists()).toBe(false)
  })

  it('should render when isOpen is true', () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    expect(wrapper.find('.modal__overlay').exists()).toBe(true)
    expect(wrapper.text()).toContain('Test Title')
    expect(wrapper.text()).toContain('Test message')
  })

  it('should render custom button text when provided', () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
        confirmButtonText: 'Yes, do it',
        cancelButtonText: 'No, cancel',
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons[0].text()).toBe('No, cancel')
    expect(buttons[1].text()).toBe('Yes, do it')
  })

  it('should use default button text when not provided', () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons[0].text()).toBe('Cancel')
    expect(buttons[1].text()).toBe('Confirm')
  })

  it('should emit confirm event when confirm button is clicked', async () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    const confirmButton = wrapper.findAll('button')[1]
    await confirmButton.trigger('click')

    expect(wrapper.emitted('confirm')).toBeTruthy()
  })

  it('should emit cancel event when cancel button is clicked', async () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    const cancelButton = wrapper.findAll('button')[0]
    await cancelButton.trigger('click')

    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('should emit cancel event when modal overlay is clicked', async () => {
    const wrapper = mount(ConfirmationDialog, {
      props: {
        isOpen: true,
        title: 'Test Title',
        message: 'Test message',
      },
    })

    const overlay = wrapper.find('.modal__overlay')
    await overlay.trigger('click')

    expect(wrapper.emitted('cancel')).toBeTruthy()
  })
})
