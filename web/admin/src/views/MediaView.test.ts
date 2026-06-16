import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import type { Pinia } from 'pinia'
import MediaView from './MediaView.vue'
import { useMediaStore } from '@/stores/domain/media'
import type { Media } from '@/stores/domain/media'

const mockMedia: Media[] = [
  {
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
    },
    uploadedBy: 'admin',
    createdAt: '2026-04-28T10:30:00Z',
    updatedAt: '2026-04-28T10:30:00Z',
  },
  {
    id: 2,
    userId: 1,
    filename: 'def456.webp',
    originalFilename: 'mountain_view.jpg',
    mimeType: 'image/webp',
    fileSize: 180000,
    width: 1920,
    height: 1080,
    altText: 'Mountain panorama',
    isWebp: true,
    filePath: '/uploads/media/def456.webp',
    url: 'http://localhost:8080/uploads/media/def456.webp',
    hash: 'sha256hash2',
    variants: {},
    uploadedBy: 'admin',
    createdAt: '2026-04-27T08:00:00Z',
    updatedAt: '2026-04-27T08:00:00Z',
  },
  {
    id: 3,
    userId: 1,
    filename: 'ghi789.webp',
    originalFilename: 'city_skyline.jpg',
    mimeType: 'image/webp',
    fileSize: 320000,
    width: 1600,
    height: 900,
    altText: 'City at night',
    isWebp: true,
    filePath: '/uploads/media/ghi789.webp',
    url: 'http://localhost:8080/uploads/media/ghi789.webp',
    hash: 'sha256hash3',
    variants: {
      _thumb: {
        filePath: '/uploads/media/ghi789_thumb.webp',
        url: 'http://localhost:8080/uploads/media/ghi789_thumb.webp',
        width: 370,
        height: 208,
      },
    },
    uploadedBy: 'editor',
    createdAt: '2026-04-26T14:00:00Z',
    updatedAt: '2026-04-26T14:00:00Z',
  },
]

