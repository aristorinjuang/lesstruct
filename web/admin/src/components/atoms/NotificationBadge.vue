<script setup lang="ts">
import { computed } from 'vue'
import type { NotificationBadgeProps } from '@/types/notifications'

const props = withDefaults(defineProps<NotificationBadgeProps>(), {
  maxCount: 99,
})

// Clamp to non-negative to guard against invalid values
const safeCount = computed(() => Math.max(0, props.count))

// Only show the badge when count is greater than 0
const isVisible = computed(() => safeCount.value > 0)

// Display "99+" for counts exceeding maxCount
const displayCount = computed(() => {
  if (safeCount.value > props.maxCount) return `${props.maxCount}+`
  return safeCount.value.toString()
})

// ARIA label for accessibility
const ariaLabel = computed(() => `${safeCount.value} pending items`)
</script>

<template>
  <span
    v-if="isVisible"
    class="notification-badge"
    :aria-label="ariaLabel"
    role="status"
    aria-live="polite"
    tabindex="0"
  >
    {{ displayCount }}
  </span>
</template>

<style scoped>
.notification-badge {
  position: absolute;
  top: -0.5rem;
  right: -0.75rem;
  background-color: var(--brand-accent);
  color: white;
  border-radius: 9999px;
  min-width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0 0.375rem;
  box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05);
  z-index: 10;
  outline: none;
}

.notification-badge:focus-visible {
  box-shadow: 0 0 0 2px white, 0 0 0 4px var(--brand-accent);
}

/* Mobile touch target (44x44px minimum) */
@media (max-width: 767px) {
  .notification-badge {
    min-width: 44px;
    min-height: 44px;
    font-size: 0.875rem;
    padding: 0 0.5rem;
  }
}
</style>
