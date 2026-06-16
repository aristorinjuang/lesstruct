import { describe, it, expect, beforeEach, vi } from 'vitest'
import { computed, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import ContentCreateView from './ContentCreateView.vue'
import ContentEditor from '@/components/organisms/ContentEditor.vue'

// Mock useAuth composable
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

// Mock vue-router
const mockPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: vi.fn(() => ({ push: mockPush })),
  useRoute: vi.fn(() => ({ path: '/content/create' })),
}))

// Mock TipTap extensions (needed because ContentEditor -> TipTapEditor imports them)
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

describe('ContentCreateView', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
    mockUserId.value = 5
  })

  const mountView = () => {
    return mount(ContentCreateView, {
      global: {
        plugins: [pinia],
        stubs: {
          ContentEditor: true,
        },
      },
    })
  }

  describe('userId from auth store', () => {
    it('should pass userId from useAuth to ContentEditor', () => {
      const wrapper = mountView()
      const contentEditor = wrapper.findComponent(ContentEditor)

      expect(contentEditor.props('userId')).toBe(5)
    })

    it('should NOT use hardcoded userId value of 1', () => {
      const wrapper = mountView()
      const contentEditor = wrapper.findComponent(ContentEditor)

      expect(contentEditor.props('userId')).not.toBe(1)
    })
  })

  describe('unauthenticated user', () => {
    it('should redirect to login when userId is null', () => {
      mockUserId.value = null
      mountView()

      expect(mockPush).toHaveBeenCalledWith('/login')
    })

    it('should not render ContentEditor when userId is null', () => {
      mockUserId.value = null
      const wrapper = mountView()

      expect(wrapper.findComponent(ContentEditor).exists()).toBe(false)
    })
  })

  describe('event handlers', () => {
    it('should handle saved event with redirect', async () => {
      const wrapper = mountView()
      const contentEditor = wrapper.findComponent(ContentEditor)

      const mockContent = {
        id: 1,
        title: 'Test Content',
        slug: 'test-content',
      }

      await contentEditor.vm.$emit('saved', mockContent, '/content')

      expect(mockPush).toHaveBeenCalledWith('/content')
    })

    it('should handle cancel event by navigating to content list', async () => {
      const wrapper = mountView()
      const contentEditor = wrapper.findComponent(ContentEditor)

      await contentEditor.vm.$emit('cancel')

      expect(mockPush).toHaveBeenCalledWith('/content')
    })
  })

  describe('rendering', () => {
    it('should render ContentEditor component when authenticated', () => {
      const wrapper = mountView()

      expect(wrapper.findComponent(ContentEditor).exists()).toBe(true)
    })

    it('should apply correct CSS class', () => {
      const wrapper = mountView()

      expect(wrapper.find('.content-create').exists()).toBe(true)
    })
  })
})
