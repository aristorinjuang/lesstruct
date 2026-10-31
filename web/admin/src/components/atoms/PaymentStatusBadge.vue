<script setup lang="ts">
import { computed } from 'vue'
import type { PaymentStatus } from '@/types/payment'

const props = defineProps<{ status: PaymentStatus | string }>()

const statusConfig = computed(() => {
  const map: Record<string, { classes: string; label: string }> = {
    pending: {
      classes: 'payment-status-badge--pending',
      label: 'Pending',
    },
    verified: {
      classes: 'payment-status-badge--verified',
      label: 'Verified',
    },
    rejected: {
      classes: 'payment-status-badge--rejected',
      label: 'Rejected',
    },
  }
  return (
    map[props.status] ?? {
      classes: 'payment-status-badge--unknown',
      label: props.status,
    }
  )
})
</script>

<template>
  <span class="payment-status-badge" :class="statusConfig.classes">
    {{ statusConfig.label }}
  </span>
</template>

<style scoped>
.payment-status-badge {
  display: inline-block;
  padding: 0.125rem 0.625rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
}

.payment-status-badge--pending {
  background-color: rgba(234, 179, 8, 0.15);
  color: #a16207;
}

.payment-status-badge--verified {
  background-color: rgba(34, 197, 94, 0.15);
  color: #15803d;
}

.payment-status-badge--rejected {
  background-color: rgba(239, 68, 68, 0.15);
  color: #b91c1c;
}

.payment-status-badge--unknown {
  background-color: rgba(148, 163, 184, 0.15);
  color: var(--color-text-muted);
}
</style>
