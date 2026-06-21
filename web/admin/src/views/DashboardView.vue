<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useDashboardStore } from '@/stores/domain/dashboard'
import { useNotificationStore } from '@/stores/ui/notifications'
import StatCard from '@/components/organisms/StatCard.vue'
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
      label: 'Published Posts',
      count: dashboardStore.stats.publishedPosts,
      icon: 'document-text',
      route: '/content?type=post&status=published',
    },
    {
      label: 'Draft Posts',
      count: dashboardStore.stats.draftPosts,
      icon: 'document',
      route: '/content?type=post&status=draft',
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
      <button
        type="button"
        class="dashboard-view__retry-button"
        @click="dashboardStore.fetchAll().catch(() => {})"
      >
        Retry
      </button>
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
  padding: 0.625rem 1.25rem;
  background-color: var(--color-destructive);
  color: var(--brand-dark-1);
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.dashboard-view__retry-button:hover {
  background-color: var(--color-destructive);
}

.dashboard-view__retry-button:focus-visible {
  outline: 2px solid var(--color-destructive);
  outline-offset: 2px;
}

.dashboard-view__alert {
  margin-bottom: 1.5rem;
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
