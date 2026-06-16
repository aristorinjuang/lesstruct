import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import EditorButton from './EditorButton.vue'

describe('EditorButton', () => {
  it('renders button with correct label', () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold'
      }
    })

    const button = wrapper.find('button')
    expect(button.attributes('aria-label')).toBe('Bold')
    expect(button.attributes('title')).toBe('Bold')
  })

  it('includes keyboard shortcut in title when provided', () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold',
        shortcut: 'Ctrl+B'
      }
    })

    expect(wrapper.find('button').attributes('title')).toBe('Bold (Ctrl+B)')
  })

  it('applies active styling when isActive is true', () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold',
        isActive: true
      }
    })

    expect(wrapper.find('button').classes()).toContain('editor-btn--active')
  })

  it('does not apply active styling when isActive is false', () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold',
        isActive: false
      }
    })

    expect(wrapper.find('button').classes()).not.toContain('editor-btn--active')
  })

  it('disables button when isDisabled is true', () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold',
        isDisabled: true
      }
    })

    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
  })

  it('emits click event when button is clicked', async () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold'
      }
    })

    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('does not emit click when button is disabled', async () => {
    const wrapper = mount(EditorButton, {
      props: {
        icon: 'bold',
        label: 'Bold',
        isDisabled: true
      }
    })

    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('renders correct icon for each icon type', () => {
    const icons = ['bold', 'italic', 'underline', 'list', 'quote', 'code', 'link', 'image', 'undo', 'redo'] as const

    icons.forEach(icon => {
      const wrapper = mount(EditorButton, {
        props: {
          icon,
          label: icon.charAt(0).toUpperCase() + icon.slice(1)
        }
      })

      expect(wrapper.find('svg').exists()).toBe(true)
      expect(wrapper.find('path').exists()).toBe(true)
    })
  })
})
