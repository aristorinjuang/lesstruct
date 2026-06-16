import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NumberInput from './NumberInput.vue'

function createWrapper(props = {}) {
  return mount(NumberInput, {
    props: { modelValue: null, ...props }
  })
}

describe('NumberInput', () => {
  it('renders input with type="number" and inputmode="numeric"', () => {
    const wrapper = createWrapper()

    const input = wrapper.find('input')
    expect(input.exists()).toBe(true)
    expect(input.attributes('type')).toBe('number')
    expect(input.attributes('inputmode')).toBe('numeric')
  })

  it('displays modelValue as the input value', () => {
    const wrapper = createWrapper({ modelValue: 42 })

    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('42')
  })

  it('emits update:modelValue on direct input change', async () => {
    const wrapper = createWrapper({ modelValue: null })

    const input = wrapper.find('input')
    await input.setValue('25')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([25])
  })

  it('emits null when input is cleared', async () => {
    const wrapper = createWrapper({ modelValue: 10 })

    const input = wrapper.find('input')
    await input.setValue('')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([null])
  })

  it('increment button emits value increased by step', async () => {
    const wrapper = createWrapper({ modelValue: 5 })

    const buttons = wrapper.findAll('button')
    const incrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Increase value')!
    await incrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([6])
  })

  it('increment button clamps to max when defined', async () => {
    const wrapper = createWrapper({ modelValue: 9, max: 10 })

    const buttons = wrapper.findAll('button')
    const incrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Increase value')!
    await incrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([10])
  })

  it('increment button does not exceed max', async () => {
    const wrapper = createWrapper({ modelValue: 10, max: 10 })

    const buttons = wrapper.findAll('button')
    const incrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Increase value')!

    expect(incrementBtn.attributes('disabled')).toBeDefined()
  })

  it('decrement button emits value decreased by step', async () => {
    const wrapper = createWrapper({ modelValue: 5 })

    const buttons = wrapper.findAll('button')
    const decrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Decrease value')!
    await decrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([4])
  })

  it('decrement button clamps to min when defined', async () => {
    const wrapper = createWrapper({ modelValue: 1, min: 0 })

    const buttons = wrapper.findAll('button')
    const decrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Decrease value')!
    await decrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([0])
  })

  it('decrement button does not go below min', async () => {
    const wrapper = createWrapper({ modelValue: 0, min: 0 })

    const buttons = wrapper.findAll('button')
    const decrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Decrease value')!

    expect(decrementBtn.attributes('disabled')).toBeDefined()
  })

  it('Arrow Up key increments value', async () => {
    const wrapper = createWrapper({ modelValue: 5 })

    await wrapper.find('input').trigger('keydown.arrow-up')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([6])
  })

  it('Arrow Down key decrements value', async () => {
    const wrapper = createWrapper({ modelValue: 5 })

    await wrapper.find('input').trigger('keydown.arrow-down')

    expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([4])
  })

  it('Arrow keys respect min/max bounds', async () => {
    const wrapper = createWrapper({ modelValue: 10, min: 0, max: 10 })

    await wrapper.find('input').trigger('keydown.arrow-up')
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([10])

    await wrapper.setProps({ modelValue: 0 })
    await wrapper.find('input').trigger('keydown.arrow-down')
    expect(wrapper.emitted('update:modelValue')![1]).toEqual([0])
  })

  it('displays helper text "Min: X — Max: Y" when both min and max defined', () => {
    const wrapper = createWrapper({ min: 1, max: 20 })

    const helper = wrapper.find('.number-input__helper')
    expect(helper.exists()).toBe(true)
    expect(helper.text()).toBe('Min: 1 — Max: 20')
  })

  it('displays partial helper text when only min defined', () => {
    const wrapper = createWrapper({ min: 0 })

    const helper = wrapper.find('.number-input__helper')
    expect(helper.exists()).toBe(true)
    expect(helper.text()).toBe('Min: 0')
  })

  it('displays partial helper text when only max defined', () => {
    const wrapper = createWrapper({ max: 100 })

    const helper = wrapper.find('.number-input__helper')
    expect(helper.exists()).toBe(true)
    expect(helper.text()).toBe('Max: 100')
  })

  it('hides helper text when no min/max defined', () => {
    const wrapper = createWrapper()

    expect(wrapper.find('.number-input__helper').exists()).toBe(false)
  })

  it('disabled state applies CSS class and prevents interaction', async () => {
    const wrapper = createWrapper({ modelValue: 5, disabled: true })

    expect(wrapper.find('.number-input').classes()).toContain('number-input--disabled')
    expect(wrapper.find('input').attributes('disabled')).toBeDefined()

    const buttons = wrapper.findAll('button')
    for (const btn of buttons) {
      expect(btn.attributes('disabled')).toBeDefined()
    }

    await wrapper.find('input').trigger('keydown.arrow-up')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('updates displayed value when modelValue prop changes reactively', async () => {
    const wrapper = createWrapper({ modelValue: 5 })

    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('5')

    await wrapper.setProps({ modelValue: 99 })
    expect((wrapper.find('input').element as HTMLInputElement).value).toBe('99')
  })

  it('stepper buttons have correct aria-labels', () => {
    const wrapper = createWrapper()

    const buttons = wrapper.findAll('button')
    const labels = buttons.map(btn => btn.attributes('aria-label'))

    expect(labels).toContain('Increase value')
    expect(labels).toContain('Decrease value')
  })

  it('uses min as starting point when incrementing from null with min defined', async () => {
    const wrapper = createWrapper({ modelValue: null, min: 5 })

    const buttons = wrapper.findAll('button')
    const incrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Increase value')!
    await incrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([5])
  })

  it('uses custom step value for increment and decrement', async () => {
    const wrapper = createWrapper({ modelValue: 10, step: 5 })

    const buttons = wrapper.findAll('button')
    const incrementBtn = buttons.find(btn => btn.attributes('aria-label') === 'Increase value')!
    await incrementBtn.trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([15])

    const wrapper2 = createWrapper({ modelValue: 10, step: 5 })
    const decrementBtn = wrapper2.findAll('button').find(btn => btn.attributes('aria-label') === 'Decrease value')!
    await decrementBtn.trigger('click')

    expect(wrapper2.emitted('update:modelValue')![0]).toEqual([5])
  })
})
