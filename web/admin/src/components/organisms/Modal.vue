<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useBreakpoint } from '@/composables/useBreakpoint'

export interface ModalProps {
  isOpen?: boolean
  title?: string
  closeOnOverlayClick?: boolean
  closeOnEscape?: boolean
}

const props = withDefaults(defineProps<ModalProps>(), {
  isOpen: false,
  closeOnOverlayClick: true,
  closeOnEscape: true,
})

const emit = defineEmits<{
  close: []
}>()

const { isSmallMobile } = useBreakpoint()

/**
 * Touch gesture state for swipe-to-dismiss on mobile
 */
const touchStartY = ref(0)
const currentTranslateY = ref(0)
const isDragging = ref(false)

/**
 * Computed class for bottom sheet behavior on mobile
 */
const isBottomSheet = computed(() => isSmallMobile.value)

/**
 * Computed style for bottom sheet drag
 */
const bottomSheetStyle = computed(() => {
  if (!isBottomSheet.value) return {}

  return {
    transform: `translateY(${currentTranslateY.value}px)`,
  }
})

/**
 * Close the modal
 */
function close() {
  emit('close')
}

/**
 * Handle overlay click
 */
function handleOverlayClick() {
  if (props.closeOnOverlayClick) {
    close()
  }
}

/**
 * Handle escape key
 */
function handleEscape(event: KeyboardEvent) {
  if (props.closeOnEscape && event.key === 'Escape' && props.isOpen) {
    close()
  }
}

/**
 * Handle touch start for swipe gesture
 */
function handleTouchStart(event: TouchEvent) {
  if (!isBottomSheet.value) return

  const touch = event.touches[0]
  if (touch) {
    touchStartY.value = touch.clientY
    isDragging.value = true
  }
}

/**
 * Handle touch move for swipe gesture
 */
function handleTouchMove(event: TouchEvent) {
  if (!isBottomSheet.value || !isDragging.value) return

  const touch = event.touches[0]
  if (touch) {
    const currentY = touch.clientY
    const deltaY = currentY - touchStartY.value

    // Only allow dragging downward
    if (deltaY > 0) {
      event.preventDefault()
      currentTranslateY.value = deltaY
    }
  }
}

/**
 * Handle touch end for swipe gesture
 */
function handleTouchEnd() {
  if (!isBottomSheet.value || !isDragging.value) return

  const SWIPE_THRESHOLD = 100

  if (currentTranslateY.value > SWIPE_THRESHOLD) {
    // Swipe threshold exceeded - close modal
    close()
  }

  // Reset
  currentTranslateY.value = 0
  isDragging.value = false
}

/**
 * Lock body scroll when modal is open
 */
watch(
  () => props.isOpen,
  (newVal) => {
    if (newVal) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    // Reset drag state
    currentTranslateY.value = 0
    isDragging.value = false
    touchStartY.value = 0
  }
)

/**
 * Manage escape key listener and touch event listeners
 */
onMounted(() => {
  document.addEventListener('keydown', handleEscape)
  document.addEventListener('touchmove', handleTouchMove, { passive: false })
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  document.removeEventListener('touchmove', handleTouchMove)
  document.body.style.overflow = ''
})
</script>

<template>
  <transition name="modal-fade">
    <div v-if="isOpen" class="modal__overlay" @click="handleOverlayClick">
      <div
        :class="['modal__container', { 'modal__container--bottom-sheet': isBottomSheet }]"
        :style="bottomSheetStyle"
        @click.stop
        @touchstart="handleTouchStart"
        @touchend="handleTouchEnd"
      >
        <!-- Drag handle for mobile bottom sheet -->
        <div v-if="isBottomSheet" class="modal__drag-handle"></div>

        <!-- Modal header (optional) -->
        <div v-if="title" class="modal__header">
          <h2 class="modal__title">{{ title }}</h2>
        </div>

        <!-- Modal content -->
        <div class="modal__content">
          <slot />
        </div>

        <!-- Modal footer (optional) -->
        <div v-if="$slots.footer" class="modal__footer">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.modal__overlay {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 60;
  padding: 16px;
}

.modal__container {
  background-color: var(--color-background);
  border-radius: 8px;
  max-width: 600px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.modal__container--bottom-sheet {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  top: auto;
  border-radius: 16px 16px 0 0;
  max-height: 85vh;
  transition: transform 0.3s ease;
}

.modal__drag-handle {
  width: 40px;
  height: 4px;
  background-color: var(--brand-light-2);
  border-radius: 2px;
  margin: 8px auto;
  flex-shrink: 0;
}

.modal__header {
  padding: 16px 16px 8px;
  flex-shrink: 0;
}

.modal__title {
  font-size: 18px;
  font-weight: 600;
  color: var(--brand-dark-2);
  margin: 0;
}

.modal__content {
  padding: 16px;
  flex: 1;
  overflow-y: auto;
}

.modal__footer {
  padding: 8px 16px 16px;
  border-top: 1px solid var(--brand-light-2);
  flex-shrink: 0;
}

/* Modal animations */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-from .modal__container,
.modal-fade-leave-to .modal__container {
  transform: scale(0.95);
}

/* Mobile bottom sheet specific styles */
@media (max-width: 639px) {
  .modal__overlay {
    padding: 0;
    align-items: flex-end;
  }

  .modal__container {
    border-radius: 16px 16px 0 0;
  }

  .modal__container:not(.modal__container--bottom-sheet) {
    width: 90%;
  }

  /* Bottom sheet slide animation overrides the default scale */
  .modal-fade-enter-from .modal__container--bottom-sheet {
    transform: translateY(100%);
  }

  .modal-fade-leave-to .modal__container--bottom-sheet {
    transform: translateY(100%);
  }
}
</style>
