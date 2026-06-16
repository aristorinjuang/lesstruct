<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import Link from '@tiptap/extension-link'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import { Table } from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableCell from '@tiptap/extension-table-cell'
import TableHeader from '@tiptap/extension-table-header'
import { Mathematics } from '@tiptap/extension-mathematics'
import { Emoji } from '@tiptap/extension-emoji'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import 'highlight.js/styles/github-dark.css'
import 'katex/dist/katex.min.css'
import { Youtube } from './TipTapYoutube'
import EditorToolbar from '@/components/molecules/EditorToolbar.vue'

const lowlight = createLowlight(common)

interface Props {
  modelValue: string
  placeholder?: string
}

interface Emits {
  (e: 'update:modelValue', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  placeholder: 'Start writing...',
})

const emit = defineEmits<Emits>()

// Reactive undo/redo state
const canUndo = ref(false)
const canRedo = ref(false)

// Parse the initial content (plain const — only used once at setup)
function parseInitialContent(value: string) {
  try {
    if (value) {
      return JSON.parse(value)
    }
  } catch {
    // If it's not valid JSON, treat it as plain text
    if (value) {
      return {
        type: 'doc',
        content: [
          {
            type: 'paragraph',
            content: [{ type: 'text', text: value }],
          },
        ],
      }
    }
  }
  return {
    type: 'doc',
    content: [{ type: 'paragraph' }],
  }
}

// Guard to prevent circular updates
let isUpdatingFromProp = false

const editor = useEditor({
  content: parseInitialContent(props.modelValue),
  extensions: [
    StarterKit.configure({
      heading: {
        levels: [1, 2, 3],
      },
      codeBlock: false,
    }),
    CodeBlockLowlight.configure({
      lowlight,
      HTMLAttributes: {
        class: 'tiptap-code-block',
      },
    }),
    Underline,
    Link.configure({
      openOnClick: false,
      HTMLAttributes: {
        class: 'text-blue-600 underline hover:text-blue-800',
      },
      autolink: true,
      linkOnPaste: true,
    }),
    Image.configure({
      inline: true,
      allowBase64: false,
      HTMLAttributes: {
        class: 'max-w-full h-auto',
      },
    }),
    Placeholder.configure({
      placeholder: props.placeholder,
      emptyEditorClass: 'is-editor-empty',
    }),
    Table.configure({
      resizable: true,
    }),
    TableRow,
    TableCell,
    TableHeader,
    Mathematics.configure({
      katexOptions: {
        throwOnError: false,
        displayMode: false,
      },
    }),
    Emoji,
    Youtube,
  ],
  editorProps: {
    attributes: {
      class: 'tiptap-editor-content',
    },
  },
  onUpdate: ({ editor }) => {
    if (isUpdatingFromProp) return
    emit('update:modelValue', JSON.stringify(editor.getJSON()))
  },
  onTransaction: ({ editor }) => {
    canUndo.value = editor.can().undo()
    canRedo.value = editor.can().redo()
  },
})

// Watch for external changes
watch(
  () => props.modelValue,
  (newValue) => {
    if (editor.value) {
      const currentJson = JSON.stringify(editor.value.getJSON())
      if (currentJson !== newValue) {
        isUpdatingFromProp = true
        try {
          const parsed = JSON.parse(newValue)
          editor.value.commands.setContent(parsed)
        } catch {
          // If it's not valid JSON, ignore
        }
        isUpdatingFromProp = false
      }
    }
  },
)

onBeforeUnmount(() => {
  editor.value?.destroy()
})

// Expose editor instance for parent components
defineExpose({
  editor,
})
</script>

<template>
  <div class="tiptap-editor">
    <EditorToolbar
      v-if="editor"
      :editor="editor"
      :can-undo="canUndo"
      :can-redo="canRedo"
    />

    <EditorContent
      v-if="editor"
      :editor="editor"
      class="tiptap-editor-content-wrapper"
    />
  </div>
</template>

<style scoped>
.tiptap-editor {
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  overflow: hidden;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content) {
  min-height: 300px;
  padding: 1rem;
  outline: none;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content:focus) {
  outline: none;
  box-shadow: inset 0 0 0 2px rgba(59, 130, 246, 0.3);
  border-radius: 2px;
}

