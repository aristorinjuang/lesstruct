<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: number | null
  min?: number
  max?: number
  disabled?: boolean
  step?: number
}

interface Emits {
  (e: 'update:modelValue', value: number | null): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  step: 1
})

const emit = defineEmits<Emits>()

const atMax = computed(() =>
  props.max !== undefined && props.modelValue !== null && props.modelValue >= props.max
)

const atMin = computed(() =>
  props.min !== undefined && props.modelValue !== null && props.modelValue <= props.min
)

const helperText = computed(() => {
  const parts: string[] = []
  if (props.min !== undefined) parts.push(`Min: ${props.min}`)
  if (props.max !== undefined) parts.push(`Max: ${props.max}`)
  return parts.join(' — ')
})

function clamp(value: number): number {
  let clamped = value
  if (props.min !== undefined) clamped = Math.max(clamped, props.min)
  if (props.max !== undefined) clamped = Math.min(clamped, props.max)
  return clamped
}

function increment() {
  if (props.disabled) return
  const current = props.modelValue ?? 0
  emit('update:modelValue', clamp(current + props.step))
}

function decrement() {
  if (props.disabled) return
  const current = props.modelValue ?? 0
  emit('update:modelValue', clamp(current - props.step))
}

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  const value = target.value
  if (value === '') {
    emit('update:modelValue', null)
  } else {
    const num = Number(value)
    if (!isNaN(num) && Number.isFinite(num)) {
      emit('update:modelValue', clamp(num))
    }
  }
}
</script>

<template>
  <div class="number-input" :class="{ 'number-input--disabled': disabled }">
    <div class="number-input__control">
      <input
        type="number"
        class="number-input__field"
        :value="modelValue"
        :min="min"
        :max="max"
        :step="step"
        :disabled="disabled"
        inputmode="numeric"
        :aria-valuemin="min"
        :aria-valuemax="max"
        :aria-valuenow="modelValue ?? undefined"
        @input="onInput"
        @keydown.arrow-up.prevent="increment"
        @keydown.arrow-down.prevent="decrement"
      />
      <div class="number-input__steppers">
        <button
          type="button"
          class="number-input__stepper-btn"
          :disabled="disabled || atMax"
          aria-label="Increase value"
          @click="increment"
        >
          +
        </button>
        <button
          type="button"
          class="number-input__stepper-btn"
          :disabled="disabled || atMin"
          aria-label="Decrease value"
          @click="decrement"
        >
          −
        </button>
      </div>
    </div>
    <span v-if="helperText" class="number-input__helper">{{ helperText }}</span>
  </div>
</template>

<style scoped>
.number-input {
  display: block;
  width: 100%;
}

.number-input__control {
  display: flex;
  align-items: stretch;
  gap: 0;
  border-radius: 0.375rem;
}

.number-input__control:focus-within {
  box-shadow: 0 0 0 3px var(--brand-primary-light);
  border-radius: 0.375rem;
}

.number-input__field {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem 0 0 0.375rem;
  font-size: 1rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  text-align: center;
  transition: border-color 0.15s ease-in-out;
  -moz-appearance: textfield;
}

.number-input__field::-webkit-inner-spin-button,
.number-input__field::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.number-input__field:focus {
  outline: none;
  border-color: var(--brand-primary);
}

.number-input__steppers {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--brand-light-2);
  border-left: none;
  border-radius: 0 0.375rem 0.375rem 0;
  overflow: hidden;
  transition: border-color 0.15s ease-in-out;
}

.number-input__control:focus-within .number-input__steppers {
  border-color: var(--brand-primary);
}

.number-input__stepper-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  flex: 1;
  min-height: 22px;
  border: none;
  background: var(--brand-light-1);
  color: var(--brand-dark-1);
  font-size: 1.1rem;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.number-input__stepper-btn:first-child {
  border-bottom: 1px solid var(--brand-light-2);
}

.number-input__stepper-btn:hover {
  background: var(--brand-primary-light);
  color: var(--brand-primary);
}

.number-input__stepper-btn:active {
  background: var(--brand-primary);
  color: var(--brand-dark-1);
}

.number-input__stepper-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.number-input__helper {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.number-input--disabled {
  opacity: 0.6;
  pointer-events: none;
  cursor: not-allowed;
}

@media (max-width: 767px) {
  .number-input__field {
    font-size: 16px;
  }
}
</style>
