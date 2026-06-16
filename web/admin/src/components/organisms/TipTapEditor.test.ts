/* eslint-disable @typescript-eslint/no-explicit-any */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createMockEditor } from './tiptapMocks'
import TipTapEditor from './TipTapEditor.vue'
import EditorToolbar from '@/components/molecules/EditorToolbar.vue'

// Mock TipTap's useEditor and EditorContent using vi.hoisted
const useEditorMock = vi.hoisted(() => vi.fn())

vi.mock('@tiptap/vue-3', () => ({
  useEditor: useEditorMock,
  EditorContent: {
    name: 'EditorContent',
    template: '<div class="prosemirror"></div>',
    props: ['editor'],
  },
}))

vi.mock('@tiptap/extension-table', () => ({
  Table: {
    configure: vi.fn(() => ({ name: 'table' })),
  },
}))

vi.mock('@tiptap/extension-table-row', () => ({
  default: { name: 'tableRow' },
}))

vi.mock('@tiptap/extension-table-cell', () => ({
  default: { name: 'tableCell' },
}))

vi.mock('@tiptap/extension-table-header', () => ({
  default: { name: 'tableHeader' },
}))

vi.mock('@tiptap/extension-code-block-lowlight', () => ({
  default: {
    name: 'codeBlockLowlight',
    configure: vi.fn(() => ({ name: 'codeBlockLowlight' })),
  },
}))

vi.mock('lowlight', () => ({
  common: {},
  createLowlight: vi.fn(() => ({
    highlight: vi.fn(() => ({ children: [] })),
  })),
}))

vi.mock('./TipTapYoutube', () => ({
  Youtube: { name: 'youtube' },
}))

vi.mock('@tiptap/extension-emoji', () => ({
  default: { name: 'emoji' },
  Emoji: { name: 'emoji' },
}))

describe('TipTapEditor', () => {
  let mockEditor: any

  beforeEach(() => {
    mockEditor = createMockEditor()
    useEditorMock.mockReturnValue(mockEditor)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('initializes editor with provided content', () => {
    const content = '{"type":"doc","content":[{"type":"paragraph"}]}'
    const wrapper = mount(TipTapEditor, {
      props: {
        modelValue: content,
      },
    })

    expect(wrapper.find('.tiptap-editor').exists()).toBe(true)
  })

  it('renders toolbar when editor is initialized', () => {
    const wrapper = mount(TipTapEditor, {
      props: {
        modelValue: '{"type":"doc","content":[]}',
      },
    })

    expect(wrapper.findComponent(EditorToolbar).exists()).toBe(true)
  })

  it('emits update:modelValue when content changes', async () => {
    let onUpdateCallback: any
    useEditorMock.mockImplementation((options: any) => {
      onUpdateCallback = options.onUpdate
      return mockEditor
    })

    const wrapper = mount(TipTapEditor, {
      props: {
        modelValue: '{"type":"doc","content":[]}',
      },
    })

    // Simulate content update
    if (onUpdateCallback) {
      mockEditor.getJSON.mockReturnValue({
        type: 'doc',
        content: [{ type: 'paragraph', content: [{ type: 'text', text: 'New content' }] }],
      })
      onUpdateCallback({ editor: mockEditor })
    }

    await nextTick()

    expect(wrapper.emitted('update:modelValue')).toBeDefined()
  })

  it('uses custom placeholder when provided', () => {
    let placeholderValue = ''
    useEditorMock.mockImplementation((options: any) => {
      placeholderValue =
        options.extensions?.find((ext: any) => ext.name === 'placeholder')?.options
          ?.placeholder || ''
      return mockEditor
    })

    mount(TipTapEditor, {
      props: {
        modelValue: '{"type":"doc","content":[]}',
        placeholder: 'Write your story...',
      },
    })

    expect(placeholderValue).toBe('Write your story...')
  })

  it('uses default placeholder when none provided', () => {
    let placeholderValue = ''
    useEditorMock.mockImplementation((options: any) => {
      placeholderValue =
        options.extensions?.find((ext: any) => ext.name === 'placeholder')?.options
          ?.placeholder || ''
      return mockEditor
    })

    mount(TipTapEditor, {
      props: {
        modelValue: '{"type":"doc","content":[]}',
      },
    })

    expect(placeholderValue).toBe('Start writing...')
  })

  it('handles plain text content gracefully', () => {
    let initialContent: any
    useEditorMock.mockImplementation((options: any) => {
      initialContent = options.content
      return mockEditor
    })

    mount(TipTapEditor, {
      props: {
        modelValue: 'Just plain text',
      },
    })

    // Should convert plain text to TipTap format
    expect(initialContent).toBeDefined()
    expect(initialContent.type).toBe('doc')
  })

  it('handles empty content with default empty doc', () => {
    let initialContent: any
    useEditorMock.mockImplementation((options: any) => {
      initialContent = options.content
      return mockEditor
    })

    mount(TipTapEditor, {
      props: {
        modelValue: '',
      },
    })

    expect(initialContent.type).toBe('doc')
    expect(initialContent.content).toBeDefined()
  })

  it('can be mounted and unmounted without errors', () => {
    const wrapper = mount(TipTapEditor, {
      props: {
        modelValue: '{"type":"doc","content":[]}',
      },
    })

    expect(wrapper.find('.tiptap-editor').exists()).toBe(true)

    // Unmount should not throw errors
    expect(() => wrapper.unmount()).not.toThrow()
  })
})
