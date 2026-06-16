<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: string
  disabled?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false
})

const emit = defineEmits<Emits>()

const classes = computed(() => [
  'date-input',
  { 'date-input--disabled': props.disabled }
])

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <input
    type="date"
    :class="classes"
    :disabled="disabled"
    :value="modelValue"
    @input="onInput"
  />
</template>

<style scoped>
.date-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  transition: border-color 0.15s ease-in-out;
}

.date-input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.date-input--disabled {
  background-color: var(--brand-light-1);
  cursor: not-allowed;
  opacity: 0.6;
}

/* Mobile: Ensure 16px minimum font-size to prevent iOS auto-zoom */
@media (max-width: 767px) {
  .date-input {
    font-size: 16px;
  }
}
</style>
