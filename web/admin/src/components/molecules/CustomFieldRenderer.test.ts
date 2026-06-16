import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import type { FieldSchema } from '@/types/customfield'
import CustomFieldRenderer from './CustomFieldRenderer.vue'
import InputText from '@/components/atoms/InputText.vue'
import NumberInput from '@/components/atoms/NumberInput.vue'
import DateInput from '@/components/atoms/DateInput.vue'
import Select from '@/components/atoms/Select.vue'
import SwitchToggle from '@/components/atoms/SwitchToggle.vue'
import FormField from './FormField.vue'

function createWrapper(field: Partial<FieldSchema> & { name: string; slug: string; type: FieldSchema['type'] }, extraProps = {}) {
  return mount(CustomFieldRenderer, {
    props: {
      field: { required: false, ...field } as FieldSchema,
      modelValue: '',
      ...extraProps
    },
    global: {
      components: { FormField }
    }
  })
}

const baseField = { name: 'Test Field', slug: 'test-field' }

describe('CustomFieldRenderer', () => {
  describe('field type to component mapping', () => {
    it('renders InputText for text field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text' })

      expect(wrapper.findComponent(InputText).exists()).toBe(true)
    })

    it('renders InputText with multiline prop for textarea field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'textarea' })

      const inputText = wrapper.findComponent(InputText)
      expect(inputText.exists()).toBe(true)
      expect(inputText.props('multiline')).toBe(true)
    })

    it('renders NumberInput for number field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'number' })

      expect(wrapper.findComponent(NumberInput).exists()).toBe(true)
    })

    it('renders DateInput for date field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'date' })

      expect(wrapper.findComponent(DateInput).exists()).toBe(true)
    })

    it('renders Select for select field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'select' })

      expect(wrapper.findComponent(Select).exists()).toBe(true)
    })

    it('renders SwitchToggle for checkbox field type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'checkbox', modelValue: false })

      expect(wrapper.findComponent(SwitchToggle).exists()).toBe(true)
    })
  })

  describe('FormField wrapping', () => {
    it('wraps field in FormField with label', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text' })

      const formField = wrapper.findComponent(FormField)
      expect(formField.exists()).toBe(true)
      expect(formField.props('label')).toBe('Test Field')
    })

    it('passes required prop to FormField', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text', required: true })

      const formField = wrapper.findComponent(FormField)
      expect(formField.props('required')).toBe(true)
    })

    it('passes error prop to FormField', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'text' },
        { error: 'This field is required' }
      )

      const formField = wrapper.findComponent(FormField)
      expect(formField.props('error')).toBe('This field is required')
    })

    it('shows required asterisk when field is required', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text', required: true })

      expect(wrapper.find('.form-field__required').exists()).toBe(true)
    })

    it('does not show required asterisk when field is not required', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text', required: false })

      expect(wrapper.find('.form-field__required').exists()).toBe(false)
    })
  })

  describe('prop forwarding', () => {
    it('passes min/max to NumberInput', () => {
      const wrapper = createWrapper({ ...baseField, type: 'number', min: 1, max: 20 })

      const numberInput = wrapper.findComponent(NumberInput)
      expect(numberInput.props('min')).toBe(1)
      expect(numberInput.props('max')).toBe(20)
    })

    it('passes maxLength to InputText for textarea type', () => {
      const wrapper = createWrapper({ ...baseField, type: 'textarea', maxLength: 500 })

      const inputText = wrapper.findComponent(InputText)
      expect(inputText.props('multiline')).toBe(true)
      expect(inputText.props('maxLength')).toBe(500)
    })

    it('maps string options to SelectOption format for select type', () => {
      const wrapper = createWrapper({
        ...baseField,
        type: 'select',
        options: ['Pastry', 'Bread', 'Cake']
      })

      const select = wrapper.findComponent(Select)
      expect(select.props('options')).toEqual([
        { value: 'Pastry', label: 'Pastry' },
        { value: 'Bread', label: 'Bread' },
        { value: 'Cake', label: 'Cake' }
      ])
    })

    it('handles missing optional fields gracefully (no options)', () => {
      const wrapper = createWrapper({ ...baseField, type: 'select' })

      const select = wrapper.findComponent(Select)
      expect(select.props('options')).toEqual([])
    })

    it('passes empty label to SwitchToggle to suppress internal label', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'checkbox' },
        { modelValue: false }
      )

      const toggle = wrapper.findComponent(SwitchToggle)
      expect(toggle.props('label')).toBe('')
    })

    it('passes aria-labelledby to SwitchToggle for accessibility', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'checkbox' },
        { modelValue: false }
      )

      const toggle = wrapper.findComponent(SwitchToggle)
      expect(toggle.attributes('aria-labelledby')).toBe('field-label-test-field')
    })
  })

  describe('v-model binding', () => {
    it('passes modelValue to the rendered atom', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'text' },
        { modelValue: 'hello world' }
      )

      const inputText = wrapper.findComponent(InputText)
      expect(inputText.props('modelValue')).toBe('hello world')
    })

    it('emits update:modelValue when atom changes', async () => {
      const wrapper = createWrapper({ ...baseField, type: 'text' })

      const inputText = wrapper.findComponent(InputText)
      inputText.vm.$emit('update:modelValue', 'new value')

      expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
      expect(wrapper.emitted('update:modelValue')![0]).toEqual(['new value'])
    })

    it('emits update:modelValue for checkbox type', async () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'checkbox' },
        { modelValue: false }
      )

      const toggle = wrapper.findComponent(SwitchToggle)
      toggle.vm.$emit('update:modelValue', true)

      expect(wrapper.emitted('update:modelValue')).toHaveLength(1)
      expect(wrapper.emitted('update:modelValue')![0]).toEqual([true])
    })
  })

  describe('disabled state', () => {
    it('passes disabled prop to the rendered atom', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'text' },
        { disabled: true }
      )

      const inputText = wrapper.findComponent(InputText)
      expect(inputText.props('disabled')).toBe(true)
    })

    it('passes disabled prop to NumberInput', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'number' },
        { disabled: true }
      )

      const numberInput = wrapper.findComponent(NumberInput)
      expect(numberInput.props('disabled')).toBe(true)
    })

    it('passes disabled prop to SwitchToggle', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'checkbox' },
        { modelValue: false, disabled: true }
      )

      const toggle = wrapper.findComponent(SwitchToggle)
      expect(toggle.props('disabled')).toBe(true)
    })

    it('passes disabled prop to DateInput', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'date' },
        { disabled: true }
      )

      const dateInput = wrapper.findComponent(DateInput)
      expect(dateInput.props('disabled')).toBe(true)
    })

    it('passes disabled prop to Select', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'select' },
        { disabled: true }
      )

      const select = wrapper.findComponent(Select)
      expect(select.props('disabled')).toBe(true)
    })
  })

  describe('systemField indicator', () => {
    it('passes labelSuffix="System" to FormField when systemField is true', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'text' },
        { systemField: true }
      )

      const formField = wrapper.findComponent(FormField)
      expect(formField.props('labelSuffix')).toBe('System')
    })

    it('passes falsy labelSuffix to FormField when systemField is false', () => {
      const wrapper = createWrapper(
        { ...baseField, type: 'text' },
        { systemField: false }
      )

      const formField = wrapper.findComponent(FormField)
      expect(formField.props('labelSuffix')).toBeFalsy()
    })

    it('passes falsy labelSuffix to FormField when systemField is not provided', () => {
      const wrapper = createWrapper({ ...baseField, type: 'text' })

      const formField = wrapper.findComponent(FormField)
      expect(formField.props('labelSuffix')).toBeFalsy()
    })
  })
})
