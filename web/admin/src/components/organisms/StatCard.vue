<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { getIcon } from '@/components/icons'
import NotificationBadge from '@/components/atoms/NotificationBadge.vue'
import type { StatCardProps } from '@/types/dashboard'

// Props
interface Props extends StatCardProps {
  /** Optional notification badge count */
  notificationBadge?: number
}

const props = withDefaults(defineProps<Props>(), {
  route: undefined,
  notificationBadge: undefined,
})

// Computed
const isClickable = computed(() => Boolean(props.route))
const iconComponent = computed(() => getIcon(props.icon))
const showNotificationBadge = computed(() =>
  props.notificationBadge !== undefined && props.notificationBadge > 0
)
</script>

<template>
  <component
    :is="route ? RouterLink : 'div'"
    :to="route"
    :class="['stat-card', { 'stat-card--clickable': isClickable }]"
    :aria-label="`${label}: ${count}`"
  >
    <div class="stat-card__icon">
      <component :is="iconComponent" v-if="iconComponent" />
      <NotificationBadge
        v-if="showNotificationBadge"
        :count="notificationBadge ?? 0"
      />
    </div>
    <div class="stat-card__content">
      <div class="stat-card__count">{{ count }}</div>
      <div class="stat-card__label">{{ label }}</div>
    </div>
  </component>
</template>

<style scoped>
.stat-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1.5rem;
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  transition: box-shadow 0.2s, transform 0.2s, border-color 0.2s;
  color: inherit;
  text-decoration: none;
}

.stat-card--clickable:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  border-color: var(--brand-primary);
}

.stat-card--clickable {
  cursor: pointer;
}

.stat-card--clickable:active {
  transform: translateY(-1px);
}

.stat-card__icon {
  position: relative;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  flex-shrink: 0;
  background-color: var(--brand-primary-light);
  color: var(--brand-primary);
}

.stat-card__icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.stat-card__content {
  flex: 1;
  min-width: 0;
}

.stat-card__count {
  font-size: 1.875rem;
  font-weight: 700;
  color: var(--brand-dark-2);
  line-height: 1;
  margin-bottom: 0.25rem;
}

.stat-card__label {
  font-size: 0.875rem;
  color: var(--brand-dark-1);
  line-height: 1.25;
}

/* Focus styles for accessibility */
.stat-card:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

/* Responsive adjustments */
@media (max-width: 640px) {
  .stat-card {
    padding: 1rem;
  }

  .stat-card__icon {
    width: 40px;
    height: 40px;
  }

  .stat-card__icon :deep(svg) {
    width: 20px;
    height: 20px;
  }

  .stat-card__count {
    font-size: 1.5rem;
  }

  .stat-card__label {
    font-size: 0.8125rem;
  }
}
</style>
