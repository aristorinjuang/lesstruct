<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  isActive?: boolean
  isDisabled?: boolean
  icon: string
  label: string
  shortcut?: string
}

interface Emits {
  (e: 'click'): void
}

const props = withDefaults(defineProps<Props>(), {
  isActive: false,
  isDisabled: false
})

const emit = defineEmits<Emits>()

const iconPath = computed(() => {
  const icons: Record<string, string> = {
    bold: 'M6 4h8a4 4 0 0 1 4 4 4 4 0 0 1-4 4H6z M6 12h9a4 4 0 0 1 4 4 4 4 0 0 1-4 4H6z',
    italic: 'M19 4h-9 M14 20H5 M15 4L9 20',
    underline: 'M6 3v7a6 6 0 0 0 6 6 6 6 0 0 0 6-6V3 M4 21h16',
    list: 'M8 6h13 M8 12h13 M8 18h13 M3 6h.01 M3 12h.01 M3 18h.01',
    numberedList: 'M8 6h13 M8 12h13 M8 18h13 M3 5v4h3V5H3zM3 11v4h3v-4H3zM3 17v4h3v-4H3z',
    quote: 'M3 21c3 0 7-1 7-8V5c0-1.25-.756-2.017-2-2H4c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2 1 0 1 0 1 1v1c0 1-1 2-2 2s-1 .008-1 1.031V20c0 1 0 1 1 1z M21 3c0 1-1 2-2 2s-1 .008-1 1.031V20c0 1 0 1 1 1 3 0 7-1 7-8V5c0-1.25-.757-2.017-2-2h-2c-1.25 0-2 .75-2 1.972V11c0 1.25.75 2 2 2 1 0 1 0 1 1v1c0 1-1 2-2 2s-1 .008-1 1.031V20c0 1 0 1 1 1z',
    code: 'M16 18l6-6-6-6 M8 6l-6 6 6 6',
    link: 'M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71 M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71',
    image: 'M21 19V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2z M11 13l-3-3H6v4h12v-4l-3 3z',
    hr: 'M4 7h16 M4 12h16 M4 17h16',
    table: 'M3 3h18v18H3V3z M3 9h18 M3 15h18 M9 3v18 M15 3v18',
    youtube: 'M22.54 6.42a2.78 2.78 0 0 0-1.94-2C18.88 4 12 4 12 4s-6.88 0-8.6.46a2.78 2.78 0 0 0-1.94 2A29.94 29.94 0 0 0 1 12a29.94 29.94 0 0 0 .46 5.58 2.78 2.78 0 0 0 1.94 2C5.12 20 12 20 12 20s6.88 0 8.6-.46a2.78 2.78 0 0 0 1.94-2A29.94 29.94 0 0 0 23 12a29.94 29.94 0 0 0-.46-5.58z M9.75 15.02l5.5-3.27-5.5-3.27v6.54z',
    'inline-math': 'M7.5 2.5c-1.5 0-2.5 1-2.5 2.5v14c0 1.5 1 2.5 2.5 2.5h9c1.5 0 2.5-1 2.5-2.5V5c0-1.5-1-2.5-2.5-2.5h-9z M6 8l2-2 2 2 M6 12l2 2 2-2 M12 16l2-2 2 2 M12 12l2 2 2-2',
    'block-math': 'M4 16h16 M4 12h16 M21 4H3v16h18V4z M8 6l-4 4 4 4 M16 18l4-4-4-4',
    undo: 'M9 14 4 9l5-5',
    redo: 'M15 14l5-5-5-5'
  }
  return icons[props.icon] || icons.bold
})
</script>

<template>
  <button
    type="button"
    class="editor-btn"
    :class="{ 'editor-btn--active': isActive }"
    :disabled="isDisabled"
    :aria-label="label + (shortcut ? ` (${shortcut})` : '')"
    :aria-pressed="isActive"
    :title="label + (shortcut ? ` (${shortcut})` : '')"
    @click="emit('click')"
  >
    <svg class="editor-btn__icon" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
      <path :d="iconPath" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>
</template>

<style scoped>
.editor-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  padding: 0.375rem;
  border: none;
  border-radius: 0.25rem;
  background: transparent;
  color: var(--brand-dark-1);
  cursor: pointer;
  transition: background-color 0.15s, color 0.15s;
  flex-shrink: 0;
}

.editor-btn:hover:not(:disabled) {
  background-color: var(--brand-light-2);
}

.editor-btn--active {
  background-color: var(--brand-primary-light);
  color: var(--brand-primary-hover);
}

.editor-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.editor-btn:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 1px;
}

.editor-btn__icon {
  width: 1.125rem;
  height: 1.125rem;
}

@media (max-width: 768px) {
  .editor-btn {
    width: 2.75rem;
    height: 2.75rem;
  }
}
</style>
