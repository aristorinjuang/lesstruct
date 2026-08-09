<script setup lang="ts">
import type { UserActionsProps } from '@/types/user'
import Button from '@/components/atoms/Button.vue'

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
      <Button
        type="button"
        variant="subtle"
        tone="success"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('approve')"
        aria-label="Approve user"
      >
        Approve
      </Button>
      <Button
        type="button"
        variant="subtle"
        tone="danger"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('reject')"
        aria-label="Reject user"
      >
        Reject
      </Button>
      <Button
        type="button"
        variant="subtle"
        tone="warning"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('markAsSpam')"
        aria-label="Mark as spam"
      >
        Mark as Spam
      </Button>
    </template>

    <!-- Active user actions -->
    <template v-else-if="userStatus === 'Active' || userStatus === 'verified'">
      <Button
        type="button"
        variant="subtle"
        tone="warning"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('suspend')"
        aria-label="Suspend user"
      >
        Suspend
      </Button>
      <Button
        type="button"
        variant="subtle"
        tone="neutral"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('softDelete')"
        aria-label="Soft delete user"
      >
        Soft Delete
      </Button>
      <Button
        type="button"
        variant="subtle"
        tone="info"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('editProfile')"
        aria-label="Edit user profile"
      >
        Edit Profile
      </Button>
    </template>

    <!-- Suspended user actions -->
    <template v-else-if="userStatus === 'Suspended' || userStatus === 'suspended'">
      <Button
        type="button"
        variant="subtle"
        tone="neutral"
        class="user-actions__button"
        :disabled="disabled"
        @click="emit('softDelete')"
        aria-label="Soft delete user"
      >
        Soft Delete
      </Button>
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
