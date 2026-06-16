<script setup lang="ts">
import { ref, computed } from 'vue'
import { type Media } from '@/stores/domain/media'

interface Props {
  media: Media
}

interface Emits {
  (e: 'insert', media: Media): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const thumbnailUrl = computed(() => {
  return props.media.variants?._thumb?.url || props.media.url
})

const imgError = ref(false)

function onImgError() {
  imgError.value = true
}

const isSelected = ref(false)
const isHovered = ref(false)

function handleClick() {
  isSelected.value = !isSelected.value
  emit('insert', props.media)
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    handleClick()
  }
}

function formatDimensions(width: number, height: number): string {
  return `${width}×${height}`
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  return date.toLocaleDateString()
}
</script>

<template>
  <div
    class="media-thumbnail"
    :class="{ 'media-thumbnail--selected': isSelected, 'media-thumbnail--hovered': isHovered }"
    :tabindex="0"
    role="button"
    :aria-label="`Insert ${media.originalFilename}`"
    :aria-selected="isSelected"
    @click="handleClick"
    @keydown="handleKeydown"
    @mouseenter="isHovered = true"
    @mouseleave="isHovered = false"
  >
    <div class="media-thumbnail__image-wrapper">
      <img
        :src="imgError ? props.media.url : thumbnailUrl"
        :alt="media.altText"
        class="media-thumbnail__image"
        loading="lazy"
        @error="onImgError"
      />
      <div v-if="isHovered || isSelected" class="media-thumbnail__overlay">
        <span class="media-thumbnail__overlay-text">Click to insert</span>
      </div>
    </div>
    <div class="media-thumbnail__info">
      <p class="media-thumbnail__filename" :title="media.originalFilename">
        {{ media.originalFilename }}
      </p>
      <p class="media-thumbnail__metadata">
        {{ formatDimensions(media.width, media.height) }}
      </p>
      <p class="media-thumbnail__date">
        {{ formatDate(media.createdAt) }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.media-thumbnail {
  cursor: pointer;
  border: 2px solid var(--brand-light-2);
  border-radius: 0.5rem;
  overflow: hidden;
  transition: all 0.2s ease-in-out;
  background-color: white;
}

.media-thumbnail:hover {
  border-color: var(--color-border-strong);
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.media-thumbnail:focus {
  outline: 2px solid var(--color-info);
  outline-offset: 2px;
}

.media-thumbnail--selected {
  border-color: var(--color-info);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

.media-thumbnail__image-wrapper {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background-color: var(--color-bg-muted);
}

.media-thumbnail__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.media-thumbnail__overlay {
  position: absolute;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: opacity 0.2s ease-in-out;
}

.media-thumbnail__overlay-text {
  color: white;
  font-size: 0.875rem;
  font-weight: 500;
}

.media-thumbnail__info {
  padding: 0.5rem;
}

.media-thumbnail__filename {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-2);
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.media-thumbnail__metadata {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin: 0.25rem 0 0;
}

.media-thumbnail__date {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin: 0;
}

@media (max-width: 768px) {
  .media-thumbnail {
    min-height: 48px;
  }

  .media-thumbnail__image-wrapper {
    aspect-ratio: 1 / 1;
  }
}
</style>
