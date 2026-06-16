import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SwitchToggle from './SwitchToggle.vue'

describe('SwitchToggle', () => {
  it('renders root element with role="switch"', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Enable notifications' }
    })

    expect(wrapper.find('[role="switch"]').exists()).toBe(true)
  })

  it('renders label text from label prop', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Enable notifications' }
    })

    expect(wrapper.text()).toContain('Enable notifications')
  })

  it('sets aria-checked to "true" when modelValue is true', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: true, label: 'Test' }
    })

    expect(wrapper.find('[role="switch"]').attributes('aria-checked')).toBe('true')
  })

  it('sets aria-checked to "false" when modelValue is false', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test' }
    })

    expect(wrapper.find('[role="switch"]').attributes('aria-checked')).toBe('false')
  })

  it('emits update:modelValue with true when clicked from off state', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test' }
    })

    await wrapper.find('[role="switch"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([true])
  })

  it('emits update:modelValue with false when clicked from on state', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: true, label: 'Test' }
    })

    await wrapper.find('[role="switch"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([false])
  })

  it('emits update:modelValue on Space key press', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test' }
    })

    await wrapper.find('[role="switch"]').trigger('keydown.space')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([true])
  })

  it('applies switch-toggle--active class when modelValue is true', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: true, label: 'Test' }
    })

    expect(wrapper.find('[role="switch"]').classes()).toContain('switch-toggle--active')
  })

  it('does not apply switch-toggle--active class when modelValue is false', () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test' }
    })

    expect(wrapper.find('[role="switch"]').classes()).not.toContain('switch-toggle--active')
  })

  it('applies disabled CSS class and does not emit on click when disabled', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test', disabled: true }
    })

    expect(wrapper.find('[role="switch"]').classes()).toContain('switch-toggle--disabled')

    await wrapper.find('[role="switch"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('does not emit on Space key press when disabled', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test', disabled: true }
    })

    await wrapper.find('[role="switch"]').trigger('keydown.space')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('updates aria-checked reactively when modelValue prop changes', async () => {
    const wrapper = mount(SwitchToggle, {
      props: { modelValue: false, label: 'Test' }
    })

    expect(wrapper.find('[role="switch"]').attributes('aria-checked')).toBe('false')

    await wrapper.setProps({ modelValue: true })
    expect(wrapper.find('[role="switch"]').attributes('aria-checked')).toBe('true')
  })
})
