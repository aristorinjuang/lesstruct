<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import type { FieldType, FieldSchema } from '@/types/customfield'
import InputText from '@/components/atoms/InputText.vue'
import NumberInput from '@/components/atoms/NumberInput.vue'
import DateInput from '@/components/atoms/DateInput.vue'
import DateTimeInput from '@/components/atoms/DateTimeInput.vue'
import Select from '@/components/atoms/Select.vue'
import SwitchToggle from '@/components/atoms/SwitchToggle.vue'
import MultiSelect from '@/components/atoms/MultiSelect.vue'
import ListInput from '@/components/molecules/ListInput.vue'
import FormField from '@/components/molecules/FormField.vue'

interface Props {
  field: FieldSchema
  modelValue: unknown
  error?: string
  disabled?: boolean
  systemField?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: unknown): void
  (e: 'blur'): void
}

const props = withDefaults(defineProps<Props>(), {
  error: '',
  disabled: false,
  systemField: false
})

defineEmits<Emits>()

const labelId = computed(() => `field-label-${props.field.slug}`)

function getFieldComponent(type: FieldType): Component {
  // gallery is not mapped here: ContentEditor hosts the gallery fields
  // directly (they need the media panel, an organism) and never reaches the
  // generic renderer.
  const map: Partial<Record<FieldType, Component>> = {
    text: InputText,
    textarea: InputText,
    number: NumberInput,
    date: DateInput,
    datetime: DateTimeInput,
    email: InputText,
    url: InputText,
    select: Select,
    checkbox: SwitchToggle,
    multiselect: MultiSelect,
    list: ListInput,
  }
  return map[type] as Component
}

function getFieldProps(field: FieldSchema): Record<string, unknown> {
  const base = { disabled: props.disabled }

  switch (field.type) {
    case 'text':
      return base
    case 'textarea': {
      const p: Record<string, unknown> = { ...base, multiline: true }
      if (field.maxLength != null) p.maxLength = field.maxLength
      return p
    }
    case 'number': {
      const p: Record<string, unknown> = { ...base }
      if (field.min != null) p.min = field.min
      if (field.max != null) p.max = field.max
      return p
    }
    case 'date':
      return base
    case 'datetime':
      return base
    case 'email':
      return { ...base, inputType: 'email' }
    case 'url':
      return { ...base, inputType: 'url' }
    case 'select':
      return { ...base, options: (field.options ?? []).map(o => ({ value: o, label: o })) }
    case 'multiselect':
      return { ...base, options: field.options ?? [] }
    case 'list':
      return {
        ...base,
        maxLength: field.maxLength,
        maxItems: field.maxItems,
        placeholder: `Add an item${field.maxLength ? ` (max ${field.maxLength} characters)` : ''}`
      }
    case 'checkbox':
      return { ...base, label: '', 'aria-labelledby': labelId.value }
    default:
      return base
  }
}
</script>

<template>
  <FormField :id="labelId" :label="field.name" :required="field.required" :error="error" :label-suffix="systemField ? 'System' : undefined" :class="`form-field--${field.type}`">
    <component
      :is="getFieldComponent(field.type)"
      :key="field.slug"
      :model-value="modelValue"
      v-bind="getFieldProps(field)"
      @update:model-value="$emit('update:modelValue', $event)"
      @blur="$emit('blur')"
    />
  </FormField>
</template>
