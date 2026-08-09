<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useDashboardStore } from '@/stores/domain/dashboard'
import { useNotificationStore } from '@/stores/ui/notifications'
import StatCard from '@/components/organisms/StatCard.vue'
import Button from '@/components/atoms/Button.vue'
import PendingRegistrationsAlert from '@/components/organisms/PendingRegistrationsAlert.vue'

// Stores
const dashboardStore = useDashboardStore()
const notificationStore = useNotificationStore()

// Fetch data on mount
onMounted(async () => {
  try {
    await dashboardStore.fetchAll()
    notificationStore.syncFromDashboard(dashboardStore.stats?.pendingRegistrations ?? 0)
  } catch (error) {
    console.error('Failed to load dashboard data:', error)
  }
})

// Computed properties for stats cards
const statsCards = computed(() => {
  if (!dashboardStore.stats) return []

  return [
    {
      label: 'Published Content',
      count: dashboardStore.stats.publishedPosts,
      icon: 'document-text',
      route: '/content?status=published',
    },
    {
      label: 'Draft Content',
      count: dashboardStore.stats.draftPosts,
      icon: 'document',
      route: '/content?status=draft',
    },
    {
      label: 'Registered Users',
      count: dashboardStore.stats.registeredUsers,
      icon: 'users',
      route: '/users',
    },
    {
      label: 'Pending Registrations',
      count: dashboardStore.stats.pendingRegistrations,
      icon: 'user',
      route: '/users?status=pending',
      notificationBadge: dashboardStore.stats.pendingRegistrations,
    },
    {
      label: 'Media Items',
      count: dashboardStore.stats.mediaItems,
      icon: 'photo',
      route: '/media',
    },
  ]
})

const postTypeLabels: Record<string, string> = {
  post: 'Posts',
  page: 'Pages',
  'menu-item': 'Menu Items',
}

function titleCasePostType(slug: string): string {
  return slug
    .split('-')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}

const contentByTypeCards = computed(() => {
  const entries = dashboardStore.stats?.contentByType ?? []
  return entries.map(entry => ({
    label: postTypeLabels[entry.postType] ?? titleCasePostType(entry.postType),
    count: entry.count,
    icon: entry.postType === 'post' ? 'document-text' : 'document',
    route: `/content?type=${entry.postType}`,
  }))
})
</script>

<template>
  <div class="dashboard-view">
    <div class="page-header--stacked">
      <h1 class="page-title">Dashboard</h1>
    </div>

    <!-- Loading state -->
    <div v-if="dashboardStore.isLoading && !dashboardStore.stats" class="state-loading">
      <div class="dashboard-view__spinner" aria-hidden="true"></div>
      <p>Loading dashboard...</p>
    </div>

    <!-- Error state -->
    <div
      v-else-if="dashboardStore.error && !dashboardStore.stats"
      class="dashboard-view__error"
    >
      <p class="dashboard-view__error-text">
        Failed to load dashboard data. Please try again.
      </p>
      <Button
        type="button"
        variant="danger"
        class="dashboard-view__retry-button"
        @click="dashboardStore.fetchAll().catch(() => {})"
      >
        Retry
      </Button>
    </div>

    <!-- Dashboard content -->
    <template v-else>
      <!-- Statistics cards grid -->
      <div class="dashboard-stats">
        <StatCard
          v-for="card in statsCards"
          :key="card.label"
          :label="card.label"
          :count="card.count"
          :icon="card.icon"
          :route="card.route"
          :notification-badge="card.notificationBadge"
        />
      </div>

      <!-- Pending registrations alert -->
      <PendingRegistrationsAlert
        v-if="dashboardStore.stats"
        :pending-count="dashboardStore.stats.pendingRegistrations"
        class="dashboard-view__alert"
      />

      <!-- Content by type -->
      <section v-if="contentByTypeCards.length > 0" class="dashboard-content-types">
        <h2 class="dashboard-content-types__title">Content by Type</h2>
        <div class="dashboard-stats">
          <StatCard
            v-for="card in contentByTypeCards"
            :key="card.label"
            :label="card.label"
            :count="card.count"
            :icon="card.icon"
            :route="card.route"
          />
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
/* Loading state */
.state-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem;
  gap: 1rem;
}

.dashboard-view__spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--brand-light-2);
  border-top-color: var(--color-destructive);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* Error state */
.dashboard-view__error {
  text-align: center;
  padding: 4rem 2rem;
  background-color: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.3);
  border-radius: 0.5rem;
}

.dashboard-view__error-text {
  font-size: 1rem;
  color: var(--color-error);
  margin: 0 0 1rem 0;
}

.dashboard-view__retry-button {
  margin-top: 0.25rem;
}

.dashboard-view__alert {
  margin-bottom: 1.5rem;
}

.dashboard-content-types {
  margin-top: 0.5rem;
}

.dashboard-content-types__title {
  font-size: 1.125rem;
  margin: 0 0 1rem 0;
  color: var(--brand-dark-1);
}

/* Statistics grid - responsive layout */
.dashboard-stats {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

/* Tablet (768px - 1023px) */
@media (min-width: 768px) {
  .dashboard-stats {
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }
}

/* Desktop (1024px+) */
@media (min-width: 1024px) {
  .dashboard-stats {
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }
}

/* Responsive adjustments */
@media (max-width: 640px) {
  .dashboard-stats {
    gap: 0.75rem;
  }
}
</style>
