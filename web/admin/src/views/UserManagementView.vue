<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useUserStore } from '@/stores/domain/user'
import api from '@/utils/request'
import Toast from '@/components/molecules/Toast.vue'
import PendingRegistrations from '@/components/organisms/PendingRegistrations.vue'
import CreateUserModal from '@/components/organisms/CreateUserModal.vue'
import EditUserModal from '@/components/organisms/EditUserModal.vue'
import UserTable from '@/components/organisms/UserTable.vue'
import type { FieldSchema } from '@/types/customfield'
import type { UserFieldsResponse } from '@/types/posttype'

const userStore = useUserStore()
const showCreateModal = ref(false)
const showEditModal = ref(false)
const editingUserId = ref<string | null>(null)
const userFields = ref<FieldSchema[]>([])
const userSystemFields = ref<FieldSchema[]>([])

// Toast state
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

async function loadUsers() {
  try {
    await Promise.all([userStore.fetchUsers(), userStore.fetchPendingUsers()])
  } catch (error) {
    displayToast('Failed to load users', 'error')
    console.error('Error loading users:', error)
  }
}

async function loadUserFieldSchemas() {
  try {
    const response = await api.get<UserFieldsResponse>('/api/v1/user_fields')
    userFields.value = response.data.data?.fields || []
    userSystemFields.value = response.data.data?.systemFields || []
  } catch (error) {
    console.error('Error loading user field schemas:', error)
    userFields.value = []
    userSystemFields.value = []
  }
}

async function handleApprove(userId: string) {
  try {
    displayToast('Approving user...', 'success')
    await userStore.approveUser(userId)
    displayToast('User approved successfully')
  } catch (error) {
    displayToast('Failed to approve user', 'error')
    console.error('Error approving user:', error)
  }
}

async function handleReject(userId: string) {
  try {
    displayToast('Rejecting user...', 'success')
    await userStore.rejectUser(userId)
    displayToast('User rejected successfully')
  } catch (error) {
    displayToast('Failed to reject user', 'error')
    console.error('Error rejecting user:', error)
  }
}

async function handleMarkAsSpam(userId: string) {
  try {
    displayToast('Marking as spam...', 'success')
    await userStore.markAsSpam(userId)
    displayToast('User marked as spam and email blocked')
  } catch (error) {
    displayToast('Failed to mark as spam', 'error')
    console.error('Error marking as spam:', error)
  }
}

async function handleSuspend(userId: string) {
  try {
    displayToast('Suspending user...', 'success')
    await userStore.suspendUser(userId)
    displayToast('User suspended successfully')
  } catch (error) {
    displayToast('Failed to suspend user', 'error')
    console.error('Error suspending user:', error)
  }
}

async function handleSoftDelete(userId: string) {
  try {
    displayToast('Deleting user...', 'success')
    await userStore.softDeleteUser(userId)
    displayToast('User soft deleted successfully')
  } catch (error) {
    displayToast('Failed to soft delete user', 'error')
    console.error('Error soft deleting user:', error)
  }
}

function handleEditProfile(userId: string) {
  editingUserId.value = userId
  showEditModal.value = true
}

onMounted(() => {
  loadUsers()
  loadUserFieldSchemas()
})
</script>

<template>
  <div class="user-management">
    <header class="page-header--stacked">
      <h1 class="page-title">User Management</h1>
      <p class="page-subtitle">
        Manage user registrations, approve new users, and control system access.
      </p>
    </header>

    <!-- Toast notification -->
    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <!-- Loading state -->
    <div v-if="userStore.isLoading && userStore.users.length === 0" class="state-loading">
      Loading users...
    </div>

    <!-- Error state -->
    <div
      v-else-if="userStore.error && userStore.users.length === 0"
      class="user-management__error"
    >
      <p>Failed to load users. Please try again.</p>
      <button type="button" class="user-management__retry-button" @click="loadUsers">
        Retry
      </button>
    </div>

    <!-- Main content -->
    <div v-else class="user-management__content">
      <!-- Pending registrations section -->
      <PendingRegistrations
        :pending-users="userStore.pendingUsers"
        :is-loading="userStore.isPendingUsersLoading"
        @approve="handleApprove"
        @reject="handleReject"
        @mark-as-spam="handleMarkAsSpam"
      />

      <!-- All users table -->
      <section class="user-management__all-users">
        <div class="user-management__section-header">
          <h2 class="card-title">All Users</h2>
          <button type="button" class="user-management__create-button" @click="showCreateModal = true">
            Create User
          </button>
        </div>
        <UserTable
          :users="userStore.users"
          :user-fields="userFields"
          :user-system-fields="userSystemFields"
          :is-loading="userStore.isUsersLoading"
          @approve="handleApprove"
          @reject="handleReject"
          @mark-as-spam="handleMarkAsSpam"
          @suspend="handleSuspend"
          @soft-delete="handleSoftDelete"
          @edit-profile="handleEditProfile"
        />
      </section>

      <CreateUserModal
        :is-open="showCreateModal"
        :user-fields="userFields"
        :user-system-fields="userSystemFields"
        @close="showCreateModal = false"
        @created="displayToast('User created successfully')"
      />

      <EditUserModal
        v-if="editingUserId"
        :is-open="showEditModal"
        :user-id="editingUserId"
        :user-fields="userFields"
        :user-system-fields="userSystemFields"
        @close="showEditModal = false; editingUserId = null"
        @updated="displayToast('User updated successfully'); showEditModal = false; editingUserId = null"
      />
    </div>
  </div>
</template>

<style scoped>
.user-management__error {
  padding: 2rem;
  text-align: center;
  color: var(--color-error-dark);
}

.user-management__retry-button {
  margin-top: 1rem;
  padding: 0.625rem 1rem;
  background-color: var(--color-destructive);
  color: var(--color-white);
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
}

.user-management__retry-button:hover {
  background-color: var(--color-destructive);
}

.user-management__content {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.user-management__section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}

.user-management__create-button {
  padding: 0.5rem 1rem;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  min-height: 44px;
  transition: background-color 0.2s;
}

.user-management__create-button:hover {
  background-color: var(--color-interactive-hover);
}

.user-management__create-button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}
</style>
