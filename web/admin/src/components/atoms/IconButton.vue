<script setup lang="ts">
type IconButtonTone = 'neutral' | 'danger'
type IconButtonSize = 'small' | 'medium' | 'large'

interface Props {
  label: string
  tone?: IconButtonTone
  size?: IconButtonSize
  disabled?: boolean
}

withDefaults(defineProps<Props>(), {
  tone: 'neutral',
  size: 'medium',
  disabled: false
})
</script>

<template>
  <button
    type="button"
    class="icon-button"
    :class="[`icon-button--${size}`, `icon-button--tone-${tone}`, { 'icon-button--disabled': disabled }]"
    :aria-label="label"
    :disabled="disabled"
  >
    <slot />
  </button>
</template>

<style scoped>
.icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: 0.375rem;
  color: inherit;
  cursor: pointer;
  transition: background-color 0.15s ease-in-out;
}

.icon-button:hover:not(.icon-button--disabled) {
  background-color: var(--brand-light-2);
}

.icon-button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.icon-button--small {
  width: 1.5rem;
  height: 1.5rem;
}

.icon-button--medium {
  width: 2rem;
  height: 2rem;
}

.icon-button--large {
  width: 2.5rem;
  height: 2.5rem;
}

.icon-button--tone-danger {
  color: var(--color-error);
}

.icon-button--tone-danger:hover:not(.icon-button--disabled) {
  background-color: var(--color-error-bg);
}

.icon-button--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
