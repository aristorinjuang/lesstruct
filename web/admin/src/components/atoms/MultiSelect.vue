<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  modelValue: string[]
  options: string[]
  disabled?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: string[]): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  modelValue: () => []
})

const emit = defineEmits<Emits>()

const hasOptions = computed(() => props.options.length > 0)

function isSelected(option: string): boolean {
  return props.modelValue.includes(option)
}

function toggle(option: string): void {
  if (props.disabled) return
  const next = isSelected(option)
    ? props.modelValue.filter((value) => value !== option)
    : [...props.modelValue, option]
  emit('update:modelValue', next)
}
</script>

<template>
  <div class="multiselect" role="group" :aria-disabled="disabled">
    <template v-if="hasOptions">
      <button
        v-for="option in options"
        :key="option"
        type="button"
        class="multiselect__chip"
        :class="{ 'multiselect__chip--active': isSelected(option) }"
        :aria-pressed="isSelected(option)"
        :disabled="disabled"
        @click="toggle(option)"
      >
        {{ option }}
      </button>
    </template>
    <span v-else class="multiselect__empty">No options configured</span>
  </div>
</template>

<style scoped>
.multiselect {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.multiselect__chip {
  padding: 0.25rem 0.75rem;
  border: 1px solid var(--btn-neutral-border);
  border-radius: 999px;
  background: var(--btn-neutral-bg);
  color: var(--btn-neutral-fg);
  font-size: 0.875rem;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.multiselect__chip:hover:not(:disabled) {
  background: var(--btn-neutral-hover-bg);
}

.multiselect__chip--active {
  background: var(--btn-primary-bg);
  border-color: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
}

.multiselect__chip:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.multiselect__empty {
  color: var(--brand-dark-2);
  font-size: 0.875rem;
}
</style>
