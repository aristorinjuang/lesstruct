import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import GenerateImageModal from './GenerateImageModal.vue'
import type { Media } from '@/stores/domain/media'

const mockGenerateImage = vi.fn()
const mockFetchMedia = vi.fn()

const mockMediaStore = {
  media: [] as Media[],
  isLoading: false,
  generateImage: mockGenerateImage,
  fetchMedia: mockFetchMedia,
}

vi.mock('@/stores/domain/media', () => ({
  useMediaStore: vi.fn(() => mockMediaStore),
}))

function buildMockMedia(id: number, filename: string): Media {
  return {
    id,
    userId: 1,
    filename: `${filename}.webp`,
    originalFilename: `${filename}.jpg`,
    mimeType: 'image/webp',
    fileSize: 204800,
    width: 800,
    height: 600,
    altText: `Alt ${filename}`,
    isWebp: true,
    filePath: `/uploads/media/${filename}.webp`,
    url: `http://localhost:8080/uploads/media/${filename}.webp`,
    hash: `hash-${filename}`,
    variants: {},
    uploadedBy: 'admin',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  }
}

const generatedMedia = buildMockMedia(100, 'ai-generated')

function mountModal(props: { isOpen: boolean; supportsReferences?: boolean } = { isOpen: true }) {
  return mount(GenerateImageModal, { props })
}

function setInputFiles(input: VueWrapper<Element>['element'], files: File[]) {
  Object.defineProperty(input, 'files', { value: files, configurable: true })
}