describe('MediaView', () => {
  let pinia: Pinia
  let mediaStore: ReturnType<typeof useMediaStore>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()

    mediaStore = useMediaStore()
  })

  const createWrapper = async () => {
    vi.spyOn(mediaStore, 'fetchMedia').mockResolvedValue(mockMedia)

    const wrapper = mount(MediaView, {
      global: {
        plugins: [pinia],
        stubs: {
          MediaDetailModal: {
            template: '<div class="media-detail-modal-stub" />',
            props: ['media'],
          },
          Toast: {
            template: '<div class="toast-stub" />',
            props: ['message', 'type', 'visible'],
          },
          Button: {
            template: '<button class="btn-stub" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
          },
          Teleport: true,
        },
      },
    })

    await flushPromises()

    // Ensure media is populated
    mediaStore.media = mockMedia
    mediaStore.isLoading = false
    await flushPromises()

    return wrapper
  }

  it('renders the Media Library heading', async () => {
    const wrapper = await createWrapper()
    expect(wrapper.find('h1').text()).toBe('Media Library')
  })

  it('renders the Upload Media button', async () => {
    const wrapper = await createWrapper()
    const uploadBtn = wrapper.findAll('button').find((btn) => btn.text().includes('Upload Media'))
    expect(uploadBtn).toBeTruthy()
  })

  it('renders search input', async () => {
    const wrapper = await createWrapper()
    const searchInput = wrapper.find('.search-wrapper__input')
    expect(searchInput.exists()).toBe(true)
    expect(searchInput.attributes('placeholder')).toBe('Search by filename...')
  })

  it('renders filter dropdown', async () => {
    const wrapper = await createWrapper()
    const select = wrapper.find('.media-view__filter-select')
    expect(select.exists()).toBe(true)
    const options = select.findAll('option')
    expect(options.map((o) => o.text())).toEqual([
      'All Media',
      'Today',
      'This Week',
      'This Month',
    ])
  })

  it('loads media on mount', async () => {
    const fetchSpy = vi.spyOn(mediaStore, 'fetchMedia').mockResolvedValue(mockMedia)
    mount(MediaView, {
      global: {
        plugins: [pinia],
        stubs: {
          MediaDetailModal: { template: '<div />' },
          Toast: { template: '<div />' },
          Button: { template: '<button><slot /></button>' },
          Teleport: true,
        },
      },
    })
    await flushPromises()
    expect(fetchSpy).toHaveBeenCalled()
  })

  it('renders empty state when no media', async () => {
    vi.spyOn(mediaStore, 'fetchMedia').mockResolvedValue([])
    mediaStore.media = []
    mediaStore.isLoading = false

    const wrapper = mount(MediaView, {
      global: {
        plugins: [pinia],
        stubs: {
          MediaDetailModal: { template: '<div />' },
          Toast: { template: '<div />' },
          Button: { template: '<button><slot /></button>' },
          Teleport: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.find('.state-empty__title').text()).toBe('No media files yet')
  })

  it('renders media grid items', async () => {
    const wrapper = await createWrapper()
    const gridItems = wrapper.findAll('.media-view__grid-item')
    expect(gridItems.length).toBe(3)
  })

  it('displays filename in grid items', async () => {
    const wrapper = await createWrapper()
    const filenames = wrapper.findAll('.media-view__grid-filename').map((el) => el.text())
    expect(filenames).toContain('sunset_beach.jpg')
    expect(filenames).toContain('mountain_view.jpg')
    expect(filenames).toContain('city_skyline.jpg')
  })

  it('displays file size and date in grid items', async () => {
    const wrapper = await createWrapper()
    const firstItem = wrapper.findAll('.media-view__grid-item')[0]
    expect(firstItem).toBeDefined()
    if (!firstItem) return
    expect(firstItem.find('.media-view__grid-date').text()).toBeTruthy()
    expect(firstItem.find('.media-view__grid-size').text()).toBeTruthy()
  })

  it('shows loading skeleton when loading', async () => {
    vi.spyOn(mediaStore, 'fetchMedia').mockImplementation(async () => {
      return []
    })
    mediaStore.media = []
    mediaStore.isLoading = true

    const wrapper = mount(MediaView, {
      global: {
        plugins: [pinia],
        stubs: {
          MediaDetailModal: { template: '<div />' },
          Toast: { template: '<div />' },
          Button: { template: '<button><slot /></button>' },
          Teleport: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('.media-view__skeleton-grid').exists()).toBe(true)
  })

  it('shows upload form when Upload Media is clicked', async () => {
    mediaStore.media = []
    mediaStore.isLoading = false

    const wrapper = mount(MediaView, {
      global: {
        plugins: [pinia],
        stubs: {
          MediaDetailModal: { template: '<div />' },
          Toast: { template: '<div />' },
          Button: { template: '<button class="btn-stub" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>' },
          Teleport: true,
        },
      },
    })
    await flushPromises()

    const uploadBtn = wrapper.findAll('button').find((btn) => btn.text().includes('Upload Media'))
    await uploadBtn?.trigger('click')
    await flushPromises()

    expect(wrapper.find('.media-view__upload-form').exists()).toBe(true)
  })

  it('opens detail modal when grid item is clicked', async () => {
    const wrapper = await createWrapper()

    const gridItem = wrapper.findAll('.media-view__grid-item')[0]
    expect(gridItem).toBeDefined()
    if (!gridItem) return
    await gridItem.trigger('click')
    await flushPromises()

    expect(wrapper.find('.media-detail-modal-stub').exists()).toBe(true)
  })

  it('renders responsive grid class', async () => {
    const wrapper = await createWrapper()
    const grid = wrapper.find('.media-view__grid')
    expect(grid.exists()).toBe(true)
    expect(grid.classes()).toContain('media-view__grid')
  })
})
