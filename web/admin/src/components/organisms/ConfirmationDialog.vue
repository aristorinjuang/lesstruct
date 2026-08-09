<script setup lang="ts">
import Modal from './Modal.vue'
import Button from '@/components/atoms/Button.vue'
import type { ConfirmationDialogProps } from '@/types/user'

withDefaults(defineProps<ConfirmationDialogProps>(), {
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
        <Button
          type="button"
          variant="neutral"
          class="confirmation-dialog__button"
          @click="handleCancel"
          aria-label="Cancel action"
        >
          {{ cancelButtonText }}
        </Button>
        <Button
          type="button"
          variant="danger"
          class="confirmation-dialog__button"
          @click="handleConfirm"
          :aria-label="confirmButtonText"
        >
          {{ confirmButtonText }}
        </Button>
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
  min-width: 80px;
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