describe('GenerateImageModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockMediaStore.media = []
    mockMediaStore.isLoading = false
    mockGenerateImage.mockResolvedValue(generatedMedia)
    mockFetchMedia.mockResolvedValue([])
    URL.createObjectURL = vi.fn(() => 'blob:mock-url') as unknown as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('hides the references section when references are not supported', () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: false })

    expect(wrapper.find('#ai-prompt').exists()).toBe(true)
    expect(wrapper.find('.generate-modal__refs').exists()).toBe(false)
  })

  it('shows the references section when references are supported', () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    expect(wrapper.find('.generate-modal__refs').exists()).toBe(true)
    expect(wrapper.find('.generate-modal__add').exists()).toBe(true)
    expect(wrapper.find('.generate-modal__library').exists()).toBe(true)
    expect(wrapper.find('.generate-modal__count').text()).toBe('0/3')
  })

  it('adds an uploaded image as a thumbnail', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [new File(['bytes'], 'ref.png', { type: 'image/png' })])
    await input.trigger('change')

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(1)
    expect(wrapper.find('.generate-modal__count').text()).toBe('1/3')
  })

  it('rejects non-image files', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [new File(['bytes'], 'notes.txt', { type: 'text/plain' })])
    await input.trigger('change')

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(0)
    expect(wrapper.find('.generate-modal__refs .generate-modal__error').text()).toContain(
      'is not an image file',
    )
  })

  it('rejects files over 10MB', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    const big = new File([new ArrayBuffer(11 * 1024 * 1024)], 'big.png', { type: 'image/png' })
    setInputFiles(input.element, [big])
    await input.trigger('change')

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(0)
    expect(wrapper.find('.generate-modal__refs .generate-modal__error').text()).toContain(
      'exceeds the 10MB limit',
    )
  })

  it('caps references at three images', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    const files = [1, 2, 3, 4].map((n) => new File(['bytes'], `ref${n}.png`, { type: 'image/png' }))
    setInputFiles(input.element, files)
    await input.trigger('change')

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(3)
    expect(wrapper.find('.generate-modal__refs .generate-modal__error').text()).toContain(
      'up to 3 reference images',
    )
  })

  it('removes a reference when its remove button is clicked', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [new File(['bytes'], 'ref.png', { type: 'image/png' })])
    await input.trigger('change')
    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(1)

    await wrapper.find('.generate-modal__remove').trigger('click')

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(0)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url')
  })

  it('adds dropped files to the references', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    await wrapper.find('.generate-modal__strip').trigger('drop', {
      dataTransfer: { files: [new File(['bytes'], 'drop.png', { type: 'image/png' })] },
    })

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(1)
  })

  it('shows an error when generating without a prompt', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    await wrapper.find('#ai-prompt').setValue('   ')
    // The Generate button stays disabled for blank prompts, so exercise the
    // Cmd/Ctrl+Enter shortcut to reach the same validation.
    await wrapper.find('#ai-prompt').trigger('keydown.enter.meta.exact')

    expect(wrapper.find('.generate-modal__body > .generate-modal__error').text()).toContain(
      'Please enter a prompt',
    )
    expect(mockGenerateImage).not.toHaveBeenCalled()
  })

  it('generates with references and emits the result', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const file = new File(['bytes'], 'ref.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [file])
    await input.trigger('change')
    await wrapper.find('#ai-prompt').setValue('A sunset in this style')

    const buttons = wrapper.findAll('.generate-modal__footer button')
    await buttons[buttons.length - 1]?.trigger('click')
    await flushPromises()

    expect(mockGenerateImage).toHaveBeenCalledWith('A sunset in this style', [file], { og: false })
    expect(wrapper.emitted('generated')).toBeTruthy()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('opens the library picker, selects media, and adds it as a reference', async () => {
    mockMediaStore.media = [buildMockMedia(1, 'sunset'), buildMockMedia(2, 'mountain')]
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        blob: async () => new Blob(['bytes'], { type: 'image/png' }),
      })),
    )
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    await wrapper.find('.generate-modal__library').trigger('click')
    await flushPromises()

    // Picker view replaces the form and lists the library without refetching.
    expect(wrapper.text()).toContain('Choose from library')
    expect(mockFetchMedia).not.toHaveBeenCalled()
    expect(wrapper.findAll('.generate-modal__grid-item')).toHaveLength(2)

    await wrapper.findAll('.generate-modal__grid-item')[0]?.trigger('click')
    expect(wrapper.text()).toContain('Add 1 image')

    const footerButtons = wrapper.findAll('.generate-modal__footer button')
    await footerButtons[footerButtons.length - 1]?.trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/uploads/media/sunset.webp')
    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(1)
    expect(wrapper.find('.generate-modal__count').text()).toBe('1/3')
  })

  it('generates an Open Graph image when the checkbox is checked', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const checkbox = wrapper.find('.generate-modal__check input')
    expect((checkbox.element as HTMLInputElement).checked).toBe(false)

    await checkbox.setValue(true)
    await wrapper.find('#ai-prompt').setValue('A sunset over the ocean')

    const buttons = wrapper.findAll('.generate-modal__footer button')
    await buttons[buttons.length - 1]?.trigger('click')
    await flushPromises()

    expect(mockGenerateImage).toHaveBeenCalledWith('A sunset over the ocean', [], { og: true })
    expect(wrapper.emitted('generated')).toBeTruthy()
    expect(wrapper.emitted('generated')?.[0]).toEqual([generatedMedia, { og: true }])
  })

  it('emits og false when the checkbox is unchecked', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: false })

    await wrapper.find('#ai-prompt').setValue('A sunset over the ocean')

    const buttons = wrapper.findAll('.generate-modal__footer button')
    await buttons[buttons.length - 1]?.trigger('click')
    await flushPromises()

    expect(mockGenerateImage).toHaveBeenCalledWith('A sunset over the ocean', [], { og: false })
    expect(wrapper.emitted('generated')?.[0]).toEqual([generatedMedia, { og: false }])
  })

  it('opens the library picker in a fullscreen modal', async () => {
    mockMediaStore.media = [buildMockMedia(1, 'sunset')]
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    expect(wrapper.find('.modal__container--full').exists()).toBe(false)

    await wrapper.find('.generate-modal__library').trigger('click')
    await flushPromises()

    expect(wrapper.find('.modal__container--full').exists()).toBe(true)

    const footerButtons = wrapper.findAll('.generate-modal__footer button')
    await footerButtons[0]?.trigger('click')

    expect(wrapper.find('.modal__container--full').exists()).toBe(false)
  })

  it('fetches the library when the picker opens with an empty store', async () => {
    mockMediaStore.media = []
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    await wrapper.find('.generate-modal__library').trigger('click')
    await flushPromises()

    expect(mockFetchMedia).toHaveBeenCalled()
  })

  it('clears references when the modal is reopened', async () => {
    const wrapper = mountModal({ isOpen: true, supportsReferences: true })

    const input = wrapper.find('input[type="file"]')
    setInputFiles(input.element, [new File(['bytes'], 'ref.png', { type: 'image/png' })])
    await input.trigger('change')
    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(1)

    await wrapper.setProps({ isOpen: false })
    await wrapper.setProps({ isOpen: true })

    expect(wrapper.findAll('.generate-modal__thumb')).toHaveLength(0)
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })
})
