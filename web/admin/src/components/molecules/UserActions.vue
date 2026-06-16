<script setup lang="ts">
import type { UserActionsProps } from '@/types/user'

defineProps<UserActionsProps>()

const emit = defineEmits<{
  approve: []
  reject: []
  markAsSpam: []
  suspend: []
  softDelete: []
  editProfile: []
}>()
</script>

<template>
  <div class="user-actions">
    <!-- Pending user actions -->
    <template v-if="userStatus === 'Pending' || userStatus === 'pending'">
      <button
        type="button"
        class="user-actions__button user-actions__button--approve"
        :disabled="disabled"
        @click="emit('approve')"
        aria-label="Approve user"
      >
        Approve
      </button>
      <button
        type="button"
        class="user-actions__button user-actions__button--reject"
        :disabled="disabled"
        @click="emit('reject')"
        aria-label="Reject user"
      >
        Reject
      </button>
      <button
        type="button"
        class="user-actions__button user-actions__button--spam"
        :disabled="disabled"
        @click="emit('markAsSpam')"
        aria-label="Mark as spam"
      >
        Mark as Spam
      </button>
    </template>

    <!-- Active user actions -->
    <template v-else-if="userStatus === 'Active' || userStatus === 'verified'">
      <button
        type="button"
        class="user-actions__button user-actions__button--suspend"
        :disabled="disabled"
        @click="emit('suspend')"
        aria-label="Suspend user"
      >
        Suspend
      </button>
      <button
        type="button"
        class="user-actions__button user-actions__button--delete"
        :disabled="disabled"
        @click="emit('softDelete')"
        aria-label="Soft delete user"
      >
        Soft Delete
      </button>
      <button
        type="button"
        class="user-actions__button user-actions__button--edit"
        :disabled="disabled"
        @click="emit('editProfile')"
        aria-label="Edit user profile"
      >
        Edit Profile
      </button>
    </template>

    <!-- Suspended user actions -->
    <template v-else-if="userStatus === 'Suspended' || userStatus === 'suspended'">
      <button
        type="button"
        class="user-actions__button user-actions__button--delete"
        :disabled="disabled"
        @click="emit('softDelete')"
        aria-label="Soft delete user"
      >
        Soft Delete
      </button>
    </template>

    <!-- Soft Deleted users - no actions -->
  </div>
</template>

<style scoped>
.user-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.user-actions__button {
  padding: 0.25rem 0.5rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
  border: 1px solid transparent;
}

.user-actions__button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.user-actions__button--approve {
  background-color: var(--color-success-bg);
  color: var(--color-success-dark);
}

.user-actions__button--approve:hover:not(:disabled) {
  background-color: var(--color-success-border);
}

.user-actions__button--reject {
  background-color: var(--color-error-bg);
  color: var(--color-error-dark);
}

.user-actions__button--reject:hover:not(:disabled) {
  background-color: var(--color-error-border);
}

.user-actions__button--spam {
  background-color: var(--color-warning-bg);
  color: var(--color-warning-dark);
}

.user-actions__button--spam:hover:not(:disabled) {
  background-color: var(--color-warning-bg);
}

.user-actions__button--suspend {
  background-color: #fed7aa;
  color: #9a3412;
}

.user-actions__button--suspend:hover:not(:disabled) {
  background-color: #fdba74;
}

.user-actions__button--delete {
  background-color: var(--brand-light-2);
  color: var(--color-text-secondary);
}

.user-actions__button--delete:hover:not(:disabled) {
  background-color: var(--color-border-strong);
}

.user-actions__button--edit {
  background-color: var(--color-info-bg);
  color: var(--color-info-dark);
}

.user-actions__button--edit:hover:not(:disabled) {
  background-color: var(--color-info-bg);
}

.user-actions__button:focus-visible {
  outline: 2px solid var(--color-destructive);
  outline-offset: 2px;
}

/* Responsive adjustments */
@media (max-width: 639px) {
  .user-actions {
    flex-direction: column;
  }

  .user-actions__button {
    width: 100%;
    padding: 0.5rem 0.75rem;
  }
}
</style>
