import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MultiSelect from './MultiSelect.vue'

describe('MultiSelect', () => {
  it('renders one chip per option', () => {
    const wrapper = mount(MultiSelect, {
      props: { modelValue: [], options: ['S', 'M', 'L'] }
    })

    const chips = wrapper.findAll('button')
    expect(chips).toHaveLength(3)
    expect(chips.map((c) => c.text())).toEqual(['S', 'M', 'L'])
  })

  it('marks selected options as pressed', () => {
    const wrapper = mount(MultiSelect, {
      props: { modelValue: ['M'], options: ['S', 'M', 'L'] }
    })

    const pressed = wrapper.findAll('button').filter((c) => c.attributes('aria-pressed') === 'true')
    expect(pressed).toHaveLength(1)
    expect(pressed[0].text()).toBe('M')
  })

  it('emits the array with the option added on toggle', async () => {
    const wrapper = mount(MultiSelect, {
      props: { modelValue: ['S'], options: ['S', 'M', 'L'] }
    })

    await wrapper.findAll('button')[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['S', 'M']])
  })

  it('emits the array with the option removed on untoggle', async () => {
    const wrapper = mount(MultiSelect, {
      props: { modelValue: ['S', 'M'], options: ['S', 'M', 'L'] }
    })

    await wrapper.findAll('button')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([['M']])
  })

  it('shows a hint when no options are configured', () => {
    const wrapper = mount(MultiSelect, {
      props: { modelValue: [], options: [] }
    })

    expect(wrapper.text()).toContain('No options configured')
    expect(wrapper.findAll('button')).toHaveLength(0)
  })
})
