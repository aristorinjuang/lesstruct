<script setup lang="ts">
import { getIcon } from '@/components/icons'
import { formatRelativeTime } from '@/utils/date'
import type { ActivityItem } from '@/types/dashboard'

interface Props {
  activities: ActivityItem[]
}

defineProps<Props>()

function getActivityIconName(type: ActivityItem['type']): string {
  switch (type) {
    case 'post_published':
      return 'document-text'
    case 'user_registered':
      return 'user'
    case 'comment_added':
      return 'chat-bubble'
    default:
      return 'document'
  }
}

function getActivityDescription(activity: ActivityItem): string {
  switch (activity.type) {
    case 'post_published':
      return `Published by ${activity.actor}`
    case 'user_registered':
      return 'User registered'
    case 'comment_added':
      return `Comment by ${activity.actor}`
    default:
      return ''
  }
}
</script>

<template>
  <section class="recent-activity" aria-labelledby="recent-activity-title">
    <h2 id="recent-activity-title" class="recent-activity__title">Recent Activity</h2>

    <div v-if="activities.length === 0" class="recent-activity__empty">
      <p class="recent-activity__empty-text">No recent activity to display.</p>
    </div>

    <ul v-else class="recent-activity__list">
      <li
        v-for="activity in activities"
        :key="`${activity.type}-${activity.date}-${activity.title}`"
        class="recent-activity__item"
      >
        <div class="recent-activity__icon">
          <component :is="getIcon(getActivityIconName(activity.type))" />
        </div>
        <div class="recent-activity__content">
          <p class="recent-activity__title">{{ activity.title }}</p>
          <p class="recent-activity__meta">
            {{ getActivityDescription(activity) }}
            <span class="recent-activity__separator">&bull;</span>
            <time :datetime="activity.date" class="recent-activity__date">
              {{ formatRelativeTime(activity.date) }}
            </time>
          </p>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.recent-activity {
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 1.5rem;
}

.recent-activity__title {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 1rem 0;
  color: var(--brand-dark-1);
}

.recent-activity__empty {
  text-align: center;
  padding: 2rem 0;
}

.recent-activity__empty-text {
  color: var(--brand-dark-2);
  margin: 0;
}

.recent-activity__list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.recent-activity__item {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--brand-light-2);
}

.recent-activity__item:last-child {
  border-bottom: none;
}

.recent-activity__icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.375rem;
  flex-shrink: 0;
  background-color: var(--brand-light-1);
  color: var(--brand-dark-2);
}

.recent-activity__icon :deep(svg) {
  width: 16px;
  height: 16px;
}

.recent-activity__content {
  flex: 1;
  min-width: 0;
}

.recent-activity__title {
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  margin: 0 0 0.25rem 0;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-activity__meta {
  font-size: 0.8125rem;
  color: var(--brand-dark-2);
  margin: 0;
  line-height: 1.4;
}

.recent-activity__separator {
  margin: 0 0.375rem;
}

.recent-activity__date {
  color: var(--brand-dark-2);
  opacity: 0.75;
}

/* Responsive adjustments */
@media (max-width: 640px) {
  .recent-activity {
    padding: 1rem;
  }

  .recent-activity__title {
    font-size: 1.125rem;
  }

  .recent-activity__item {
    padding: 0.625rem 0;
  }
}
</style>
