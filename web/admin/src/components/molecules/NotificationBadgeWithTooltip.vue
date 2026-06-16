<script setup lang="ts">
import { ref } from 'vue'
import NotificationBadge from '@/components/atoms/NotificationBadge.vue'
import type { NotificationBadgeWithTooltipProps } from '@/types/notifications'

withDefaults(defineProps<NotificationBadgeWithTooltipProps>(), {
  maxCount: 99,
})

const isTooltipVisible = ref(false)

function showTooltip() {
  isTooltipVisible.value = true
}

function hideTooltip() {
  isTooltipVisible.value = false
}
</script>

<template>
  <span
    class="notification-badge-with-tooltip"
    @mouseenter="showTooltip"
    @mouseleave="hideTooltip"
    @focusin="showTooltip"
    @focusout="hideTooltip"
  >
    <NotificationBadge
      :count="count"
      :max-count="maxCount"
    />
    <span
      v-if="isTooltipVisible"
      class="notification-badge-tooltip"
      role="tooltip"
    >
      {{ tooltipText }}
    </span>
  </span>
</template>

<style scoped>
.notification-badge-with-tooltip {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.notification-badge-tooltip {
  position: absolute;
  bottom: calc(100% + 0.5rem);
  left: 50%;
  transform: translateX(-50%);
  background-color: var(--color-bg-inverse);
  color: white;
  font-size: 0.75rem;
  font-weight: 500;
  padding: 0.375rem 0.625rem;
  border-radius: 0.375rem;
  white-space: nowrap;
  pointer-events: none;
  box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1);
  z-index: 50;
}

.notification-badge-tooltip::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 4px solid transparent;
  border-top-color: var(--color-bg-inverse);
}

/* Mobile adjustments */
@media (max-width: 767px) {
  .notification-badge-tooltip {
    font-size: 0.875rem;
    padding: 0.5rem 0.75rem;
  }
}
</style>
