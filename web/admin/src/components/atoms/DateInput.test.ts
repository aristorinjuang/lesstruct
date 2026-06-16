import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DateInput from './DateInput.vue'

describe('DateInput', () => {
  it('renders native input type="date"', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '' }
    })

    const input = wrapper.find('input')
    expect(input.exists()).toBe(true)
    expect(input.attributes('type')).toBe('date')
  })

  it('displays modelValue as the input value', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '2026-05-09' }
    })

    expect(wrapper.find('input').element.value).toBe('2026-05-09')
  })

  it('emits update:modelValue on input change', async () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '' }
    })

    const input = wrapper.find('input')
    await input.setValue('2026-12-25')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['2026-12-25'])
  })

  it('updates displayed value when modelValue prop changes', async () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '' }
    })

    expect(wrapper.find('input').element.value).toBe('')

    await wrapper.setProps({ modelValue: '2026-12-25' })
    expect(wrapper.find('input').element.value).toBe('2026-12-25')
  })

  it('applies disabled attribute when disabled prop is true', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '', disabled: true }
    })

    expect(wrapper.find('input').attributes('disabled')).toBeDefined()
  })

  it('applies date-input CSS class', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '' }
    })

    expect(wrapper.find('input').classes()).toContain('date-input')
  })

  it('applies date-input--disabled class when disabled', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '', disabled: true }
    })

    expect(wrapper.find('input').classes()).toContain('date-input--disabled')
  })

  it('does not apply date-input--disabled class when enabled', () => {
    const wrapper = mount(DateInput, {
      props: { modelValue: '', disabled: false }
    })

    expect(wrapper.find('input').classes()).not.toContain('date-input--disabled')
  })
})
