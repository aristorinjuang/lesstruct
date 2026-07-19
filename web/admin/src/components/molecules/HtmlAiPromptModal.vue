<script setup lang="ts">
import { ref } from 'vue'
import Button from '@/components/atoms/Button.vue'
import { HTML_AI_PRESETS } from '@/components/molecules/htmlAiPresets'

interface Props {
  isLoading: boolean
  hasExistingContent: boolean
  initialPrompt?: string
}

interface Emits {
  (e: 'generate', prompt: string): void
  (e: 'close'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const prompt = ref(props.initialPrompt || '')
const showPresets = ref(!props.initialPrompt)

function selectPreset(presetPrompt: string) {
  prompt.value = presetPrompt
  showPresets.value = false
}

function submit() {
  if (!prompt.value.trim() || props.isLoading) return
  emit('generate', prompt.value.trim())
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
    <div class="html-ai-modal" @keydown="handleKeydown">
      <div class="html-ai-modal__backdrop" @click="close"></div>
      <div class="html-ai-modal__dialog" role="dialog" aria-modal="true" aria-label="Generate HTML with AI">
        <div class="html-ai-modal__header">
          <h3 class="html-ai-modal__title">
            {{ hasExistingContent ? 'Refine HTML with AI' : 'Generate HTML with AI' }}
          </h3>
          <button class="html-ai-modal__close" @click="close" aria-label="Close">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
              <path d="M5 5l10 10M15 5l-10 10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </button>
        </div>

        <div v-if="showPresets" class="html-ai-modal__presets">
          <span class="html-ai-modal__presets-label">Quick start:</span>
          <div class="html-ai-modal__presets-grid">
            <button
              v-for="preset in HTML_AI_PRESETS"
              :key="preset.id"
              type="button"
              class="html-ai-modal__preset-chip"
              @click="selectPreset(preset.prompt)"
            >
              {{ preset.label }}
            </button>
          </div>
        </div>

        <div class="html-ai-modal__body">
          <label for="html-ai-prompt" class="html-ai-modal__label">
            {{ hasExistingContent
              ? 'Describe what changes you want — the AI will modify the existing HTML accordingly.'
              : 'Describe what you want — the AI will generate the HTML & CSS for you.' }}
          </label>
          <textarea
            id="html-ai-prompt"
            v-model="prompt"
            class="html-ai-modal__textarea"
            :placeholder="hasExistingContent
              ? 'e.g., Make the colors warmer and add a third column...'
              : 'e.g., A pricing section with three tiers, highlighted middle option...'"
            rows="4"
            autofocus
            @keydown.enter.meta="submit"
            @keydown.enter.ctrl="submit"
          ></textarea>
          <p class="html-ai-modal__hint">
            Press <kbd>Ctrl</kbd>+<kbd>Enter</kbd> to generate
          </p>
        </div>

        <div class="html-ai-modal__footer">
          <Button variant="secondary" @click="close" :disabled="isLoading">
            Cancel
          </Button>
          <Button
            variant="primary"
            :is-loading="isLoading"
            :disabled="!prompt.trim()"
            @click="submit"
          >
            {{ isLoading ? 'Generating...' : (hasExistingContent ? 'Refine' : 'Generate') }}
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.html-ai-modal {
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

.html-ai-modal__backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
}

.html-ai-modal__dialog {
  position: relative;
  background-color: var(--color-background);
  border-radius: 0.75rem;
  padding: 1.5rem;
  max-width: 560px;
  width: 90%;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
}

.html-ai-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}

.html-ai-modal__title {
  margin: 0;
  font-size: 1.125rem;
  color: var(--brand-dark-1);
}

.html-ai-modal__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border: none;
  background: none;
  cursor: pointer;
  color: var(--brand-dark-2);
  border-radius: 0.375rem;
  transition: background-color 150ms;
}

.html-ai-modal__close:hover {
  background-color: var(--color-background-mute);
}

.html-ai-modal__presets {
  margin-bottom: 1rem;
}

.html-ai-modal__presets-label {
  display: block;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--brand-dark-2);
  margin-bottom: 0.5rem;
}

.html-ai-modal__presets-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.html-ai-modal__preset-chip {
  display: inline-block;
  padding: 0.375rem 0.75rem;
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 9999px;
  background: none;
  font-size: 0.8125rem;
  color: var(--brand-dark-1);
  cursor: pointer;
  transition: border-color 150ms, background-color 150ms;
}

.html-ai-modal__preset-chip:hover {
  border-color: var(--brand-primary, #4f46e5);
  background-color: var(--color-background-mute);
}

.html-ai-modal__body {
  margin-bottom: 1.25rem;
}

.html-ai-modal__label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  margin-bottom: 0.5rem;
}

.html-ai-modal__textarea {
  display: block;
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 0.5rem;
  font-size: 0.9375rem;
  font-family: inherit;
  resize: vertical;
  min-height: 5rem;
  color: var(--brand-dark-1);
  background-color: var(--color-background);
  transition: border-color 150ms;
}

.html-ai-modal__textarea:focus {
  outline: none;
  border-color: var(--brand-primary, #4f46e5);
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.1);
}

.html-ai-modal__hint {
  margin: 0.375rem 0 0;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.html-ai-modal__hint kbd {
  display: inline-block;
  padding: 0.1rem 0.3rem;
  font-size: 0.6875rem;
  font-family: inherit;
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 0.25rem;
  background: var(--color-background-mute);
}

.html-ai-modal__footer {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}
</style>
