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
  'datetime-input',
  { 'datetime-input--disabled': props.disabled }
])

const displayValue = computed(() => toLocalInput(props.modelValue))

function toLocalInput(rfc3339: string): string {
  if (!rfc3339) return ''
  const d = new Date(rfc3339)
  if (isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', toRFC3339(target.value))
}

function toRFC3339(local: string): string {
  if (!local) return ''
  const d = new Date(local)
  if (isNaN(d.getTime())) return ''
  const offsetMin = -d.getTimezoneOffset()
  const sign = offsetMin >= 0 ? '+' : '-'
  const abs = Math.abs(offsetMin)
  const hh = String(Math.floor(abs / 60)).padStart(2, '0')
  const mm = String(abs % 60).padStart(2, '0')
  return `${local}:00${sign}${hh}:${mm}`
}
</script>

<template>
  <input
    type="datetime-local"
    :class="classes"
    :disabled="disabled"
    :value="displayValue"
    @input="onInput"
  />
</template>

<style scoped>
.datetime-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 1rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  transition: border-color 0.15s ease-in-out;
}

.datetime-input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.datetime-input--disabled {
  background-color: var(--brand-light-1);
  cursor: not-allowed;
  opacity: 0.6;
}

/* Mobile: Ensure 16px minimum font-size to prevent iOS auto-zoom */
@media (max-width: 767px) {
  .datetime-input {
    font-size: 16px;
  }
}
</style>
