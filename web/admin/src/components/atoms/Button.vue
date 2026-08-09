<script setup lang="ts">
import { computed, ref } from 'vue'

type ButtonVariant = 'primary' | 'secondary' | 'neutral' | 'danger' | 'ghost' | 'link' | 'subtle'
type ButtonTone = 'success' | 'danger' | 'warning' | 'info' | 'neutral'
type ButtonSize = 'small' | 'medium' | 'large'

interface Props {
  type?: 'button' | 'submit' | 'reset'
  variant?: ButtonVariant
  tone?: ButtonTone
  size?: ButtonSize
  disabled?: boolean
  isLoading?: boolean
  fullWidth?: boolean
}

interface Emits {
  (e: 'click', event: MouseEvent): void
}

const props = withDefaults(defineProps<Props>(), {
  type: 'button',
  variant: 'primary',
  tone: 'neutral',
  size: 'medium',
  disabled: false,
  isLoading: false,
  fullWidth: false
})

const emit = defineEmits<Emits>()

const rootEl = ref<HTMLButtonElement | null>(null)

const normalizedVariant = computed(() =>
  props.variant === 'secondary' ? 'neutral' : props.variant
)

const classes = computed(() => [
  'button',
  `button--${normalizedVariant.value}`,
  ...(normalizedVariant.value === 'subtle' ||
  normalizedVariant.value === 'ghost' ||
  normalizedVariant.value === 'link'
    ? [`button--tone-${props.tone}`]
    : []),
  `button--${props.size}`,
  { 'button--disabled': props.disabled || props.isLoading },
  { 'button--full-width': props.fullWidth }
])

function onClick(event: MouseEvent) {
  if (!props.disabled && !props.isLoading) {
    emit('click', event)
  }
}

defineExpose({
  focus: () => rootEl.value?.focus(),
})
</script>

<template>
  <button
    ref="rootEl"
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
  transition:
    background-color 0.15s ease-in-out,
    border-color 0.15s ease-in-out,
    color 0.15s ease-in-out;
}

.button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.button--primary {
  background-color: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
}

.button--primary:hover:not(.button--disabled) {
  background-color: var(--btn-primary-hover-bg);
}

.button--neutral {
  background-color: var(--btn-neutral-bg);
  border: 1px solid var(--btn-neutral-border);
  color: var(--btn-neutral-fg);
}

.button--neutral:hover:not(.button--disabled) {
  background-color: var(--btn-neutral-hover-bg);
}

.button--danger {
  background-color: var(--btn-danger-bg);
  color: var(--btn-danger-fg);
}

.button--danger:hover:not(.button--disabled) {
  background-color: var(--btn-danger-hover-bg);
}

.button--ghost {
  background-color: transparent;
  border: 1px solid var(--btn-ghost-border);
  color: var(--btn-ghost-fg);
}

.button--ghost:hover:not(.button--disabled) {
  background-color: var(--btn-ghost-hover-bg);
}

.button--link {
  background-color: transparent;
  color: var(--btn-link-fg);
  padding: 0.25rem 0.5rem;
}

.button--link:hover:not(.button--disabled) {
  color: var(--btn-link-hover-fg);
  text-decoration: underline;
}

.button--subtle {
  background-color: var(--btn-subtle-neutral-bg);
  color: var(--btn-subtle-neutral-fg);
  padding: 0.25rem 0.625rem;
  font-size: 0.8125rem;
}

.button--subtle:hover:not(.button--disabled) {
  background-color: var(--btn-subtle-neutral-hover-bg);
}

.button--subtle.button--tone-success {
  background-color: var(--btn-subtle-success-bg);
  color: var(--btn-subtle-success-fg);
}

.button--subtle.button--tone-success:hover:not(.button--disabled) {
  background-color: var(--btn-subtle-success-hover-bg);
}

.button--subtle.button--tone-danger {
  background-color: var(--btn-subtle-danger-bg);
  color: var(--btn-subtle-danger-fg);
}

.button--subtle.button--tone-danger:hover:not(.button--disabled) {
  background-color: var(--btn-subtle-danger-hover-bg);
}

.button--subtle.button--tone-warning {
  background-color: var(--btn-subtle-warning-bg);
  color: var(--btn-subtle-warning-fg);
}

.button--subtle.button--tone-warning:hover:not(.button--disabled) {
  background-color: var(--btn-subtle-warning-hover-bg);
}

.button--subtle.button--tone-info {
  background-color: var(--btn-subtle-info-bg);
  color: var(--btn-subtle-info-fg);
}

.button--subtle.button--tone-info:hover:not(.button--disabled) {
  background-color: var(--btn-subtle-info-hover-bg);
}

.button--ghost.button--tone-danger,
.button--link.button--tone-danger {
  border-color: var(--color-error-border);
  color: var(--color-error);
}

.button--ghost.button--tone-danger:hover:not(.button--disabled),
.button--link.button--tone-danger:hover:not(.button--disabled) {
  background-color: var(--color-error-bg);
  border-color: var(--color-error);
  color: var(--color-error-dark);
}

.button--small {
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
}

.button--large {
  padding: 0.75rem 1.5rem;
  font-size: 1.125rem;
}

.button--full-width {
  width: 100%;
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
