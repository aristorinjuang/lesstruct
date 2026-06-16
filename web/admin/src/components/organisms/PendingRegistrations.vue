<script setup lang="ts">
import { formatDateTime } from '@/utils/date'
import { useConfirmationDialog } from '@/composables/useConfirmationDialog'
import type { PendingRegistrationsProps } from '@/types/user'
import UserActions from '@/components/molecules/UserActions.vue'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'

const props = withDefaults(defineProps<PendingRegistrationsProps>(), {
  isLoading: false,
})

const emit = defineEmits<{
  approve: [userId: string]
  reject: [userId: string]
  markAsSpam: [userId: string]
}>()

const { confirmationDialog, showDialog, handleConfirm, handleCancel } =
  useConfirmationDialog()

function handleApprove(userId: string) {
  emit('approve', userId)
}

function handleReject(userId: string) {
  showDialog(
    'Reject User',
    'Are you sure you want to reject this user? They can re-register.',
    'Reject',
    (id) => emit('reject', id),
    userId
  )
}

function handleMarkAsSpam(userId: string) {
  showDialog(
    'Mark as Spam',
    'Are you sure you want to mark this as spam? This email address will be blocked.',
    'Mark as Spam',
    (id) => emit('markAsSpam', id),
    userId
  )
}
</script>

<template>
  <section v-if="pendingUsers.length > 0" class="pending-registrations">
    <h2 class="pending-registrations__title">Pending Registrations</h2>

    <div v-if="isLoading" class="pending-registrations__loading">
      Loading pending registrations...
    </div>

    <div v-else class="pending-registrations__list">
      <div
        v-for="user in pendingUsers"
        :key="user.id"
        class="pending-registrations__card"
      >
        <div class="pending-registrations__user-info">
          <h3 class="pending-registrations__username">{{ user.username }}</h3>
          <p class="pending-registrations__email">{{ user.email }}</p>
          <p class="pending-registrations__date">
            Registered: {{ formatDateTime(user.createdAt) }}
          </p>
        </div>

        <UserActions
          :user-status="user.status"
          @approve="handleApprove(user.id)"
          @reject="handleReject(user.id)"
          @mark-as-spam="handleMarkAsSpam(user.id)"
        />
      </div>
    </div>

    <ConfirmationDialog
      :is-open="confirmationDialog.isOpen"
      :title="confirmationDialog.title"
      :message="confirmationDialog.message"
      :confirm-button-text="confirmationDialog.confirmButtonText"
      @confirm="handleConfirm"
      @cancel="handleCancel"
    />
  </section>
</template>

<style scoped>
.pending-registrations {
  background-color: var(--brand-accent-light);
  border: 1px solid var(--brand-accent);
  border-radius: 0.5rem;
  padding: 1.5rem;
  margin-bottom: 2rem;
}

.pending-registrations__title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--brand-accent-hover);
  margin: 0 0 1rem 0;
}

.pending-registrations__loading {
  text-align: center;
  color: var(--brand-accent-hover);
  padding: 2rem;
}

.pending-registrations__list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.pending-registrations__card {
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 1rem;
  box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.pending-registrations__user-info {
  flex: 1;
}

.pending-registrations__username {
  font-size: 1rem;
  font-weight: 600;
  color: var(--brand-dark-1);
  margin: 0 0 0.25rem 0;
}

.pending-registrations__email {
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  margin: 0 0 0.25rem 0;
}

.pending-registrations__date {
  font-size: 0.75rem;
  color: var(--brand-dark-2);
  margin: 0;
}

/* Responsive adjustments */
@media (min-width: 768px) {
  .pending-registrations__card {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }
}
</style>
