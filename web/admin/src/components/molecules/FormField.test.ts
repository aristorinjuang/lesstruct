import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FormField from './FormField.vue'

describe('FormField', () => {
  it('renders label when provided', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', id: 'field-title' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__label').text()).toBe('Title')
  })

  it('does not render label when not provided', () => {
    const wrapper = mount(FormField, {
      props: {},
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__label').exists()).toBe(false)
  })

  it('renders required asterisk when required is true', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', required: true },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__required').exists()).toBe(true)
    expect(wrapper.find('.form-field__required').text()).toBe('*')
  })

  it('does not render required asterisk when required is false', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', required: false },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__required').exists()).toBe(false)
  })

  it('renders error message when provided', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', error: 'This field is required' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__error').exists()).toBe(true)
    expect(wrapper.find('.form-field__error').text()).toBe('This field is required')
  })

  it('does not render error when not provided', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__error').exists()).toBe(false)
  })

  it('sets aria-invalid when error is present', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', error: 'Error' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field').attributes('aria-invalid')).toBe('true')
  })

  it('does not set aria-invalid when no error', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field').attributes('aria-invalid')).toBeUndefined()
  })

  it('renders labelSuffix when provided', () => {
    const wrapper = mount(FormField, {
      props: { label: 'SKU', labelSuffix: 'System' },
      slots: { default: '<input type="text" />' },
    })

    const suffix = wrapper.find('.form-field__label-suffix')
    expect(suffix.exists()).toBe(true)
    expect(suffix.text()).toBe('System')
  })

  it('does not render labelSuffix when undefined', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__label-suffix').exists()).toBe(false)
  })

  it('does not render labelSuffix when empty string', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', labelSuffix: '' },
      slots: { default: '<input type="text" />' },
    })

    expect(wrapper.find('.form-field__label-suffix').exists()).toBe(false)
  })
})
