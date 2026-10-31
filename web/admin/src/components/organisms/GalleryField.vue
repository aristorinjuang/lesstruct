<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import MediaPanel from '@/components/organisms/MediaPanel.vue'
import type { Media } from '@/stores/domain/media'
import type { GalleryItem } from '@/types/customfield'

interface Props {
  modelValue: GalleryItem[] | string
  disabled?: boolean
  maxItems?: number
}

interface Emits {
  (e: 'update:modelValue', value: GalleryItem[]): void
  (e: 'show-toast', message: string, type: string): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  maxItems: undefined
})

const emit = defineEmits<Emits>()

const isMediaPanelOpen = ref(false)

function toggleMediaPanel(): void {
  if (props.disabled) return
  isMediaPanelOpen.value = !isMediaPanelOpen.value
}

function coerceGallery(value: unknown): GalleryItem[] {
  if (typeof value === 'string') {
    return value
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line !== '')
      .map((url) => ({ url, alt: '' }))
  }
  if (!Array.isArray(value)) return []
  const out: GalleryItem[] = []
  for (const entry of value as unknown[]) {
    if (typeof entry === 'string') {
      const url = entry.trim()
      if (url !== '') out.push({ url, alt: '' })
      continue
    }
    if (entry && typeof entry === 'object') {
      const record = entry as Record<string, unknown>
      const rawUrl = record['url']
      if (typeof rawUrl !== 'string') continue
      const url = rawUrl.trim()
      if (url === '') continue
      const rawAlt = record['alt']
      out.push({ url, alt: typeof rawAlt === 'string' ? rawAlt : '' })
    }
  }
  return out
}

const normalized = computed(() => coerceGallery(props.modelValue))

const atLimit = computed(() => props.maxItems !== undefined && normalized.value.length >= props.maxItems)

function addMedia(media: Media): void {
  if (atLimit.value) {
    emit('show-toast', `Maximum of ${props.maxItems} images reached.`, 'error')
    return
  }
  emit('update:modelValue', [...normalized.value, { url: media.url, alt: media.altText ?? '' }])
}

function updateAlt(index: number, alt: string): void {
  if (props.disabled) return
  emit(
    'update:modelValue',
    normalized.value.map((item, i) => (i === index ? { ...item, alt } : item))
  )
}

function removeItem(index: number): void {
  if (props.disabled) return
  emit('update:modelValue', normalized.value.filter((_, i) => i !== index))
}

function move(index: number, delta: number): void {
  if (props.disabled) return
  const target = index + delta
  if (target < 0 || target >= normalized.value.length) return
  const next = [...normalized.value]
  ;[next[index], next[target]] = [next[target], next[index]]
  emit('update:modelValue', next)
}
</script>

<template>
  <div class="gallery-field">
    <ul v-if="normalized.length" class="gallery-field__items">
      <li v-for="(item, index) in normalized" :key="`${item.url}-${index}`" class="gallery-field__item">
        <img :src="item.url" :alt="item.alt" class="gallery-field__thumb" loading="lazy" />
        <div class="gallery-field__body">
          <input
            :value="item.alt"
            type="text"
            class="gallery-field__alt"
            placeholder="Alt text"
            :disabled="disabled"
            @input="updateAlt(index, ($event.target as HTMLInputElement).value)"
          />
          <div class="gallery-field__actions">
            <button
              type="button"
              class="gallery-field__btn"
              :disabled="disabled || index === 0"
              aria-label="Move up"
              @click="move(index, -1)"
            >
              ↑
            </button>
            <button
              type="button"
              class="gallery-field__btn"
              :disabled="disabled || index === normalized.length - 1"
              aria-label="Move down"
              @click="move(index, 1)"
            >
              ↓
            </button>
            <button
              type="button"
              class="gallery-field__btn gallery-field__btn--remove"
              :disabled="disabled"
              aria-label="Remove"
              @click="removeItem(index)"
            >
              ×
            </button>
          </div>
        </div>
      </li>
    </ul>
    <p v-else class="gallery-field__empty">No images yet.</p>

    <Button
      size="small"
      variant="neutral"
      :disabled="disabled || atLimit"
      @click="toggleMediaPanel"
    >
      {{ isMediaPanelOpen ? 'Hide media' : 'Add images' }}
    </Button>
    <p v-if="atLimit" class="gallery-field__limit">Maximum of {{ maxItems }} images reached.</p>

    <MediaPanel
      :is-open="isMediaPanelOpen"
      @insert-image="addMedia"
      @show-toast="(msg: string, type: string) => emit('show-toast', msg, type)"
    />
  </div>
</template>

<style scoped>
.gallery-field__items {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.gallery-field__item {
  display: flex;
  gap: 0.5rem;
  padding: 0.5rem;
  border: 1px solid var(--btn-neutral-border);
  border-radius: 0.375rem;
  background: var(--btn-neutral-bg);
}

.gallery-field__thumb {
  width: 64px;
  height: 64px;
  object-fit: cover;
  border-radius: 0.375rem;
  flex-shrink: 0;
}

.gallery-field__body {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  flex: 1;
}

.gallery-field__alt {
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  font-size: 0.875rem;
}

.gallery-field__actions {
  display: flex;
  gap: 0.25rem;
}

.gallery-field__btn {
  border: none;
  background: transparent;
  color: var(--color-heading);
  cursor: pointer;
  padding: 0 0.25rem;
}

.gallery-field__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.gallery-field__btn--remove {
  color: var(--color-error);
}

.gallery-field__empty {
  color: var(--color-text);
  font-size: 0.875rem;
  margin: 0 0 0.5rem;
}

.gallery-field__limit {
  color: var(--color-error);
  font-size: 0.75rem;
  margin: 0.25rem 0 0;
}
</style>
