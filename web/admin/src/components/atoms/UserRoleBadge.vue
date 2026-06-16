<script setup lang="ts">
import { computed } from 'vue'
import type { UserRole } from '@/types/user'

const props = defineProps<{ role: UserRole }>()

const roleClasses = computed(() => {
  const map: Record<string, string> = {
    Admin: 'user-role-badge--admin',
    Contributor: 'user-role-badge--contributor',
    Commentator: 'user-role-badge--commentator',
  }

  return map[props.role] ?? 'user-role-badge--unknown'
})
</script>

<template>
  <span :class="['user-role-badge', roleClasses]" role="status" aria-live="polite">
    {{ role }}
  </span>
</template>

<style scoped>
.user-role-badge {
  min-width: 70px;
  text-align: center;
  padding: 0.25rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
  display: inline-block;
}

.user-role-badge--admin {
  background-color: var(--brand-accent-light);
  color: var(--brand-accent);
}

.user-role-badge--contributor {
  background-color: var(--brand-primary-light);
  color: var(--brand-primary);
}

.user-role-badge--commentator {
  background-color: var(--color-info-bg);
  color: var(--color-info-dark);
}

.user-role-badge--unknown {
  background-color: var(--brand-light-1);
  color: var(--brand-dark-1);
  border: 1px solid var(--brand-light-2);
}
</style>
