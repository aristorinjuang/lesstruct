<script setup lang="ts">
import { computed, ref } from 'vue'

interface Props {
  modelValue: string[]
  disabled?: boolean
  maxLength?: number
  maxItems?: number
  placeholder?: string
}

interface Emits {
  (e: 'update:modelValue', value: string[]): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  maxLength: undefined,
  maxItems: undefined,
  placeholder: 'Add an item'
})

const emit = defineEmits<Emits>()

const draft = ref('')

const atLimit = computed(() => props.maxItems !== undefined && props.modelValue.length >= props.maxItems)
const canAdd = computed(() => {
  const value = draft.value.trim()
  return !props.disabled && value !== '' && !atLimit.value
})

function addItem(): void {
  if (!canAdd.value) return
  emit('update:modelValue', [...props.modelValue, draft.value.trim()])
  draft.value = ''
}

function removeItem(index: number): void {
  if (props.disabled) return
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== index))
}

function move(index: number, delta: number): void {
  if (props.disabled) return
  const target = index + delta
  if (target < 0 || target >= props.modelValue.length) return
  const next = [...props.modelValue]
  ;[next[index], next[target]] = [next[target], next[index]]
  emit('update:modelValue', next)
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter') {
    event.preventDefault()
    addItem()
  }
}
</script>

<template>
  <div class="list-input">
    <ul v-if="modelValue.length" class="list-input__items">
      <li v-for="(item, index) in modelValue" :key="`${item}-${index}`" class="list-input__item">
        <span class="list-input__text">{{ item }}</span>
        <span class="list-input__actions">
          <button
            type="button"
            class="list-input__btn"
            :disabled="disabled || index === 0"
            aria-label="Move up"
            @click="move(index, -1)"
          >
            ↑
          </button>
          <button
            type="button"
            class="list-input__btn"
            :disabled="disabled || index === modelValue.length - 1"
            aria-label="Move down"
            @click="move(index, 1)"
          >
            ↓
          </button>
          <button
            type="button"
            class="list-input__btn list-input__btn--remove"
            :disabled="disabled"
            aria-label="Remove"
            @click="removeItem(index)"
          >
            ×
          </button>
        </span>
      </li>
    </ul>
    <p v-else class="list-input__empty">No items yet.</p>

    <div class="list-input__add">
      <input
        v-model="draft"
        type="text"
        class="list-input__field"
        :placeholder="placeholder"
        :disabled="disabled || atLimit"
        :maxlength="maxLength"
        @keydown="onKeydown"
      />
      <button
        type="button"
        class="list-input__add-btn"
        :disabled="!canAdd"
        @click="addItem"
      >
        Add
      </button>
    </div>
    <p v-if="atLimit" class="list-input__limit">Maximum of {{ maxItems }} items reached.</p>
  </div>
</template>

<style scoped>
.list-input__items {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.list-input__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--btn-neutral-border);
  border-radius: 0.375rem;
  background: var(--btn-neutral-bg);
}

.list-input__text {
  overflow-wrap: anywhere;
  font-size: 0.875rem;
}

.list-input__actions {
  display: flex;
  gap: 0.25rem;
  flex-shrink: 0;
}

.list-input__btn {
  border: none;
  background: transparent;
  color: var(--color-heading);
  cursor: pointer;
  padding: 0 0.25rem;
  line-height: 1.4;
}

.list-input__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.list-input__btn--remove {
  color: var(--color-error);
}

.list-input__empty {
  color: var(--color-text);
  font-size: 0.875rem;
  margin: 0 0 0.5rem;
}

.list-input__add {
  display: flex;
  gap: 0.5rem;
}

.list-input__field {
  flex: 1;
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  font-size: 0.875rem;
}

.list-input__add-btn {
  padding: 0.25rem 0.75rem;
  border: 1px solid var(--btn-neutral-border);
  border-radius: 0.375rem;
  background: var(--btn-neutral-bg);
  color: var(--btn-neutral-fg);
  cursor: pointer;
  font-size: 0.875rem;
}

.list-input__add-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.list-input__limit {
  color: var(--color-error);
  font-size: 0.75rem;
  margin: 0.25rem 0 0;
}
</style>
