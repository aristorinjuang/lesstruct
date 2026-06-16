<script setup lang="ts">
import { computed } from 'vue'
import type { PendingRegistrationsAlertProps } from '@/types/dashboard'

const props = defineProps<PendingRegistrationsAlertProps>()

// Only show when there are pending registrations
const isVisible = computed(() => props.pendingCount > 0)

// Pluralization
const registrationText = computed(() =>
  props.pendingCount === 1 ? 'registration' : 'registrations'
)
</script>

<template>
  <RouterLink
    v-if="isVisible"
    to="/users?status=pending"
    class="pending-registrations-alert pending-registrations-alert--warning"
    role="alert"
    aria-live="polite"
  >
    <div class="pending-registrations-alert__content">
      <div class="pending-registrations-alert__icon">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
          stroke-width="1.5"
          stroke="currentColor"
          class="pending-registrations-alert__icon-svg"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
          />
        </svg>
      </div>
      <div class="pending-registrations-alert__text">
        <p class="pending-registrations-alert__message">
          <strong>{{ pendingCount }}</strong> pending {{ registrationText }} requires review.
        </p>
      </div>
      <div class="pending-registrations-alert__action">
        <span class="pending-registrations-alert__button">
          Review Registrations
        </span>
      </div>
    </div>
  </RouterLink>
</template>

<style scoped>
.pending-registrations-alert {
  display: block;
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  border-radius: 0.5rem;
  padding: 1rem;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s;
}

.pending-registrations-alert:hover {
  background-color: var(--color-warning-bg);
  border-color: var(--color-warning-border);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(245, 158, 11, 0.2);
}

.pending-registrations-alert:active {
  transform: translateY(0);
}

.pending-registrations-alert__content {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.pending-registrations-alert__icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.375rem;
  flex-shrink: 0;
  background-color: var(--color-warning-border);
  color: var(--color-warning-dark);
}

.pending-registrations-alert__icon-svg {
  width: 20px;
  height: 20px;
}

.pending-registrations-alert__text {
  flex: 1;
  min-width: 0;
}

.pending-registrations-alert__message {
  font-size: 0.9375rem;
  color: var(--color-warning-dark);
  margin: 0;
  line-height: 1.4;
}

.pending-registrations-alert__message strong {
  font-weight: 700;
  color: var(--color-warning-dark);
}

.pending-registrations-alert__action {
  flex-shrink: 0;
}

.pending-registrations-alert__button {
  display: inline-block;
  padding: 0.5rem 1rem;
  background-color: var(--color-warning-border);
  color: white;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background-color 0.2s;
}

.pending-registrations-alert:hover .pending-registrations-alert__button {
  background-color: var(--color-warning-border);
}

/* Focus styles for accessibility */
.pending-registrations-alert:focus-visible {
  outline: 2px solid var(--color-warning-border);
  outline-offset: 2px;
}

/* Responsive adjustments */
@media (max-width: 640px) {
  .pending-registrations-alert__content {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.625rem;
  }

  .pending-registrations-alert__action {
    width: 100%;
  }

  .pending-registrations-alert__button {
    display: block;
    text-align: center;
    width: 100%;
    box-sizing: border-box;
  }
}
</style>
