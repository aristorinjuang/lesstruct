<script setup lang="ts">
import { computed } from 'vue'
import type { OrderStatus } from '@/types/order'

const props = defineProps<{ status: OrderStatus | string }>()

const statusConfig = computed(() => {
  const map: Record<string, { classes: string; label: string }> = {
    pending: {
      classes: 'order-status-badge--pending',
      label: 'Pending',
    },
    paid: {
      classes: 'order-status-badge--paid',
      label: 'Paid',
    },
    packed: {
      classes: 'order-status-badge--packed',
      label: 'Packed',
    },
    shipped: {
      classes: 'order-status-badge--shipped',
      label: 'Shipped',
    },
    delivered: {
      classes: 'order-status-badge--delivered',
      label: 'Delivered',
    },
    cancelled: {
      classes: 'order-status-badge--cancelled',
      label: 'Cancelled',
    },
    refunded: {
      classes: 'order-status-badge--refunded',
      label: 'Refunded',
    },
    expired: {
      classes: 'order-status-badge--expired',
      label: 'Expired',
    },
  }

  const config = map[props.status] ?? {
    classes: 'order-status-badge--unknown',
    label: props.status,
  }

  return {
    classString: config.classes,
    label: config.label,
  }
})
</script>

<template>
  <span :class="['order-status-badge', statusConfig.classString]" role="status" aria-live="polite">
    {{ statusConfig.label }}
  </span>
</template>

<style scoped>
.order-status-badge {
  min-width: 90px;
  text-align: center;
  padding: 0.25rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
  display: inline-block;
}

.order-status-badge--pending {
  background-color: rgba(234, 179, 8, 0.1);
  color: #ca8a04;
}

.order-status-badge--paid {
  background-color: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}

.order-status-badge--packed {
  background-color: rgba(59, 130, 246, 0.1);
  color: #2563eb;
}

.order-status-badge--shipped {
  background-color: rgba(14, 165, 233, 0.1);
  color: #0284c7;
}

.order-status-badge--delivered {
  background-color: rgba(16, 185, 129, 0.1);
  color: #059669;
}

.order-status-badge--cancelled {
  background-color: rgba(239, 68, 68, 0.1);
  color: var(--color-danger);
}

.order-status-badge--refunded {
  background-color: rgba(168, 85, 247, 0.1);
  color: #9333ea;
}

.order-status-badge--expired {
  background-color: rgba(156, 163, 175, 0.1);
  color: var(--color-text-muted);
}

.order-status-badge--unknown {
  background-color: rgba(156, 163, 175, 0.1);
  color: var(--color-text-muted);
}
</style>
