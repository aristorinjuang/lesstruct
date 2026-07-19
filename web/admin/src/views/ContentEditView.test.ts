import { describe, it, expect, beforeEach, vi } from 'vitest'
import { computed, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import ContentEditView from './ContentEditView.vue'
import ContentEditor from '@/components/organisms/ContentEditor.vue'
import { useContentStore } from '@/stores/domain/content'

const mockUserId = ref<number | null>(5)
vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    userId: computed(() => mockUserId.value),
    isAuthenticated: computed(() => mockUserId.value !== null),
    token: ref('mock-token'),
  })),
  setAuthToken: vi.fn(),
  getAuthStatus: vi.fn(() => true),
}))

const mockPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: vi.fn(() => ({ push: mockPush })),
  useRoute: vi.fn(() => ({
    path: '/content/1/edit',
    params: { id: '1' },
    query: { type: 'post' },
  })),
}))

vi.mock('@tiptap/starter-kit', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-underline', () => ({ default: {} }))
vi.mock('@tiptap/extension-link', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-image', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-placeholder', () => ({ default: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-table', () => ({ Table: { configure: vi.fn(() => ({})) } }))
vi.mock('@tiptap/extension-table-row', () => ({ default: {} }))
vi.mock('@tiptap/extension-table-cell', () => ({ default: {} }))
vi.mock('@tiptap/extension-table-header', () => ({ default: {} }))
vi.mock('@/components/organisms/TipTapYoutube', () => ({ Youtube: { name: 'youtube' } }))
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

const mockContent = {
  id: 1,
  userId: 5,
  title: 'Test Post',
  slug: 'test-post',
  content: '{"type":"doc","content":[]}',
  tags: [],
  status: 'draft' as const,
  createdAt: '2026-04-08T00:00:00Z',
  postType: 'post',
  language: 'en',
  updatedAt: '2026-04-08T00:00:00Z',
}

describe('ContentEditView', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
    mockUserId.value = 5
  })

  const mountView = () => {
    return mount(ContentEditView, {
      global: {
        plugins: [pinia],
        stubs: {
          ContentEditor: true,
        },
      },
    })
  }

  describe('userId from auth store', () => {
    it('should pass userId from useAuth to ContentEditor', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockResolvedValue(mockContent as any)

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      const contentEditor = wrapper.findComponent(ContentEditor)
      expect(contentEditor.props('userId')).toBe(5)
    })
  })

  describe('unauthenticated user', () => {
    it('should redirect to login when userId is null', () => {
      mockUserId.value = null
      mountView()

      expect(mockPush).toHaveBeenCalledWith('/login')
    })

    it('should not render ContentEditor when userId is null', async () => {
      mockUserId.value = null
      const wrapper = mountView()
      await wrapper.vm.$nextTick()

      expect(wrapper.findComponent(ContentEditor).exists()).toBe(false)
    })
  })

  describe('content loading', () => {
    it('should load content by id and pass to editor', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockResolvedValue(mockContent as any)

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      const contentEditor = wrapper.findComponent(ContentEditor)
      expect(contentEditor.exists()).toBe(true)
      expect(contentEditor.props('contentId')).toBe(1)
    })

    it('should show error when content load fails', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockRejectedValue(new Error('not found'))

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.content-edit__state').text()).toBe('Failed to load content')
    })
  })

  describe('event handlers', () => {
    it('should handle saved event with redirect', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockResolvedValue(mockContent as any)

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      const contentEditor = wrapper.findComponent(ContentEditor)
      await contentEditor.vm.$emit('saved', mockContent, '/content')

      expect(mockPush).toHaveBeenCalledWith('/content?type=post')
    })

    it('should handle cancel event by navigating to content list', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockResolvedValue(mockContent as any)

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      const contentEditor = wrapper.findComponent(ContentEditor)
      await contentEditor.vm.$emit('cancel')

      expect(mockPush).toHaveBeenCalledWith('/content?type=post')
    })

    it('should handle deleted event by navigating to content list', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'getById').mockResolvedValue(mockContent as any)

      const wrapper = mountView()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      const contentEditor = wrapper.findComponent(ContentEditor)
      await contentEditor.vm.$emit('deleted')

      expect(mockPush).toHaveBeenCalledWith('/content?type=post')
    })
  })

  describe('rendering', () => {
    it('should render loading state initially', () => {
      const wrapper = mountView()
      expect(wrapper.find('.content-edit__state').text()).toBe('Loading…')
    })

    it('should apply correct CSS class', () => {
      const wrapper = mountView()
      expect(wrapper.find('.content-edit').exists()).toBe(true)
    })
  })
})
