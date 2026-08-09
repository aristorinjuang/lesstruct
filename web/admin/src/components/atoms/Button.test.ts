import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Button from './Button.vue'

describe('Button', () => {
  it('renders a button element with default classes', () => {
    const wrapper = mount(Button)

    expect(wrapper.find('button').exists()).toBe(true)
    expect(wrapper.find('button').classes()).toContain('button')
    expect(wrapper.find('button').classes()).toContain('button--primary')
    expect(wrapper.find('button').classes()).toContain('button--medium')
  })

  it('applies the variant class from the variant prop', () => {
    const wrapper = mount(Button, { props: { variant: 'danger' } })

    expect(wrapper.find('button').classes()).toContain('button--danger')
  })

  it('maps the legacy secondary variant to neutral', () => {
    const wrapper = mount(Button, { props: { variant: 'secondary' } })

    expect(wrapper.find('button').classes()).toContain('button--neutral')
    expect(wrapper.find('button').classes()).not.toContain('button--secondary')
  })

  it('applies the size class from the size prop', () => {
    const wrapper = mount(Button, { props: { size: 'small' } })

    expect(wrapper.find('button').classes()).toContain('button--small')
  })

  it('applies the tone class for subtle variant', () => {
    const wrapper = mount(Button, { props: { variant: 'subtle', tone: 'success' } })

    expect(wrapper.find('button').classes()).toContain('button--subtle')
    expect(wrapper.find('button').classes()).toContain('button--tone-success')
  })

  it('applies the tone class for ghost variant', () => {
    const wrapper = mount(Button, { props: { variant: 'ghost', tone: 'danger' } })

    expect(wrapper.find('button').classes()).toContain('button--ghost')
    expect(wrapper.find('button').classes()).toContain('button--tone-danger')
  })

  it('does not apply a tone class for primary variant', () => {
    const wrapper = mount(Button, { props: { variant: 'primary', tone: 'danger' } })

    expect(wrapper.find('button').classes()).not.toContain('button--tone-danger')
  })

  it('applies the full-width class when fullWidth is true', () => {
    const wrapper = mount(Button, { props: { fullWidth: true } })

    expect(wrapper.find('button').classes()).toContain('button--full-width')
  })

  it('renders slot content', () => {
    const wrapper = mount(Button, { slots: { default: 'Save' } })

    expect(wrapper.text()).toContain('Save')
  })

  it('sets the type attribute from the type prop', () => {
    const wrapper = mount(Button, { props: { type: 'submit' } })

    expect(wrapper.find('button').attributes('type')).toBe('submit')
  })

  it('emits click on button click', async () => {
    const wrapper = mount(Button)

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('does not emit click when disabled', async () => {
    const wrapper = mount(Button, { props: { disabled: true } })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('does not emit click when loading', async () => {
    const wrapper = mount(Button, { props: { isLoading: true } })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('sets the disabled attribute when disabled', () => {
    const wrapper = mount(Button, { props: { disabled: true } })

    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
    expect(wrapper.find('button').classes()).toContain('button--disabled')
  })

  it('renders a spinner and disables the button when loading', () => {
    const wrapper = mount(Button, { props: { isLoading: true } })

    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
    expect(wrapper.find('.button__spinner').exists()).toBe(true)
    expect(wrapper.find('button').classes()).toContain('button--disabled')
  })
})
