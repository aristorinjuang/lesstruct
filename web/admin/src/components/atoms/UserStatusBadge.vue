<script setup lang="ts">
import { computed } from 'vue'
import type { UserStatus } from '@/types/user'

const props = defineProps<{ status: UserStatus }>()

const statusConfig = computed(() => {
  const map: Record<string, { classes: string; label: string }> = {
    Pending: {
      classes: 'user-status-badge--pending',
      label: 'Pending',
    },
    Active: {
      classes: 'user-status-badge--active',
      label: 'Active',
    },
    Suspended: {
      classes: 'user-status-badge--suspended',
      label: 'Suspended',
    },
    SoftDeleted: {
      classes: 'user-status-badge--soft-deleted',
      label: 'Soft Deleted',
    },
  }

  const config = map[props.status] ?? {
    classes: 'user-status-badge--unknown',
    label: props.status,
  }

  return {
    classString: config.classes,
    label: config.label,
  }
})
</script>

<template>
  <span
    :class="['user-status-badge', statusConfig.classString]"
    role="status"
    aria-live="polite"
  >
    {{ statusConfig.label }}
  </span>
</template>

<style scoped>
.user-status-badge {
  min-width: 90px;
  text-align: center;
  padding: 0.25rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
  display: inline-block;
}

.user-status-badge--pending {
  background-color: rgba(234, 179, 8, 0.1);
  color: #ca8a04;
}

.user-status-badge--active {
  background-color: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.user-status-badge--suspended {
  background-color: rgba(249, 115, 22, 0.1);
  color: #ea580c;
}

.user-status-badge--soft-deleted {
  background-color: rgba(156, 163, 175, 0.1);
  color: var(--color-text-muted);
}

.user-status-badge--unknown {
  background-color: rgba(156, 163, 175, 0.1);
  color: var(--color-text-muted);
}
</style>
