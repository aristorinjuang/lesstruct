<script setup lang="ts">
import { ref, computed } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { html } from '@codemirror/lang-html'
import { oneDark } from '@codemirror/theme-one-dark'
import Button from '@/components/atoms/Button.vue'

interface Props {
  modelValue: string
  placeholder?: string
}

interface Emits {
  (e: 'update:modelValue', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  placeholder: '',
})

const emit = defineEmits<Emits>()

const extensions = [html(), oneDark]

const isPreviewOpen = ref(false)

const sanitizedContent = computed(() => {
  const raw = props.modelValue || ''
  const cleaned = raw
    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
    .replace(/\bon\w+\s*=/gi, 'data-blocked-')

  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <div class="content-body">${cleaned}</div>
</body>
</html>`
})

function onUpdate(value: string) {
  emit('update:modelValue', value)
}

function togglePreview() {
  isPreviewOpen.value = !isPreviewOpen.value
}
</script>

<template>
  <div class="html-code-editor">
    <div class="html-code-editor__toolbar">
      <Button
        type="button"
        variant="neutral"
        size="small"
        @click="togglePreview"
      >
        {{ isPreviewOpen ? 'Show Code' : 'Show Preview' }}
      </Button>
    </div>
    <div class="html-code-editor__main">
      <div v-if="!isPreviewOpen" class="html-code-editor__code">
        <Codemirror
          :model-value="modelValue"
          :placeholder="placeholder"
          :extensions="extensions"
          :style="{ minHeight: '60vh' }"
          @update:model-value="onUpdate"
        />
      </div>
      <div v-else class="html-code-editor__preview">
        <iframe
          :srcdoc="sanitizedContent"
          sandbox="allow-same-origin"
          class="html-code-editor__iframe"
        ></iframe>
      </div>
    </div>
  </div>
</template>

<style scoped>
.html-code-editor__toolbar {
  margin-bottom: 0.5rem;
}

.html-code-editor__main {
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  overflow: hidden;
}

.html-code-editor__code {
  overflow: auto;
}

.html-code-editor__preview {
  background-color: var(--color-background);
}

.html-code-editor__iframe {
  width: 100%;
  min-height: 60vh;
  border: none;
}
</style>
