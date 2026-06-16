<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { getIcon } from '@/components/icons'
import { useBreakpoint } from '@/composables/useBreakpoint'

export interface ToastProps {
  id?: string
  message: string
  type?: 'success' | 'error' | 'warning' | 'info'
  duration?: number
  visible?: boolean
}

const props = withDefaults(defineProps<ToastProps>(), {
  id: () => `toast-${Date.now()}-${Math.random()}`,
  type: 'info',
  duration: 5000,
  visible: true,
})

const emit = defineEmits<{
  dismiss: [id: string]
}>()

const { isSmallMobile } = useBreakpoint()

/**
 * Internal visibility state
 */
const isVisible = ref(props.visible)

/**
 * Computed class for toast styling based on type
 */
const toastClass = computed(() => `toast--${props.type}`)

/**
 * Check if we're on mobile (< 640px)
 */
const isMobile = computed(() => isSmallMobile.value)

/**
 * Handle dismiss action
 */
function dismiss() {
  isVisible.value = false
  emit('dismiss', props.id)
}

/**
 * Auto-dismiss after duration
 */
let dismissTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.visible,
  (newValue) => {
    if (dismissTimer) clearTimeout(dismissTimer)
    isVisible.value = newValue
    if (newValue && props.duration > 0) {
      dismissTimer = setTimeout(() => {
        dismiss()
        dismissTimer = null
      }, props.duration)
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  if (dismissTimer) clearTimeout(dismissTimer)
})

// Expose dismiss method for parent components
defineExpose({
  dismiss,
})
</script>

<template>
  <transition name="toast-slide">
    <div
      v-if="isVisible"
      :class="['toast', toastClass, { 'toast--full-width': isMobile }]"
      role="status"
      aria-live="polite"
    >
      <span class="toast__message">{{ message }}</span>
      <button
        class="toast__close"
        @click="dismiss"
        :aria-label="`Dismiss ${type} notification`"
        type="button"
      >
        <component :is="getIcon('x-mark')" class="toast__close-icon" />
      </button>
    </div>
  </transition>
</template>

<style scoped>
.toast {
  position: fixed;
  top: 16px;
  right: 16px;
  padding: 16px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  gap: 12px;
  z-index: 100;
  min-width: 300px;
  max-width: 500px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.toast--full-width {
  left: 16px;
  right: 16px;
  min-width: auto;
  max-width: none;
}

.toast--success {
  background-color: var(--color-success-bg);
  border: 1px solid var(--color-success-border);
  color: var(--color-success-dark);
}

.toast--error {
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  color: var(--color-error-dark);
}

.toast--warning {
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-bg);
  color: var(--color-warning-dark);
}

.toast--info {
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-bg);
  color: var(--color-info-dark);
}

.toast__message {
  flex: 1;
  font-size: 14px;
  line-height: 1.5;
}

.toast__close {
  min-width: 44px;
  min-height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  transition: background-color 0.2s;
  flex-shrink: 0;
}

.toast__close:hover {
  background-color: rgba(0, 0, 0, 0.05);
}

.toast__close:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 2px;
}

.toast__close-icon {
  width: 20px;
  height: 20px;
}

/* Toast animations */
.toast-slide-enter-active,
.toast-slide-leave-active {
  transition: all 0.3s ease;
}

.toast-slide-enter-from {
  opacity: 0;
  transform: translateY(-20px);
}

.toast-slide-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}
</style>
