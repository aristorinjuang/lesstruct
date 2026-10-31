<script setup lang="ts">
import { ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import MediaPanel from '@/components/organisms/MediaPanel.vue'
import type { Media } from '@/stores/domain/media'

interface Props {
  modelValue: string
  disabled?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: string): void
  (e: 'show-toast', message: string, type: string): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false
})

const emit = defineEmits<Emits>()

const isMediaPanelOpen = ref(false)

function toggleMediaPanel(): void {
  if (props.disabled) return
  isMediaPanelOpen.value = !isMediaPanelOpen.value
}

function setMedia(media: Media): void {
  emit('update:modelValue', media.url)
  isMediaPanelOpen.value = false
}

function clear(): void {
  if (props.disabled) return
  emit('update:modelValue', '')
}
</script>

<template>
  <div class="featured-image-field">
    <div v-if="modelValue" class="featured-image-field__preview">
      <img :src="modelValue" alt="Featured image preview" class="featured-image-field__img" />
      <div class="featured-image-field__actions">
        <Button
          size="small"
          variant="neutral"
          :disabled="disabled"
          @click="toggleMediaPanel"
        >
          {{ isMediaPanelOpen ? 'Hide media' : 'Change' }}
        </Button>
        <Button
          size="small"
          variant="danger"
          :disabled="disabled"
          @click="clear"
        >
          Remove
        </Button>
      </div>
    </div>
    <Button
      v-else
      size="small"
      variant="neutral"
      :disabled="disabled"
      @click="toggleMediaPanel"
    >
      {{ isMediaPanelOpen ? 'Hide media' : 'Set featured image' }}
    </Button>

    <MediaPanel
      :is-open="isMediaPanelOpen"
      @insert-image="setMedia"
      @show-toast="(msg: string, type: string) => emit('show-toast', msg, type)"
    />
  </div>
</template>

<style scoped>
.featured-image-field__preview {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.featured-image-field__img {
  width: 120px;
  height: 96px;
  object-fit: cover;
  border-radius: 0.375rem;
  border: 1px solid var(--color-border);
}

.featured-image-field__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
