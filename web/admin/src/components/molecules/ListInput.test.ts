import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ListInput from './ListInput.vue'

describe('ListInput', () => {
  it('renders one row per item', () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: ['Cotton', 'Machine wash'] }
    })

    expect(wrapper.findAll('.list-input__item')).toHaveLength(2)
    expect(wrapper.text()).toContain('Cotton')
  })

  it('adds a trimmed item from the input', async () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: [] }
    })

    await wrapper.find('input').setValue('  Cotton  ')
    await wrapper.find('.list-input__add-btn').trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['Cotton']])
  })

  it('adds an item on Enter', async () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: [] }
    })

    await wrapper.find('input').setValue('Cotton')
    await wrapper.find('input').trigger('keydown', { key: 'Enter' })

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['Cotton']])
  })

  it('does not add empty items', async () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: [] }
    })

    await wrapper.find('input').setValue('   ')
    await wrapper.find('.list-input__add-btn').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('removes an item', async () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: ['Cotton', 'Wool'] }
    })

    await wrapper.findAll('.list-input__btn--remove')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['Wool']])
  })

  it('reorders an item down', async () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: ['Cotton', 'Wool'] }
    })

    await wrapper.findAll('.list-input__btn[aria-label="Move down"]')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['Wool', 'Cotton']])
  })

  it('disables adding at maxItems', () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: ['Cotton'], maxItems: 1 }
    })

    expect(wrapper.find('.list-input__add-btn').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Maximum of 1 items reached.')
  })

  it('caps the draft input at maxLength', () => {
    const wrapper = mount(ListInput, {
      props: { modelValue: [], maxLength: 10 }
    })

    expect(wrapper.find('input').attributes('maxlength')).toBe('10')
  })
})
