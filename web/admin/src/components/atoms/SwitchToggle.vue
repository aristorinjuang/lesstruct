<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: boolean
  label: string
  disabled?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: boolean): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false
})

const emit = defineEmits<Emits>()

const classes = computed(() => [
  'switch-toggle',
  { 'switch-toggle--active': props.modelValue },
  { 'switch-toggle--disabled': props.disabled }
])

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>

<template>
  <div
    :class="classes"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="label"
    :tabindex="disabled ? -1 : 0"
    @click="toggle"
    :aria-disabled="disabled"
    @keydown.space.prevent="toggle"
    @keydown.enter.prevent="toggle"
  >
    <span class="switch-toggle__label">{{ label }}</span>
    <span class="switch-toggle__track">
      <span class="switch-toggle__thumb" />
    </span>
  </div>
</template>

<style scoped>
.switch-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 44px;
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  border-radius: 0.375rem;
  border: 1px solid transparent;
}

.switch-toggle:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.switch-toggle__track {
  width: 44px;
  height: 24px;
  border-radius: 12px;
  background-color: var(--brand-light-2);
  transition: background-color 0.15s ease-in-out;
  position: relative;
  flex-shrink: 0;
}

.switch-toggle__thumb {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background-color: white;
  position: absolute;
  top: 3px;
  left: 3px;
  transition: transform 0.15s ease-in-out;
}

.switch-toggle--active .switch-toggle__track {
  background-color: var(--brand-primary);
}

.switch-toggle--active .switch-toggle__thumb {
  transform: translateX(20px);
}

.switch-toggle--disabled {
  opacity: 0.6;
  pointer-events: none;
  cursor: not-allowed;
}

.switch-toggle__label {
  font-size: 1rem;
  color: var(--brand-dark-1);
  flex: 1;
  padding-right: 0.75rem;
}

@media (max-width: 767px) {
  .switch-toggle__label {
    font-size: 16px;
  }
}
</style>
