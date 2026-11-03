<script setup lang="ts">
import { ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import InputText from '@/components/atoms/InputText.vue'
import TipTapEditor from '@/components/organisms/TipTapEditor.vue'

interface Props {
  transcript: string
  content: string
  suggestedTitle: string
  suggestedMetaDescription: string
  titleTaken: boolean
  metaDescriptionTaken: boolean
}

interface Emits {
  (e: 'insert', payload: { title: string; metaDescription: string }): void
  (e: 'close'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const title = ref(props.suggestedTitle)
const metaDescription = ref(props.suggestedMetaDescription)

function insert() {
  emit('insert', { title: title.value.trim(), metaDescription: metaDescription.value.trim() })
}

function close() {
  emit('close')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    close()
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="dictation-modal" @keydown="handleKeydown">
      <div class="dictation-modal__backdrop" @click="close"></div>
      <div class="dictation-modal__dialog" role="dialog" aria-modal="true" aria-label="Review dictation">
        <div class="dictation-modal__header">
          <h3 class="dictation-modal__title">
            Review dictation
          </h3>
          <IconButton class="dictation-modal__close" label="Close" @click="close">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
              <path d="M5 5l10 10M15 5l-10 10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </IconButton>
        </div>

        <div class="dictation-modal__body">
          <details class="dictation-modal__transcript">
            <summary class="dictation-modal__transcript-toggle">
              What you said
            </summary>
            <p class="dictation-modal__transcript-text">
              {{ transcript }}
            </p>
          </details>

          <div class="dictation-modal__preview-label">
            Formatted content
          </div>
          <div class="dictation-modal__preview">
            <TipTapEditor
              :model-value="content"
              :editable="false"
            />
          </div>

          <label for="dictation-title" class="dictation-modal__label">
            Title
          </label>
          <InputText
            id="dictation-title"
            :model-value="title"
            :disabled="titleTaken"
            :max-length="60"
            placeholder="AI-suggested title..."
            @update:model-value="title = $event"
          />
          <p v-if="titleTaken" class="dictation-modal__hint">
            Your current title is kept — the suggestion applies only to empty fields.
          </p>

          <label for="dictation-meta" class="dictation-modal__label">
            Meta description
          </label>
          <InputText
            id="dictation-meta"
            :model-value="metaDescription"
            :disabled="metaDescriptionTaken"
            :multiline="true"
            :max-length="160"
            placeholder="AI-suggested meta description..."
            @update:model-value="metaDescription = $event"
          />
          <p v-if="metaDescriptionTaken" class="dictation-modal__hint">
            Your current meta description is kept — the suggestion applies only to empty fields.
          </p>
        </div>

        <div class="dictation-modal__footer">
          <Button variant="neutral" @click="close">
            Cancel
          </Button>
          <Button variant="primary" @click="insert">
            Insert into content
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.dictation-modal {
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

.dictation-modal__backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
}

.dictation-modal__dialog {
  position: relative;
  background-color: var(--color-background);
  border-radius: 0.75rem;
  padding: 1.5rem;
  max-width: 640px;
  width: 90%;
  max-height: 85vh;
  overflow-y: auto;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
}

.dictation-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}

.dictation-modal__title {
  margin: 0;
  font-size: 1.125rem;
  color: var(--brand-dark-1);
}

.dictation-modal__close {
  flex-shrink: 0;
}

.dictation-modal__body {
  margin-bottom: 1.25rem;
}

.dictation-modal__transcript {
  margin-bottom: 1rem;
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 0.5rem;
  padding: 0.5rem 0.75rem;
}

.dictation-modal__transcript-toggle {
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.dictation-modal__transcript-text {
  margin: 0.5rem 0 0;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  white-space: pre-wrap;
}

.dictation-modal__preview-label,
.dictation-modal__label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  margin-bottom: 0.5rem;
}

.dictation-modal__preview {
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 0.5rem;
  margin-bottom: 1rem;
  max-height: 16rem;
  overflow-y: auto;
}

.dictation-modal__hint {
  margin: 0.375rem 0 1rem;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.dictation-modal__footer {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}
</style>
