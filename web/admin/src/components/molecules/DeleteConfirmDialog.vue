<script setup lang="ts">
import Button from '@/components/atoms/Button.vue'

interface Props {
  title: string
  itemName: string
  isOpen: boolean
  isLoading?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()

function confirm() {
  emit('confirm')
}

function cancel() {
  emit('cancel')
}
</script>

<template>
  <Teleport to="body">
    <div v-if="isOpen" class="delete-confirm">
      <div class="delete-confirm__backdrop" @click="cancel"></div>
      <div class="delete-confirm__dialog">
        <h3 class="delete-confirm__title">{{ title }}</h3>
        <p class="delete-confirm__warning">
          Are you sure you want to delete "{{ itemName }}"? This action cannot be undone.
        </p>
        <div class="delete-confirm__actions">
          <Button variant="secondary" @click="cancel" :disabled="isLoading">Cancel</Button>
          <Button variant="danger" @click="confirm" :is-loading="isLoading">Delete</Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.delete-confirm {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 2000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.delete-confirm__backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
}

.delete-confirm__dialog {
  position: relative;
  background-color: var(--color-background);
  border-radius: 0.5rem;
  padding: 1.5rem;
  max-width: 400px;
  width: 90%;
}

.delete-confirm__title {
  margin: 0 0 0.75rem;
  font-size: 1.125rem;
  color: var(--brand-dark-1);
}

.delete-confirm__warning {
  margin: 0 0 1.5rem;
  color: var(--brand-dark-2);
  font-size: 0.9375rem;
}

.delete-confirm__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}
</style>
