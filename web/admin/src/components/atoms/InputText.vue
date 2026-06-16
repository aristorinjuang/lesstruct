<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: string
  placeholder?: string
  disabled?: boolean
  size?: 'small' | 'medium' | 'large'
  multiline?: boolean
  maxLength?: number
}

interface Emits {
  (e: 'update:modelValue', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  placeholder: '',
  disabled: false,
  size: 'medium',
  multiline: false
})

const emit = defineEmits<Emits>()

const classes = computed(() => [
  'input-text',
  `input-text--${props.size}`,
  { 'input-text--disabled': props.disabled }
])

function onInput(event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <div class="input-text-wrapper">
    <textarea
      v-if="multiline"
      :class="classes"
      :placeholder="placeholder"
      :disabled="disabled"
      :maxlength="maxLength"
      :value="modelValue"
      @input="onInput"
    />
    <input
      v-else
      :class="classes"
      :placeholder="placeholder"
      :disabled="disabled"
      :maxlength="maxLength"
      :value="modelValue"
      @input="onInput"
    />
    <span v-if="maxLength" class="input-text__counter">{{ (modelValue ?? '').length }}/{{ maxLength }}</span>
  </div>
</template>

<style scoped>
.input-text-wrapper {
  width: 100%;
}

.input-text {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 1rem;
  font-family: inherit;
  transition: border-color 0.15s ease-in-out;
}

.input-text:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.input-text--small {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
}

.input-text--large {
  padding: 0.75rem 1rem;
  font-size: 1.25rem;
}

.input-text--disabled {
  background-color: var(--color-bg-muted);
  cursor: not-allowed;
}

textarea.input-text {
  resize: vertical;
  min-height: 80px;
}

.input-text__counter {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
  text-align: right;
}

/* Mobile: Ensure 16px minimum font-size to prevent iOS auto-zoom */
@media (max-width: 767px) {
  .input-text {
    font-size: 16px;
  }

  .input-text--small {
    font-size: 16px;
  }

  .input-text--large {
    font-size: 16px;
  }
}
</style>
