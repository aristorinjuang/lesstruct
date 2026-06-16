import { describe, it, expect, vi, beforeEach } from 'vitest'
import { computed, ref } from 'vue'
import { mount } from '@vue/test-utils'
import MediaDetailModal from './MediaDetailModal.vue'
import type { Media } from '@/stores/domain/media'

const mockUserId = ref<number | null>(1)
const mockRole = ref<string | null>(null)

vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    userId: computed(() => mockUserId.value),
    isAuthenticated: computed(() => true),
    role: computed(() => mockRole.value),
  })),
}))

const mockMedia: Media = {
  id: 1,
  userId: 1,
  filename: 'abc123.webp',
  originalFilename: 'sunset_beach.jpg',
  mimeType: 'image/webp',
  fileSize: 245678,
  width: 1200,
  height: 800,
  altText: 'Sunset over the ocean',
  isWebp: true,
  filePath: '/uploads/media/abc123.webp',
  url: 'http://localhost:8080/uploads/media/abc123.webp',
  hash: 'sha256hash1',
  variants: {
    _thumb: {
      filePath: '/uploads/media/abc123_thumb.webp',
      url: 'http://localhost:8080/uploads/media/abc123_thumb.webp',
      width: 370,
      height: 247,
    },
    _medium: {
      filePath: '/uploads/media/abc123_medium.webp',
      url: 'http://localhost:8080/uploads/media/abc123_medium.webp',
      width: 800,
      height: 534,
    },
  },
  uploadedBy: 'admin',
  createdAt: '2026-04-28T10:30:00Z',
  updatedAt: '2026-04-28T10:30:00Z',
}

describe('MediaDetailModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.style.overflow = ''
  })

  const createWrapper = (props: {
    media: Media | null
  }) => {
    return mount(MediaDetailModal, {
      props,
      global: {
        stubs: {
          Teleport: {
            template: '<div><slot /></div>',
          },
        },
      },
    })
  }

  it('renders nothing when media is null', () => {
    const wrapper = createWrapper({ media: null })
    expect(wrapper.find('.media-detail-modal__backdrop').exists()).toBe(false)
  })

  it('renders modal when media is provided', () => {
    const wrapper = createWrapper({ media: mockMedia })

    expect(wrapper.find('.media-detail-modal__backdrop').exists()).toBe(true)
    expect(wrapper.find('.media-detail-modal').exists()).toBe(true)
  })

  it('displays the filename', () => {
    const wrapper = createWrapper({ media: mockMedia })

    expect(wrapper.find('.media-detail-modal__filename').text()).toBe('sunset_beach.jpg')
  })

  it('displays the image', () => {
    const wrapper = createWrapper({ media: mockMedia })

    const img = wrapper.find('.media-detail-modal__image')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('http://localhost:8080/uploads/media/abc123.webp')
    expect(img.attributes('alt')).toBe('Sunset over the ocean')
  })

  it('displays file size', () => {
    const wrapper = createWrapper({ media: mockMedia })
    const metaValues = wrapper.findAll('.media-detail-modal__meta-value')
    const fileSizeEl = metaValues.find((el) => el.text().includes('KB'))
    expect(fileSizeEl).toBeTruthy()
  })

  it('displays dimensions', () => {
    const wrapper = createWrapper({ media: mockMedia })
    const metaValues = wrapper.findAll('.media-detail-modal__meta-value')
    const dimsEl = metaValues.find((el) => el.text().includes('1200'))
    expect(dimsEl).toBeTruthy()
    expect(dimsEl?.text()).toContain('800')
  })

  it('has Download and Delete buttons', () => {
    const wrapper = createWrapper({ media: mockMedia })

    expect(wrapper.find('.media-detail-modal__btn--download').exists()).toBe(true)
    expect(wrapper.find('.media-detail-modal__btn--delete').exists()).toBe(true)
  })

  it('emits close event when close button is clicked', async () => {
    const wrapper = createWrapper({ media: mockMedia })

    await wrapper.find('.media-detail-modal__close').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits delete event when delete button is clicked', async () => {
    const wrapper = createWrapper({ media: mockMedia })

    await wrapper.find('.media-detail-modal__btn--delete').trigger('click')

    expect(wrapper.emitted('delete')).toBeTruthy()
  })

  it('has correct ARIA attributes', () => {
    const wrapper = createWrapper({ media: mockMedia })

    expect(wrapper.find('.media-detail-modal').attributes('role')).toBe('dialog')
    expect(wrapper.find('.media-detail-modal__close').attributes('aria-label')).toBe('Close')
  })

  it('triggers download when download button is clicked', async () => {
    const clickSpy = vi.fn()
    const mockLink = document.createElement('a')
    mockLink.click = clickSpy
    const realCreateElement = document.createElement.bind(document)
    const realAppendChild = document.body.appendChild.bind(document.body)
    const realRemoveChild = document.body.removeChild.bind(document.body)

    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      if (tag === 'a') return mockLink
      return realCreateElement(tag)
    })
    vi.spyOn(document.body, 'appendChild').mockImplementation((node: Node) => {
      if (node === mockLink) return mockLink
      return realAppendChild(node)
    })
    vi.spyOn(document.body, 'removeChild').mockImplementation((node: Node) => {
      if (node === mockLink) return mockLink
      return realRemoveChild(node)
    })

    const wrapper = createWrapper({ media: mockMedia })
    await wrapper.find('.media-detail-modal__btn--download').trigger('click')

    expect(clickSpy).toHaveBeenCalled()

    vi.restoreAllMocks()
  })

  it('formats file size correctly', () => {
    const wrapper = createWrapper({ media: mockMedia })

    const vm = wrapper.vm as any
    expect(vm.formatFileSize(1024)).toBe('1.0 KB')
    expect(vm.formatFileSize(500)).toBe('500 B')
    expect(vm.formatFileSize(2 * 1024 * 1024)).toBe('2.0 MB')
  })
})
