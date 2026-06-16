<script setup lang="ts">
import { formatDate } from '@/utils/date'
import { useConfirmationDialog } from '@/composables/useConfirmationDialog'
import type { UserTableProps } from '@/types/user'
import type { FieldSchema } from '@/types/customfield'
import UserStatusBadge from '@/components/atoms/UserStatusBadge.vue'
import UserRoleBadge from '@/components/atoms/UserRoleBadge.vue'
import UserActions from '@/components/molecules/UserActions.vue'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'

const props = withDefaults(defineProps<UserTableProps>(), {
  isLoading: false,
  userFields: () => [],
  userSystemFields: () => [],
})

const emit = defineEmits<{
  approve: [userId: string]
  reject: [userId: string]
  markAsSpam: [userId: string]
  suspend: [userId: string]
  softDelete: [userId: string]
  editProfile: [userId: string]
}>()

const { confirmationDialog, showDialog, handleConfirm, handleCancel } =
  useConfirmationDialog()

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function formatFieldValue(field: FieldSchema, value: any): string {
  if (value === undefined || value === null || value === '') return '—'
  switch (field.type) {
    case 'checkbox': return value ? 'Yes' : 'No'
    case 'date': return formatDate(String(value))
    default: return String(value)
  }
}

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

function handleSuspend(userId: string) {
  showDialog(
    'Suspend User',
    'Are you sure you want to suspend this user?',
    'Suspend',
    (id) => emit('suspend', id),
    userId
  )
}

function handleSoftDelete(userId: string) {
  showDialog(
    'Soft Delete User',
    'Are you sure you want to soft delete this user? This action can be reversed.',
    'Soft Delete',
    (id) => emit('softDelete', id),
    userId
  )
}

function handleEditProfile(userId: string) {
  emit('edit-profile', userId)
}
</script>

<template>
  <div class="user-table">
    <div v-if="isLoading && users.length === 0" class="user-table__loading">
      Loading users...
    </div>

    <div v-else-if="users.length === 0" class="user-table__empty">
      No users found.
    </div>

    <div v-else class="user-table__container" :class="{ 'user-table__container--loading': isLoading && users.length > 0 }">
      <table class="user-table__table">
        <thead>
          <tr>
            <th scope="col" class="user-table__avatar-col"></th>
            <th scope="col">Username</th>
            <th scope="col">Name</th>
            <th scope="col">Email</th>
            <th v-for="field in userFields" :key="'th-' + field.slug" scope="col">{{ field.name }}</th>
            <th v-for="field in userSystemFields" :key="'th-sys-' + field.slug" scope="col" class="user-table__system-header">{{ field.name }}</th>
            <th scope="col">Role</th>
            <th scope="col">Status</th>
            <th scope="col">Registration Date</th>
            <th scope="col">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td class="user-table__avatar-cell">
              <img
                v-if="user.profilePicture"
                :src="user.profilePicture"
                :alt="user.username"
                class="user-table__avatar-img"
              />
              <div v-else class="user-table__avatar-placeholder">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M20 21a8 8 0 1 0-16 0"/></svg>
              </div>
            </td>
            <td class="user-table__username">{{ user.username }}</td>
            <td class="user-table__name">{{ user.name || '—' }}</td>
            <td class="user-table__email">{{ user.email }}</td>
            <td v-for="field in userFields" :key="'td-' + field.slug" class="user-table__field">
              {{ formatFieldValue(field, user.customFields?.[field.slug]) }}
            </td>
            <td v-for="field in userSystemFields" :key="'td-sys-' + field.slug" class="user-table__field user-table__field--system">
              {{ formatFieldValue(field, user.customFields?.[field.slug]) }}
            </td>
            <td class="user-table__role">
              <UserRoleBadge :role="user.role" />
            </td>
            <td class="user-table__status">
              <UserStatusBadge :status="user.status" />
            </td>
            <td class="user-table__date">{{ formatDate(user.createdAt) }}</td>
            <td class="user-table__actions">
              <UserActions
                :user-status="user.status"
                @approve="handleApprove(user.id)"
                @reject="handleReject(user.id)"
                @mark-as-spam="handleMarkAsSpam(user.id)"
                @suspend="handleSuspend(user.id)"
                @soft-delete="handleSoftDelete(user.id)"
                @edit-profile="handleEditProfile(user.id)"
              />
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="isLoading && users.length > 0" class="user-table__loading-overlay">
        <div class="user-table__spinner"></div>
        <span>Refreshing...</span>
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
  </div>
</template>

<style scoped>
.user-table {
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  overflow: hidden;
}

.user-table__loading,
.user-table__empty {
  padding: 2rem;
  text-align: center;
  color: var(--brand-dark-2);
}

.user-table__container {
  overflow-x: auto;
}

.user-table__table {
  width: 100%;
  border-collapse: collapse;
  min-width: 800px;
}

.user-table__table th {
  background-color: var(--brand-light-1);
  padding: 0.75rem 1rem;
  text-align: left;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--brand-dark-2);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--brand-light-2);
}

.user-table__table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--brand-light-2);
  color: var(--brand-dark-1);
}

.user-table__table tbody tr:last-child td {
  border-bottom: none;
}

.user-table__table tbody tr:hover {
  background-color: var(--brand-light-1);
}

.user-table__avatar-col {
  width: 44px;
}

.user-table__avatar-cell {
  padding: 0.375rem 0.5rem;
  vertical-align: middle;
}

.user-table__avatar-img {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
}

.user-table__avatar-placeholder {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brand-dark-2);
}

.user-table__username {
  font-weight: 500;
  color: var(--brand-dark-1);
}

.user-table__name {
  color: var(--brand-dark-1);
}

.user-table__email {
  color: var(--brand-dark-2);
  font-size: 0.875rem;
}

.user-table__field {
  color: var(--brand-dark-2);
  font-size: 0.875rem;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-table__field--system {
  font-style: italic;
  opacity: 0.85;
}

.user-table__system-header {
  font-style: italic;
}

.user-table__date {
  color: var(--brand-dark-2);
  font-size: 0.875rem;
  white-space: nowrap;
}

.user-table__actions {
  padding: 0.375rem 0;
}

.user-table__container--loading {
  position: relative;
}

.user-table__loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--color-background);
  opacity: 0.9;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  z-index: 10;
}

.user-table__loading-overlay span {
  color: var(--brand-dark-1);
  font-size: 0.875rem;
}

.user-table__spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--brand-light-2);
  border-top-color: var(--brand-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* Responsive adjustments */
@media (max-width: 767px) {
  .user-table__container {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }

  .user-table__table {
    min-width: 700px;
  }

  .user-table__table th,
  .user-table__table td {
    padding: 0.625rem 0.75rem;
    font-size: 0.8125rem;
  }
}
</style>
