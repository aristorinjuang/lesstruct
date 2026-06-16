<script setup lang="ts">
import Modal from './Modal.vue'
import type { ConfirmationDialogProps } from '@/types/user'

const props = withDefaults(defineProps<ConfirmationDialogProps>(), {
  confirmButtonText: 'Confirm',
  cancelButtonText: 'Cancel',
})

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

function handleConfirm() {
  emit('confirm')
}

function handleCancel() {
  emit('cancel')
}
</script>

<template>
  <Modal :is-open="isOpen" :title="title" @close="handleCancel">
    <div class="confirmation-dialog">
      <p class="confirmation-dialog__message">{{ message }}</p>

      <div class="confirmation-dialog__actions">
        <button
          type="button"
          class="confirmation-dialog__button confirmation-dialog__button--cancel"
          @click="handleCancel"
          aria-label="Cancel action"
        >
          {{ cancelButtonText }}
        </button>
        <button
          type="button"
          class="confirmation-dialog__button confirmation-dialog__button--confirm"
          @click="handleConfirm"
          :aria-label="confirmButtonText"
        >
          {{ confirmButtonText }}
        </button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.confirmation-dialog {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.confirmation-dialog__message {
  margin: 0;
  color: var(--brand-dark-1);
  line-height: 1.5;
  font-size: 0.9375rem;
}

.confirmation-dialog__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}

.confirmation-dialog__button {
  padding: 0.625rem 1rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
  border: 1px solid transparent;
  min-height: 44px;
  min-width: 80px;
}

.confirmation-dialog__button--cancel {
  background-color: var(--brand-light-1);
  color: var(--brand-dark-2);
  border-color: var(--brand-light-2);
}

.confirmation-dialog__button--cancel:hover {
  background-color: var(--brand-light-2);
}

.confirmation-dialog__button--confirm {
  background-color: var(--color-destructive);
  color: var(--color-white);
}

.confirmation-dialog__button--confirm:hover {
  background-color: var(--color-destructive);
}

.confirmation-dialog__button:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

/* Responsive adjustments */
@media (max-width: 639px) {
  .confirmation-dialog__actions {
    flex-direction: column-reverse;
  }

  .confirmation-dialog__button {
    width: 100%;
  }
}
</style>
