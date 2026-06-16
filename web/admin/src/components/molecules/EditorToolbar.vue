<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { type Editor } from '@tiptap/vue-3'
import EditorButton from '@/components/atoms/EditorButton.vue'

type HeadingLevel = 1 | 2 | 3 | 4 | 5 | 6

interface Props {
  editor: Editor
  canUndo: boolean
  canRedo: boolean
}

const props = defineProps<Props>()

const showLinkDialog = ref(false)
const linkUrl = ref('')
const showImageDialog = ref(false)
const imageUrl = ref('')
const linkInputRef = ref<HTMLInputElement | null>(null)
const imageInputRef = ref<HTMLInputElement | null>(null)

const showTableDialog = ref(false)
const tableRows = ref(3)
const tableCols = ref(3)
const tableRowsRef = ref<HTMLInputElement | null>(null)

const showYoutubeDialog = ref(false)
const youtubeUrl = ref('')
const youtubeInputRef = ref<HTMLInputElement | null>(null)

const showMathDialog = ref(false)
const mathLatex = ref('')
const mathMode = ref<'inline' | 'block'>('inline')
const mathInputRef = ref<HTMLInputElement | null>(null)

function isValidUrl(url: string): boolean {
  if (!url) return false
  const trimmed = url.trim()
  if (trimmed.startsWith('javascript:')) return false
  if (trimmed.startsWith('data:')) return false
  if (trimmed.startsWith('vbscript:')) return false
  return true
}

function setLink() {
  if (linkUrl.value && isValidUrl(linkUrl.value)) {
    props.editor.chain().focus().setLink({ href: linkUrl.value.trim() }).run()
  }
  showLinkDialog.value = false
  linkUrl.value = ''
}

function setImage() {
  if (imageUrl.value && isValidUrl(imageUrl.value)) {
    props.editor.chain().focus().setImage({ src: imageUrl.value.trim() }).run()
  }
  showImageDialog.value = false
  imageUrl.value = ''
}

function removeLink() {
  props.editor.chain().focus().unsetLink().run()
}

function openLinkDialog() {
  showLinkDialog.value = true
  linkUrl.value = ''
  nextTick(() => {
    linkInputRef.value?.focus()
  })
}

function openImageDialog() {
  showImageDialog.value = true
  imageUrl.value = ''
  nextTick(() => {
    imageInputRef.value?.focus()
  })
}

function closeLinkDialog() {
  showLinkDialog.value = false
  linkUrl.value = ''
}

function closeImageDialog() {
  showImageDialog.value = false
  imageUrl.value = ''
}

function openTableDialog() {
  showTableDialog.value = true
  tableRows.value = 3
  tableCols.value = 3
  nextTick(() => {
    tableRowsRef.value?.focus()
  })
}

function insertTable() {
  const rows = Math.max(1, Math.min(10, tableRows.value))
  const cols = Math.max(1, Math.min(10, tableCols.value))
  props.editor
    .chain()
    .focus()
    .insertTable({ rows, cols, withHeaderRow: true })
    .run()
  showTableDialog.value = false
}

function closeTableDialog() {
  showTableDialog.value = false
}

function openYoutubeDialog() {
  showYoutubeDialog.value = true
  youtubeUrl.value = ''
  nextTick(() => {
    youtubeInputRef.value?.focus()
  })
}

function insertYoutube() {
  if (youtubeUrl.value) {
    props.editor
      .chain()
      .focus()
      .insertYoutube({ src: youtubeUrl.value.trim() })
      .run()
  }
  showYoutubeDialog.value = false
  youtubeUrl.value = ''
}

function openMathDialog(mode: 'inline' | 'block') {
  showMathDialog.value = true
  mathMode.value = mode
  mathLatex.value = ''
  nextTick(() => {
    mathInputRef.value?.focus()
  })
}

function insertMath() {
  if (mathLatex.value) {
    const trimmed = mathLatex.value.trim()
    if (trimmed) {
      if (mathMode.value === 'inline') {
        props.editor.chain().focus().insertInlineMath({ latex: trimmed }).run()
      } else {
        props.editor.chain().focus().insertBlockMath({ latex: trimmed }).run()
      }
    }
  }
  showMathDialog.value = false
  mathLatex.value = ''
}

function closeMathDialog() {
  showMathDialog.value = false
  mathLatex.value = ''
}

function handleMathKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeMathDialog()
  }
}

function closeYoutubeDialog() {
  showYoutubeDialog.value = false
  youtubeUrl.value = ''
}

function handleYoutubeKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeYoutubeDialog()
  }
}

function handleTableKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeTableDialog()
  }
}

function handleLinkKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeLinkDialog()
  }
}

function handleImageKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeImageDialog()
  }
}

function handleHeadingChange(e: Event) {
  const select = e.target as HTMLSelectElement
  const value = select.value
  if (value === 'paragraph') {
    props.editor.chain().focus().setParagraph().run()
  } else {
    const level = parseInt(value) as HeadingLevel

    props.editor.chain().focus().toggleHeading({ level }).run()
  }
}

function getActiveHeadingLevel(): string {
  for (const level of [1, 2, 3]) {
    if (props.editor.isActive('heading', { level })) {
      return String(level)
    }
  }
  return 'paragraph'
}

defineExpose({
  setLink,
  setImage,
  removeLink,
})
</script>

<template>
  <div class="editor-toolbar">
    <!-- Text Formatting -->
    <EditorButton
      :is-active="editor.isActive('bold')"
      :is-disabled="!editor.can().chain().focus().toggleBold().run()"
      icon="bold"
      label="Bold"
      shortcut="Ctrl+B"
      @click="editor.chain().focus().toggleBold().run()"
    />

    <EditorButton
      :is-active="editor.isActive('italic')"
      :is-disabled="!editor.can().chain().focus().toggleItalic().run()"
      icon="italic"
      label="Italic"
      shortcut="Ctrl+I"
      @click="editor.chain().focus().toggleItalic().run()"
    />

    <EditorButton
      :is-active="editor.isActive('underline')"
      :is-disabled="!editor.can().chain().focus().toggleUnderline().run()"
      icon="underline"
      label="Underline"
      shortcut="Ctrl+U"
      @click="editor.chain().focus().toggleUnderline().run()"
    />

    <div class="toolbar-divider" />

    <!-- Headings -->
    <select
      class="heading-select"
      aria-label="Heading level"
      :value="getActiveHeadingLevel()"
      @change="handleHeadingChange"
    >
      <option value="paragraph">Normal</option>
      <option value="1">Heading 1</option>
      <option value="2">Heading 2</option>
      <option value="3">Heading 3</option>
    </select>

    <div class="toolbar-divider" />

    <!-- Lists -->
    <EditorButton
      :is-active="editor.isActive('bulletList')"
      icon="list"
      label="Bullet List"
      @click="editor.chain().focus().toggleBulletList().run()"
    />

    <EditorButton
      :is-active="editor.isActive('orderedList')"
      icon="numberedList"
      label="Numbered List"
      @click="editor.chain().focus().toggleOrderedList().run()"
    />

    <div class="toolbar-divider" />

    <!-- Quote and Code -->
    <EditorButton
      :is-active="editor.isActive('blockquote')"
      icon="quote"
      label="Quote"
      @click="editor.chain().focus().toggleBlockquote().run()"
    />

    <EditorButton
      :is-active="editor.isActive('codeBlock')"
      icon="code"
      label="Code Block"
      @click="editor.chain().focus().toggleCodeBlock().run()"
    />

    <div class="toolbar-divider" />

    <!-- Link -->
    <EditorButton
      :is-active="editor.isActive('link')"
      icon="link"
      label="Insert Link"
      shortcut="Ctrl+K"
      @click="openLinkDialog"
    />

    <!-- Image -->
    <EditorButton
      icon="image"
      label="Insert Image"
      @click="openImageDialog"
    />

    <div class="toolbar-divider" />

    <!-- Table -->
    <EditorButton
      icon="table"
      label="Insert Table"
      @click="openTableDialog"
    />

    <!-- Horizontal Rule -->
    <EditorButton
      icon="hr"
      label="Horizontal Rule"
      @click="editor.chain().focus().setHorizontalRule().run()"
    />

    <div class="toolbar-divider" />

    <!-- Inline Math -->
    <EditorButton
      icon="inline-math"
      label="Inline Math (LaTeX)"
      @click="openMathDialog('inline')"
    />

    <!-- Block Math -->
    <EditorButton
      icon="block-math"
      label="Block Math (LaTeX)"
      @click="openMathDialog('block')"
    />

    <div class="toolbar-divider" />

    <!-- YouTube -->
    <EditorButton
      icon="youtube"
      label="Insert YouTube Video"
      @click="openYoutubeDialog"
    />

    <div class="toolbar-spacer" />

    <!-- Undo/Redo -->
    <EditorButton
      :is-disabled="!canUndo"
      icon="undo"
      label="Undo"
      shortcut="Ctrl+Z"
      @click="editor.chain().focus().undo().run()"
    />

    <EditorButton
      :is-disabled="!canRedo"
      icon="redo"
      label="Redo"
      shortcut="Ctrl+Shift+Z"
      @click="editor.chain().focus().redo().run()"
    />

    <!-- Link Dialog -->
    <Teleport to="body">
      <div
        v-if="showLinkDialog"
        class="dialog-overlay"
        role="dialog"
        aria-modal="true"
        aria-label="Insert Link"
        @click.self="closeLinkDialog"
        @keydown="handleLinkKeydown"
      >
        <div class="dialog-content">
          <h3>Insert Link</h3>
          <input
            ref="linkInputRef"
            v-model="linkUrl"
            type="url"
            placeholder="https://example.com"
            class="dialog-input"
            @keydown.enter="setLink"
          />
          <p class="dialog-hint">Selected text will be used as the link display text.</p>
          <div class="dialog-actions">
            <button
              v-if="editor.isActive('link')"
              type="button"
              class="dialog-button danger"
              @click="removeLink(); closeLinkDialog()"
            >
              Remove Link
            </button>
            <div class="dialog-actions-right">
              <button type="button" class="dialog-button" @click="closeLinkDialog">
                Cancel
              </button>
              <button
                type="button"
                class="dialog-button primary"
                :disabled="!isValidUrl(linkUrl)"
                @click="setLink"
              >
                Insert
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Image Dialog -->
    <Teleport to="body">
      <div
        v-if="showImageDialog"
        class="dialog-overlay"
        role="dialog"
        aria-modal="true"
        aria-label="Insert Image"
        @click.self="closeImageDialog"
        @keydown="handleImageKeydown"
      >
        <div class="dialog-content">
          <h3>Insert Image</h3>
          <input
            ref="imageInputRef"
            v-model="imageUrl"
            type="url"
            placeholder="https://example.com/image.jpg"
            class="dialog-input"
            @keydown.enter="setImage"
          />
          <div class="dialog-actions">
            <button type="button" class="dialog-button" @click="closeImageDialog">
              Cancel
            </button>
            <button
              type="button"
              class="dialog-button primary"
              :disabled="!isValidUrl(imageUrl)"
              @click="setImage"
            >
              Insert
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Table Dialog -->
    <Teleport to="body">
      <div
        v-if="showTableDialog"
        class="dialog-overlay"
        role="dialog"
        aria-modal="true"
        aria-label="Insert Table"
        @click.self="closeTableDialog"
        @keydown="handleTableKeydown"
      >
        <div class="dialog-content">
          <h3>Insert Table</h3>
          <div class="dialog-row">
            <label class="dialog-label">
              Rows
              <input
                ref="tableRowsRef"
                v-model.number="tableRows"
                type="number"
                min="1"
                max="10"
                class="dialog-input dialog-input--small"
                @keydown.enter="insertTable"
              />
            </label>
            <label class="dialog-label">
              Columns
              <input
                v-model.number="tableCols"
                type="number"
                min="1"
                max="10"
                class="dialog-input dialog-input--small"
                @keydown.enter="insertTable"
              />
            </label>
          </div>
          <div class="dialog-actions">
            <button type="button" class="dialog-button" @click="closeTableDialog">
              Cancel
            </button>
            <button type="button" class="dialog-button primary" @click="insertTable">
              Insert
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- YouTube Dialog -->
    <Teleport to="body">
      <div
        v-if="showYoutubeDialog"
        class="dialog-overlay"
        role="dialog"
        aria-modal="true"
        aria-label="Insert YouTube Video"
        @click.self="closeYoutubeDialog"
        @keydown="handleYoutubeKeydown"
      >
        <div class="dialog-content">
          <h3>Insert YouTube Video</h3>
          <input
            ref="youtubeInputRef"
            v-model="youtubeUrl"
            type="url"
            placeholder="https://www.youtube.com/watch?v=..."
            class="dialog-input"
            @keydown.enter="insertYoutube"
          />
          <p class="dialog-hint">
            Paste a YouTube URL (watch, embed, or short link).
          </p>
          <div class="dialog-actions">
            <button type="button" class="dialog-button" @click="closeYoutubeDialog">
              Cancel
            </button>
            <button
              type="button"
              class="dialog-button primary"
              :disabled="!youtubeUrl"
              @click="insertYoutube"
            >
              Insert
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Math Dialog -->
    <Teleport to="body">
      <div
        v-if="showMathDialog"
        class="dialog-overlay"
        role="dialog"
        aria-modal="true"
        aria-label="Insert LaTeX Math"
        @click.self="closeMathDialog"
        @keydown="handleMathKeydown"
      >
        <div class="dialog-content">
          <h3>{{ mathMode === 'inline' ? 'Insert Inline Math' : 'Insert Block Math' }}</h3>
          <input
            ref="mathInputRef"
            v-model="mathLatex"
            type="text"
            :placeholder="mathMode === 'inline' ? 'e.g. E = mc^2' : 'e.g. \\int_0^\\infty e^{-x^2} dx'"
            class="dialog-input"
            @keydown.enter="insertMath"
          />
          <p class="dialog-hint">Enter LaTeX expression. KaTeX will render it.</p>
          <div class="dialog-actions">
            <button type="button" class="dialog-button" @click="closeMathDialog">
              Cancel
            </button>
            <button
              type="button"
              class="dialog-button primary"
              :disabled="!mathLatex.trim()"
              @click="insertMath"
            >
              Insert
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.editor-toolbar {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 0.25rem;
  padding: 0.5rem;
  border-bottom: 1px solid var(--brand-light-2);
  background-color: var(--brand-light-1);
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  position: relative;
}

