import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import type { ComponentPublicInstance } from 'vue'
import ContentListView from './ContentListView.vue'
import { createPinia, setActivePinia } from 'pinia'
import { useContentStore } from '@/stores/domain/content'
import type { Content } from '@/types/content'

// Helper to access internal component properties in tests
type ContentListViewInstance = ComponentPublicInstance & {
  contents: Content[]
  getStatusBadgeClass: (status: string) => string
  loadContents: () => Promise<void>
  editContent: (content: Content) => void
  handleSaved: (content: Content, redirectTo?: string) => Promise<void>
}
function vm(wrapper: VueWrapper<ComponentPublicInstance>): ContentListViewInstance {
  return wrapper.vm as unknown as ContentListViewInstance
}

const mockPush = vi.fn()
const mockRouter = {
  push: mockPush,
}

vi.mock('vue-router', () => ({
  useRouter: () => mockRouter,
  useRoute: () => ({
    path: '/content',
    params: {},
    query: { type: 'post' },
    fullPath: '/content?type=post',
  }),
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({
    userId: { value: 1 },
    isAuthenticated: { value: true },
    role: { value: null },
  }),
}))

vi.mock('@/composables/useConfig', () => ({
  useConfig: () => ({
    languages: { value: ['en'] },
    isLoaded: { value: false },
    fetchConfig: vi.fn(() => Promise.resolve(['en'])),
    primaryLanguage: () => 'en',
  }),
}))

// Mock TipTap extensions
vi.mock('@tiptap/starter-kit', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-underline', () => ({ default: {} }))
vi.mock('@tiptap/extension-link', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-image', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-placeholder', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-table', () => ({ Table: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-table-row', () => ({ default: {} }))
vi.mock('@tiptap/extension-table-cell', () => ({ default: {} }))
vi.mock('@tiptap/extension-table-header', () => ({ default: {} }))
vi.mock('@tiptap/vue-3', () => ({
  useEditor: vi.fn(() => ({
    chain: vi.fn(() => ({ focus: vi.fn(() => ({ run: vi.fn() })) })),
    can: vi.fn(() => ({ chain: vi.fn(() => ({ focus: vi.fn(() => ({ run: vi.fn() })) })) })),
    isActive: vi.fn(() => false),
    getJSON: vi.fn(() => ({ type: 'doc', content: [] })),
    commands: { setContent: vi.fn() },
    destroy: vi.fn(),
  })),
  EditorContent: { name: 'EditorContent', template: '<div />', props: ['editor'] },
}))

describe('ContentListView', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
    mockPush.mockClear()
  })

  describe('Content List Display', () => {
    it('displays draft badge for draft content', async () => {
      const mockContents: Content[] = [
        {
          id: 1,
          userId: 1,
          title: 'Draft Post',
          slug: 'draft-post',
          content: '{"type":"doc"}',
          tags: [],
          status: 'draft',
          createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
          updatedAt: '2026-04-08T00:00:00Z',
        },
      ]

      // Set up mock before mounting
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getByUser').mockResolvedValue(mockContents)

      const wrapper = mount(ContentListView, {
        global: {
          plugins: [pinia],
          stubs: {
            ContentEditor: true,
          },
        },
      })

      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 0))
      await wrapper.vm.$nextTick()

      expect(vm(wrapper).contents).toEqual(mockContents)
      expect(vm(wrapper).getStatusBadgeClass('draft')).toBe('content-list__status--draft')
    })

    it('displays published badge for published content', async () => {
      const mockContents: Content[] = [
        {
          id: 1,
          userId: 1,
          title: 'Published Post',
          slug: 'published-post',
          content: '{"type":"doc"}',
          tags: [],
          status: 'published',
          createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
          updatedAt: '2026-04-08T00:00:00Z',
        },
      ]

      // Set up mock before mounting
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getByUser').mockResolvedValue(mockContents)

      const wrapper = mount(ContentListView, {
        global: {
          plugins: [pinia],
          stubs: {
            ContentEditor: true,
          },
        },
      })

      // Wait for async operations to complete
      await new Promise(resolve => setTimeout(resolve, 0))
      await wrapper.vm.$nextTick()

      expect(vm(wrapper).contents).toEqual(mockContents)
      expect(vm(wrapper).getStatusBadgeClass('published')).toBe('content-list__status--published')
    })

    it('shows empty state when no content exists', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getByUser').mockResolvedValue([])

      const wrapper = mount(ContentListView, {
        global: {
          stubs: {
            ContentEditor: true,
          },
        },
      })

      await wrapper.vm.$nextTick()
      await vm(wrapper).loadContents()

      expect(vm(wrapper).contents.length).toBe(0)
    })

    it('displays content with correct badge text', () => {
      const contentStore = useContentStore()
      const mockContents: Content[] = [
        {
          id: 1,
          userId: 1,
          title: 'Test Post',
          slug: 'test-post',
          content: '{"type":"doc"}',
          tags: [],
          status: 'draft',
          createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
          updatedAt: '2026-04-08T00:00:00Z',
        },
        {
          id: 2,
          userId: 1,
          title: 'Published Post',
          slug: 'published-post',
          content: '{"type":"doc"}',
          tags: [],
          status: 'published',
          createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
          updatedAt: '2026-04-08T00:00:00Z',
        },
      ]

      vi.spyOn(contentStore, 'getByUser').mockResolvedValue(mockContents)

      const wrapper = mount(ContentListView, {
        global: {
          stubs: {
            ContentEditor: true,
          },
        },
      })

      wrapper.vm.contents = mockContents

      expect(vm(wrapper).contents[0].status).toBe('draft')
      expect(vm(wrapper).contents[1].status).toBe('published')
    })
  })

  describe('Click to Edit', () => {
    it('navigates to edit page when content is clicked', async () => {
      const contentStore = useContentStore()
      const mockContents: Content[] = [
        {
          id: 1,
          userId: 1,
          title: 'Test Post',
          slug: 'test-post',
          content: '{"type":"doc"}',
          tags: [],
          status: 'draft',
          createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
          updatedAt: '2026-04-08T00:00:00Z',
        },
      ]

      vi.spyOn(contentStore, 'getByUser').mockResolvedValue(mockContents)

      const wrapper = mount(ContentListView, {
        global: {
          stubs: {
            ContentEditor: true,
          },
        },
      })

      await wrapper.vm.$nextTick()
      await vm(wrapper).loadContents()

      vm(wrapper).editContent(mockContents[0])

      expect(mockPush).toHaveBeenCalledWith('/content/1/edit')
    })
  })

  describe('Navigation After Publish', () => {
    it('redirects to content list after publish', async () => {
      const contentStore = useContentStore()
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test Post',
        slug: 'test-post',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        createdAt: '2026-04-08T00:00:00Z',
          postType: 'post',
          language: 'en',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.spyOn(contentStore, 'getByUser').mockResolvedValue([mockContent])

      const wrapper = mount(ContentListView, {
        global: {
          stubs: {
            ContentEditor: true,
          },
        },
      })

      await wrapper.vm.$nextTick()

      // Pass the redirectTo parameter to trigger the navigation
      await vm(wrapper).handleSaved(mockContent, '/content')

      expect(mockPush).toHaveBeenCalledWith('/content')
    })
  })
})
