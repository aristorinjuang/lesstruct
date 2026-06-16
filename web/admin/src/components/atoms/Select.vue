<script setup lang="ts" generic="T extends string | number">
import { computed } from 'vue'

interface SelectOption {
  value: T
  label: string
}

interface Props {
  modelValue: T
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: T): void
}

const props = withDefaults(defineProps<Props>(), {
  placeholder: 'Select an option',
  disabled: false
})

const emit = defineEmits<Emits>()

const classes = computed(() => [
  'select',
  { 'select--disabled': props.disabled }
])

function onChange(event: Event) {
  const target = event.target as HTMLSelectElement
  emit('update:modelValue', target.value as T)
}
</script>

<template>
  <select
    :class="classes"
    :disabled="disabled"
    :value="modelValue"
    @change="onChange"
  >
    <option v-if="placeholder" value="" disabled selected>{{ placeholder }}</option>
    <option
      v-for="option in options"
      :key="option.value"
      :value="option.value"
    >
      {{ option.label }}
    </option>
  </select>
</template>

<style scoped>
.select {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 1rem;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
  cursor: pointer;
  transition: border-color 0.15s ease-in-out;
}

.select:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.select--disabled {
  background-color: var(--brand-light-1);
  cursor: not-allowed;
  opacity: 0.6;
}

/* Mobile: Ensure 16px minimum font-size to prevent iOS auto-zoom */
@media (max-width: 767px) {
  .select {
    font-size: 16px;
  }
}
</style>
