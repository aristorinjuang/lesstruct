import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import InputText from './InputText.vue'

function createWrapper(props = {}) {
  return mount(InputText, {
    props: { modelValue: '', ...props }
  })
}

describe('InputText', () => {
  it('renders an input element by default', () => {
    const wrapper = createWrapper()

    expect(wrapper.find('input').exists()).toBe(true)
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('displays modelValue as the input value', () => {
    const wrapper = createWrapper({ modelValue: 'hello' })

    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('hello')
  })

  it('emits update:modelValue on input', async () => {
    const wrapper = createWrapper()

    await wrapper.find('input').setValue('test')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['test'])
  })

  it('applies disabled attribute', () => {
    const wrapper = createWrapper({ disabled: true })

    expect(wrapper.find('input').attributes('disabled')).toBeDefined()
  })

  it('applies size CSS class', () => {
    const wrapper = createWrapper({ size: 'large' })

    expect(wrapper.find('input').classes()).toContain('input-text--large')
  })

  it('applies placeholder attribute', () => {
    const wrapper = createWrapper({ placeholder: 'Enter text...' })

    expect(wrapper.find('input').attributes('placeholder')).toBe('Enter text...')
  })

  describe('multiline mode', () => {
    it('renders a textarea when multiline is true', () => {
      const wrapper = createWrapper({ multiline: true })

      expect(wrapper.find('textarea').exists()).toBe(true)
      expect(wrapper.find('input').exists()).toBe(false)
    })

    it('emits update:modelValue on textarea input', async () => {
      const wrapper = createWrapper({ multiline: true })

      await wrapper.find('textarea').setValue('multiline text')

      expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
      expect(wrapper.emitted('update:modelValue')![0]).toEqual(['multiline text'])
    })

    it('applies disabled attribute to textarea', () => {
      const wrapper = createWrapper({ multiline: true, disabled: true })

      expect(wrapper.find('textarea').attributes('disabled')).toBeDefined()
    })

    it('applies maxlength attribute to textarea when maxLength is provided', () => {
      const wrapper = createWrapper({ multiline: true, maxLength: 500 })

      expect(wrapper.find('textarea').attributes('maxlength')).toBe('500')
    })

    it('displays character counter when maxLength is provided', () => {
      const wrapper = createWrapper({ modelValue: 'hello', multiline: true, maxLength: 100 })

      const counter = wrapper.find('.input-text__counter')
      expect(counter.exists()).toBe(true)
      expect(counter.text()).toBe('5/100')
    })

    it('displays character counter for non-multiline input when maxLength is provided', () => {
      const wrapper = createWrapper({ modelValue: 'ab', maxLength: 50 })

      const counter = wrapper.find('.input-text__counter')
      expect(counter.exists()).toBe(true)
      expect(counter.text()).toBe('2/50')
    })

    it('does not display character counter when maxLength is not provided', () => {
      const wrapper = createWrapper({ multiline: true })

      expect(wrapper.find('.input-text__counter').exists()).toBe(false)
    })

    it('shares identical size classes with input mode', () => {
      const wrapper = createWrapper({ multiline: true, size: 'small' })

      expect(wrapper.find('textarea').classes()).toContain('input-text--small')
    })

    it('updates character counter reactively when modelValue changes', async () => {
      const wrapper = createWrapper({ modelValue: 'hello', multiline: true, maxLength: 100 })

      expect(wrapper.find('.input-text__counter').text()).toBe('5/100')

      await wrapper.setProps({ modelValue: 'hello world!' })
      expect(wrapper.find('.input-text__counter').text()).toBe('12/100')
    })
  })
})
