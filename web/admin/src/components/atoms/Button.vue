<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  type?: 'button' | 'submit' | 'reset'
  variant?: 'primary' | 'secondary' | 'danger'
  size?: 'small' | 'medium' | 'large'
  disabled?: boolean
  isLoading?: boolean
}

interface Emits {
  (e: 'click', event: MouseEvent): void
}

const props = withDefaults(defineProps<Props>(), {
  type: 'button',
  variant: 'primary',
  size: 'medium',
  disabled: false,
  isLoading: false
})

const emit = defineEmits<Emits>()

const classes = computed(() => [
  'button',
  `button--${props.variant}`,
  `button--${props.size}`,
  { 'button--disabled': props.disabled || props.isLoading }
])

function onClick(event: MouseEvent) {
  if (!props.disabled && !props.isLoading) {
    emit('click', event)
  }
}
</script>

<template>
  <button
    :class="classes"
    :type="type"
    :disabled="disabled || isLoading"
    @click="onClick"
  >
    <span v-if="isLoading" class="button__spinner"></span>
    <slot />
  </button>
</template>

<style scoped>
.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 0.375rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease-in-out;
}

.button--primary {
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
}

.button--primary:hover:not(.button--disabled) {
  background-color: var(--brand-primary-hover);
}

.button--secondary {
  background-color: var(--brand-secondary);
  color: white;
}

.button--secondary:hover:not(.button--disabled) {
  background-color: var(--brand-secondary-hover);
}

.button--danger {
  background-color: var(--color-error);
  color: var(--color-white);
}

.button--danger:hover:not(.button--disabled) {
  background-color: var(--color-error);
}

.button--small {
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
}

.button--large {
  padding: 0.75rem 1.5rem;
  font-size: 1.125rem;
}

.button--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.button__spinner {
  width: 1rem;
  height: 1rem;
  border: 2px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
