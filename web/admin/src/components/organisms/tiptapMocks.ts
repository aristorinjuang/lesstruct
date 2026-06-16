/* eslint-disable @typescript-eslint/no-explicit-any */
import { vi } from 'vitest'

function makeRun(result = true): any {
  return vi.fn(() => ({ run: vi.fn(() => result) }))
}

function makeCanChain(): any {
  return {
    undo: vi.fn(() => true),
    redo: vi.fn(() => true),
    chain: vi.fn(() => ({
      focus: vi.fn(() => ({
        toggleBold: makeRun(),
        toggleItalic: makeRun(),
        toggleUnderline: makeRun(),
        toggleBulletList: makeRun(),
        toggleOrderedList: makeRun(),
        toggleBlockquote: makeRun(),
        toggleCodeBlock: makeRun(),
        toggleHeading: makeRun(),
      })),
    })),
  }
}

export function createMockEditor() {
  const chainResult: any = {
    focus: vi.fn(() => chainResult),
    toggleBold: vi.fn(() => chainResult),
    toggleItalic: vi.fn(() => chainResult),
    toggleUnderline: vi.fn(() => chainResult),
    toggleHeading: vi.fn(() => chainResult),
    setParagraph: vi.fn(() => chainResult),
    toggleBulletList: vi.fn(() => chainResult),
    toggleOrderedList: vi.fn(() => chainResult),
    toggleBlockquote: vi.fn(() => chainResult),
    toggleCodeBlock: vi.fn(() => chainResult),
    setLink: vi.fn(() => chainResult),
    unsetLink: vi.fn(() => chainResult),
    setImage: vi.fn(() => chainResult),
    setHorizontalRule: vi.fn(() => chainResult),
    insertTable: vi.fn(() => chainResult),
    insertYoutube: vi.fn(() => chainResult),
    undo: vi.fn(() => chainResult),
    redo: vi.fn(() => chainResult),
    run: vi.fn(() => true),
  }

  const editor: any = {
    chain: vi.fn(() => chainResult),
    focus: vi.fn(() => chainResult),
    toggleBold: vi.fn(() => chainResult),
    toggleItalic: vi.fn(() => chainResult),
    toggleUnderline: vi.fn(() => chainResult),
    toggleHeading: vi.fn(() => chainResult),
    setParagraph: vi.fn(() => chainResult),
    toggleBulletList: vi.fn(() => chainResult),
    toggleOrderedList: vi.fn(() => chainResult),
    toggleBlockquote: vi.fn(() => chainResult),
    toggleCodeBlock: vi.fn(() => chainResult),
    setLink: vi.fn(() => chainResult),
    unsetLink: vi.fn(() => chainResult),
    setImage: vi.fn(() => chainResult),
    setHorizontalRule: vi.fn(() => chainResult),
    insertTable: vi.fn(() => chainResult),
    insertYoutube: vi.fn(() => chainResult),
    undo: vi.fn(() => chainResult),
    redo: vi.fn(() => chainResult),
    run: vi.fn(() => true),
    can: vi.fn(() => makeCanChain()),
    isActive: vi.fn((_name: string, _attrs?: any) => false),
    getJSON: vi.fn(() => ({
      type: 'doc',
      content: [{ type: 'paragraph' }],
    })),
    commands: {
      setContent: vi.fn(),
    },
    destroy: vi.fn(),
  }

  return editor
}