/* Placeholder styling */
.tiptap-editor-content-wrapper :deep(.is-editor-empty):before {
  content: attr(data-placeholder);
  float: left;
  color: var(--brand-dark-2);
  opacity: 0.5;
  pointer-events: none;
  height: 0;
}

/* Basic prose styling */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content) > * + * {
  margin-top: 0.75em;
}

/* Headings */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content h1) {
  font-size: 1.875rem;
  font-weight: 700;
  line-height: 1.2;
  margin-top: 1rem;
  margin-bottom: 0.5rem;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content h2) {
  font-size: 1.5rem;
  font-weight: 600;
  line-height: 1.3;
  margin-top: 0.875rem;
  margin-bottom: 0.5rem;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content h3) {
  font-size: 1.25rem;
  font-weight: 600;
  line-height: 1.4;
  margin-top: 0.75rem;
  margin-bottom: 0.5rem;
}

/* Paragraphs */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content p) {
  font-size: 1rem;
  line-height: 1.6;
}

/* Lists */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content ul),
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content ol) {
  padding-left: 1.5rem;
  margin: 0.5rem 0;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content li) {
  margin: 0.25rem 0;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content ul) {
  list-style-type: disc;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content ol) {
  list-style-type: decimal;
}

/* Blockquote */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content blockquote) {
  border-left: 4px solid var(--brand-light-2);
  padding-left: 1rem;
  margin: 1rem 0;
  font-style: italic;
  color: var(--brand-dark-2);
}

/* Code block */
.tiptap-editor-content-wrapper :deep(.tiptap-code-block) {
  background-color: var(--color-bg-inverse);
  color: var(--brand-light-1);
  padding: 1rem;
  border-radius: 0.375rem;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.875rem;
  overflow-x: auto;
  margin: 1rem 0;
}

/* Inline code */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content code) {
  padding: 0.125rem 0.25rem;
  border-radius: 0.25rem;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.875em;
}

/* Links */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content a) {
  color: var(--color-info);
  text-decoration: underline;
  cursor: pointer;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content a:hover) {
  color: var(--color-info);
}

/* Images */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content img) {
  max-width: 100%;
  height: auto;
  border-radius: 0.375rem;
  margin: 1rem 0;
}

/* Horizontal rule */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content hr) {
  border: none;
  border-top: 1px solid var(--brand-light-2);
  margin: 2rem 0;
}

/* Strong and emphasis */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content strong) {
  font-weight: 700;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content em) {
  font-style: italic;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content u) {
  text-decoration: underline;
}

/* Tables */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content table) {
  border-collapse: collapse;
  margin: 1rem 0;
  width: auto;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content th),
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content td) {
  border: 1px solid var(--brand-light-2);
  padding: 0.5rem 0.75rem;
  min-width: 80px;
  position: relative;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content th) {
  background-color: var(--brand-light-1);
  font-weight: 600;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content .selectedCell) {
  background-color: var(--brand-primary-light);
}

/* YouTube embed */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content .youtube-wrapper) {
  position: relative;
  padding-bottom: 56.25%;
  height: 0;
  overflow: hidden;
  margin: 1rem 0;
  background-color: #000;
}

.tiptap-editor-content-wrapper :deep(.tiptap-editor-content .youtube-wrapper iframe) {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  border: none;
}

/* Selection */
.tiptap-editor-content-wrapper :deep(.tiptap-editor-content ::selection) {
  background-color: var(--color-info-bg);
}

/* KaTeX math rendering */
.tiptap-editor-content-wrapper :deep(.tiptap-mathematics-render) {
  display: inline;
  padding: 0.125rem 0.25rem;
}

.tiptap-editor-content-wrapper :deep(.tiptap-mathematics-render[data-type='block-math']) {
  display: block;
  margin: 1rem 0;
  text-align: center;
}

.tiptap-editor-content-wrapper :deep(.tiptap-mathematics-render--editable) {
  cursor: pointer;
  border-radius: 0.25rem;
  transition: background-color 0.15s;
}

.tiptap-editor-content-wrapper :deep(.tiptap-mathematics-render--editable:hover) {
  background-color: var(--brand-light-1);
}

.tiptap-editor-content-wrapper :deep(.katex) {
  font-size: 1.1em;
}

.tiptap-editor-content-wrapper :deep(.katex-display) {
  margin: 0;
}
</style>
