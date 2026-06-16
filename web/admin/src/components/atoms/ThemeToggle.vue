<script setup lang="ts">
import { computed } from 'vue'
import { useTheme } from '@/composables/useTheme'
import IconSun from '@/components/icons/IconSun.vue'
import IconMoon from '@/components/icons/IconMoon.vue'

const { resolvedTheme, toggleTheme } = useTheme()

const ariaLabel = computed(() => {
  return resolvedTheme.value === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'
})

function handleClick() {
  toggleTheme()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    handleClick()
  }
}
</script>

<template>
  <button
    class="theme-toggle"
    :aria-label="ariaLabel"
    :title="ariaLabel"
    @click="handleClick"
    @keydown="handleKeydown"
    type="button"
  >
    <IconMoon v-if="resolvedTheme === 'light'" class="theme-toggle__icon" />
    <IconSun v-else class="theme-toggle__icon" />
  </button>
</template>

<style scoped>
.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  padding: 0.5rem;
  cursor: pointer;
  transition: background-color 0.2s, border-color 0.2s, color 0.3s ease-in-out;
  min-width: 44px;
  min-height: 44px;
  color: var(--brand-dark-1);
}

.theme-toggle:hover {
  background-color: var(--brand-primary-light);
  border-color: var(--brand-primary);
}

.theme-toggle:focus {
  outline: none;
  box-shadow: 0 0 0 3px var(--brand-primary-light);
}

.theme-toggle__icon {
  width: 20px;
  height: 20px;
}
</style>