.editor-toolbar::-webkit-scrollbar {
  display: none;
}

/* Scroll fade indicators */
.editor-toolbar::after {
  content: '';
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 2rem;
  background: linear-gradient(to right, transparent, var(--brand-light-1));
  pointer-events: none;
}

.toolbar-divider {
  width: 1px;
  height: 1.5rem;
  background-color: var(--brand-light-2);
  flex-shrink: 0;
}

.toolbar-spacer {
  flex-grow: 1;
}

.heading-select {
  padding: 0.5rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
  font-size: 0.875rem;
  cursor: pointer;
  flex-shrink: 0;
}

.heading-select:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 2px var(--brand-primary-light);
}

.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog-content {
  background-color: var(--color-background);
  padding: 1.5rem;
  border-radius: 0.5rem;
  min-width: 300px;
  max-width: 90vw;
}

.dialog-content h3 {
  margin: 0 0 1rem 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--brand-dark-1);
}

.dialog-row {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
}

.dialog-label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--brand-dark-1);
  flex: 1;
}

.dialog-input {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  background-color: var(--brand-light-1);
  color: var(--brand-dark-1);
}

.dialog-input--small {
  width: 80px;
}

.dialog-input:focus {
  outline: none;
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 2px var(--brand-primary-light);
}

.dialog-hint {
  margin: 0.5rem 0 0 0;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.dialog-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 1rem;
  gap: 0.5rem;
}

.dialog-actions-right {
  display: flex;
  gap: 0.5rem;
}

.dialog-button {
  padding: 0.5rem 1rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
  font-size: 0.875rem;
  cursor: pointer;
  transition: all 0.2s;
}

.dialog-button:hover {
  background-color: var(--brand-light-1);
}

.dialog-button.primary {
  background-color: var(--color-info);
  color: var(--color-white);
  border-color: var(--color-info);
}

.dialog-button.primary:hover {
  background-color: var(--color-info);
}

.dialog-button.primary:disabled {
  background-color: var(--color-info);
  border-color: var(--color-info);
  cursor: not-allowed;
}

.dialog-button.danger {
  background-color: var(--color-error);
  color: var(--color-white);
  border-color: var(--color-error);
}

.dialog-button.danger:hover {
  background-color: var(--color-error);
}

@media (max-width: 768px) {
  .editor-toolbar button,
  .editor-toolbar select {
    min-width: 44px;
    min-height: 44px;
  }
}
</style>
