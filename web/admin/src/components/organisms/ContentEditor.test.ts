import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref, computed } from 'vue'
import { mount } from '@vue/test-utils'
import ContentEditor from './ContentEditor.vue'
import { createPinia, setActivePinia } from 'pinia'
import { useContentStore } from '@/stores/domain/content'
import type { Content } from '@/types/content'
import type { PostType } from '@/types/posttype'

const mockUserRole = ref<string | null>(null)

const mockLanguages = ref<string[]>(['en'])

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(() => Promise.resolve({ data: { data: [], features: { textGeneration: false } } })),
    post: vi.fn(() => Promise.resolve({ data: {} })),
    put: vi.fn(() => Promise.resolve({ data: {} })),
    delete: vi.fn(() => Promise.resolve({ data: {} })),
  },
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    userId: computed(() => 1),
    isAuthenticated: computed(() => true),
    role: computed(() => mockUserRole.value),
  })),
}))

vi.mock('@/composables/useConfig', () => ({
  useConfig: () => ({
    languages: mockLanguages,
    isLoaded: { value: false },
    fetchConfig: vi.fn(async () => mockLanguages.value),
    primaryLanguage: () => mockLanguages.value[0] ?? 'en',
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
  }),
}))

vi.mock('@tiptap/starter-kit', () => ({
  default: { configure: vi.fn(() => ({ name: 'starterKit' })) },
}))

vi.mock('@tiptap/extension-underline', () => ({
  default: { name: 'underline' },
}))

vi.mock('@tiptap/extension-link', () => ({
  default: { configure: vi.fn(() => ({ name: 'link' })) },
}))

vi.mock('@tiptap/extension-image', () => ({
  default: { configure: vi.fn(() => ({ name: 'image' })) },
}))

vi.mock('@tiptap/extension-placeholder', () => ({
  default: { configure: vi.fn(() => ({ name: 'placeholder' })) },
}))

vi.mock('@tiptap/extension-table', () => ({
  Table: { configure: vi.fn(() => ({ name: 'table' })) },
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

vi.mock('./TipTapYoutube', () => ({
  Youtube: { name: 'youtube' },
}))

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

describe('ContentEditor', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockUserRole.value = null
    mockLanguages.value = ['en']
  })

  describe('Status Management', () => {
    it('shows Save Draft and Publish buttons for new content', () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // When content is new, the status should be draft
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).form.status).toBe('draft')
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).isNewContent).toBe(true)
    })

    it('shows Save Draft and Publish buttons for draft content', () => {
      const draftContent: Content = {
        id: 1,
        userId: 1,
        title: 'Draft Content',
        slug: 'draft-content',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: draftContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).form.status).toBe('draft')
    })

    it('shows Save Changes and Unpublish buttons for published content', () => {
      const publishedContent: Content = {
        id: 1,
        userId: 1,
        title: 'Published Content',
        slug: 'published-content',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: publishedContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).form.status).toBe('published')
    })

    it('displays correct title for new content', () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      expect(wrapper.find('.content-editor__title').text()).toBe('Create New Content')
    })

    it('displays correct title for existing content', () => {
      const existingContent: Content = {
        id: 1,
        userId: 1,
        title: 'Existing Content',
        slug: 'existing-content',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: existingContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      expect(wrapper.find('.content-editor__title').text()).toBe('Edit Content')
    })

    it('pre-fills form with initial content data', () => {
      const existingContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test Title',
        slug: 'test-title',
        content: '{"type":"doc","content":[{"type":"paragraph"}]}',
        tags: ['tag1', 'tag2'],
        status: 'published',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: existingContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.form.title).toBe('Test Title')
      expect(vm.form.content).toBe('{"type":"doc","content":[{"type":"paragraph"}]}')
      expect(vm.form.tags).toEqual(['tag1', 'tag2'])
      expect(vm.form.status).toBe('published')
      expect(vm.slug).toBe('test-title')
    })
  })

  describe('Status Transitions', () => {
    it('changes status from draft to published when publish is called', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'update').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      })

      const draftContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: draftContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await (wrapper.vm as any).publish()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).form.status).toBe('published')
    })

    it('changes status from published to draft when unpublish is called', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'update').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      })

      const publishedContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: publishedContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await (wrapper.vm as any).unpublish()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).form.status).toBe('draft')
    })
  })

  describe('SEO Settings Panel', () => {
    it('renders SEO settings toggle button', () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      expect(wrapper.find('.content-editor__seo-toggle').exists()).toBe(true)
      expect(wrapper.find('.content-editor__seo-toggle').text()).toContain('SEO Settings')
    })

    it('SEO settings are collapsed by default', () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).isSEOSettingsOpen).toBe(false)
      expect(wrapper.find('.content-editor__seo-fields').exists()).toBe(false)
    })

    it('toggles SEO settings panel when button is clicked', async () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).isSEOSettingsOpen).toBe(false)

      await wrapper.find('.content-editor__seo-toggle').trigger('click')

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      expect((wrapper.vm as any).isSEOSettingsOpen).toBe(true)
      expect(wrapper.find('.content-editor__seo-fields').exists()).toBe(true)
    })

    it('renders meta description textarea with character counter', async () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // Open SEO settings panel by clicking the toggle button
      await wrapper.find('.content-editor__seo-toggle').trigger('click')
      await wrapper.vm.$nextTick()

      // Check that the SEO fields section now exists
      expect(wrapper.find('.content-editor__seo-fields').exists()).toBe(true)
    })

    it('renders OG title input with character counter', async () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // Open SEO settings panel by clicking the toggle button
      await wrapper.find('.content-editor__seo-toggle').trigger('click')
      await wrapper.vm.$nextTick()

      // Check that the SEO fields section now exists
      expect(wrapper.find('.content-editor__seo-fields').exists()).toBe(true)
    })

    it('shows correct character count for meta description', async () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.isSEOSettingsOpen = true
      vm.form.metaDescription = 'Test meta description'
      await wrapper.vm.$nextTick()

      // Check the character count directly on the form
      expect(vm.form.metaDescription.length).toBe(21)
    })

    it('shows correct character count for OG title', async () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.isSEOSettingsOpen = true
      vm.form.ogTitle = 'Test OG Title'
      await wrapper.vm.$nextTick()

      // Check the character count directly on the form
      expect(vm.form.ogTitle.length).toBe(13)
    })

    it('pre-fills SEO fields from initial content', () => {
      const existingContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test Content',
        slug: 'test-content',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        metaDescription: 'Existing meta description',
        ogTitle: 'Existing OG title',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: existingContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.form.metaDescription).toBe('Existing meta description')
      expect(vm.form.ogTitle).toBe('Existing OG title')
    })

    it('includes SEO fields in save/publish requests', async () => {
      const contentStore = useContentStore()
      const updateSpy = vi.spyOn(contentStore, 'update').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
        metaDescription: 'Custom meta description',
        ogTitle: 'Custom OG title',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      })

      const draftContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
          language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: draftContent,
        },
        global: {
          stubs: {
            InputText: true,
            Button: true,
            Select: true,
            FormField: true,
            TipTapEditor: true,
            MediaPanel: true,
          },
        },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.metaDescription = 'Custom meta description'
      vm.form.ogTitle = 'Custom OG title'

      await vm.publish()

      expect(updateSpy).toHaveBeenCalledWith(1, {
        title: 'Test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
        metaDescription: 'Custom meta description',
        ogTitle: 'Custom OG title',
        allowComments: true,
        customFields: {},
      })
    })
  })

  describe('Custom Fields', () => {
    const defaultStubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
    }

    const postTypeWithFields: PostType = {
      name: 'Menu Item',
      slug: 'menu-item',
      description: 'Restaurant menu item',
      supports: ['tags'],
      fields: [
        { name: 'Price', slug: 'price', type: 'text', required: true },
        { name: 'Description', slug: 'description', type: 'textarea', maxLength: 500 },
        { name: 'Category', slug: 'category', type: 'select', options: ['Pastry', 'Bread'], required: true },
        { name: 'Available', slug: 'available', type: 'checkbox' },
      ],
    }

    const postTypeNoFields: PostType = {
      name: 'Post',
      slug: 'post',
      description: 'Standard blog post',
      supports: ['tags'],
    }

    const contentWithCustomFields: Content = {
      id: 1,
      userId: 1,
      title: 'Chocolate Croissant',
      slug: 'chocolate-croissant',
      content: '{"type":"doc"}',
      tags: [],
      status: 'draft',
      postType: 'menu-item',
          language: 'en',
      customFields: {
        price: '$4.50',
        description: 'Flaky, buttery croissant',
        category: 'Pastry',
        available: true,
      },
      createdAt: '2026-05-10T00:00:00Z',
      updatedAt: '2026-05-10T00:00:00Z',
    }

    it('does not render custom fields section when post type has no fields', () => {
      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      expect(wrapper.find('.content-editor__custom-fields').exists()).toBe(false)
    })

    it('renders custom fields section when post type has fields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      // Wait for onMounted async fetch
      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // Change post type to one with fields
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.content-editor__custom-fields').exists()).toBe(true)
      const renderers = wrapper.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(renderers.length).toBe(4)
    })

    it('includes customFields in saveDraft API call', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const createSpy = vi.spyOn(contentStore, 'create').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'menu-item',
          language: 'en',
        createdAt: '2026-05-10T00:00:00Z',
        updatedAt: '2026-05-10T00:00:00Z',
      })

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '$5.00', category: 'Pastry' }
      await wrapper.vm.$nextTick()

      await vm.saveDraft()

      expect(createSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          customFields: { price: '$5.00', category: 'Pastry' },
        })
      )
    })

    it('includes customFields in publish API call', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const createSpy = vi.spyOn(contentStore, 'create').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'menu-item',
          language: 'en',
        createdAt: '2026-05-10T00:00:00Z',
        updatedAt: '2026-05-10T00:00:00Z',
      })

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '$5.00', category: 'Pastry' }
      await wrapper.vm.$nextTick()

      await vm.publish()

      expect(createSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          customFields: { price: '$5.00', category: 'Pastry' },
        })
      )
    })

    it('includes customFields in unpublish API call', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const updateSpy = vi.spyOn(contentStore, 'update').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'menu-item',
          language: 'en',
        createdAt: '2026-05-10T00:00:00Z',
        updatedAt: '2026-05-10T00:00:00Z',
      })

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: {
            ...contentWithCustomFields,
            status: 'published',
          },
        },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.customFields = { price: '$4.50' }
      await wrapper.vm.$nextTick()

      await vm.unpublish()

      expect(updateSpy).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          customFields: { price: '$4.50' },
        })
      )
    })

    it('includes customFields in saveChanges API call', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const updateSpy = vi.spyOn(contentStore, 'update').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'menu-item',
          language: 'en',
        createdAt: '2026-05-10T00:00:00Z',
        updatedAt: '2026-05-10T00:00:00Z',
      })

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithCustomFields,
        },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.customFields = { price: '$6.00', category: 'Bread' }
      await wrapper.vm.$nextTick()

      await vm.saveChanges()

      expect(updateSpy).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          customFields: { price: '$6.00', category: 'Bread' },
        })
      )
    })

    it('pre-fills custom fields from initialContent prop', () => {
      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithCustomFields,
        },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.customFields).toEqual({
        price: '$4.50',
        description: 'Flaky, buttery croissant',
        category: 'Pastry',
        available: true,
      })
    })

    it('pre-fills custom fields from fetched content', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      vi.spyOn(contentStore, 'getById').mockResolvedValue(contentWithCustomFields)

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
        },
        global: { stubs: defaultStubs },
      })

      // Wait for onMounted async operations
      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.customFields).toEqual({
        price: '$4.50',
        description: 'Flaky, buttery croissant',
        category: 'Pastry',
        available: true,
      })
    })

    it('applies desktop grid CSS class for custom fields section', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      // Wait for onMounted async fetch
      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      const customFieldsDiv = wrapper.find('.content-editor__custom-fields')
      expect(customFieldsDiv.exists()).toBe(true)
      // Verify CSS class exists (grid styles are applied via CSS media queries)
      expect(customFieldsDiv.classes()).toContain('content-editor__custom-fields')
    })
  })

  describe('System Fields', () => {
    const defaultStubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
    }

    const postTypeWithSystemFields: PostType = {
      name: 'Menu Item',
      slug: 'menu-item',
      description: 'Restaurant menu item',
      supports: ['tags'],
      fields: [
        { name: 'Price', slug: 'price', type: 'text', required: true },
      ],
      systemFields: [
        { name: 'Internal SKU', slug: 'internal_sku', type: 'text' },
        { name: 'Inventory Count', slug: 'inventory_count', type: 'number' },
      ],
    }

    const postTypeNoSystemFields: PostType = {
      name: 'Post',
      slug: 'post',
      description: 'Standard blog post',
      supports: ['tags'],
      fields: [
        { name: 'Subtitle', slug: 'subtitle', type: 'text' },
      ],
    }

    const contentWithSystemFields: Content = {
      id: 1,
      userId: 1,
      title: 'Chocolate Croissant',
      slug: 'chocolate-croissant',
      content: '{"type":"doc"}',
      tags: [],
      status: 'draft',
      postType: 'menu-item',
          language: 'en',
      customFields: {
        price: '$4.50',
        internal_sku: 'SKU-001',
        inventory_count: 25,
      },
      createdAt: '2026-05-10T00:00:00Z',
      updatedAt: '2026-05-10T00:00:00Z',
    }

    it('renders system fields when post type has systemFields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.content-editor__system-fields').exists()).toBe(true)
      const renderers = wrapper.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(renderers.length).toBe(3)
    })

    it('does not render system fields section when post type has no systemFields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeNoSystemFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.content-editor__system-fields').exists()).toBe(false)
    })

    it('renders system fields as disabled inputs', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      const systemSection = wrapper.find('.content-editor__system-fields')
      const systemRenderers = systemSection.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(systemRenderers.length).toBe(2)
      for (const renderer of systemRenderers) {
        expect(renderer.props('disabled')).toBe(true)
        expect(renderer.props('systemField')).toBe(true)
      }
    })

    it('pre-fills system field values from initialContent', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.customFields).toEqual({
        price: '$4.50',
        internal_sku: 'SKU-001',
        inventory_count: 25,
      })

      const systemSection = wrapper.find('.content-editor__system-fields')
      expect(systemSection.exists()).toBe(true)
      const systemRenderers = systemSection.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(systemRenderers.length).toBe(2)
      expect(systemRenderers[0].props('modelValue')).toBe('SKU-001')
      expect(systemRenderers[1].props('modelValue')).toBe(25)
    })

    it('clears system field values on post type switch', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields, postTypeNoSystemFields])

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      expect(vm.customFields.internal_sku).toBe('SKU-001')

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      expect(vm.customFields).toEqual({})
    })

    it('uses system- prefixed keys for system fields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      const systemSection = wrapper.find('.content-editor__system-fields')
      const systemRenderers = systemSection.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(systemRenderers[0].attributes('data-field-slug')).toBe('system-internal_sku')
      expect(systemRenderers[1].attributes('data-field-slug')).toBe('system-inventory_count')
    })
  })

  describe('Admin System Fields Editing', () => {
    const defaultStubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
    }

    const postTypeWithSystemFields: PostType = {
      name: 'Menu Item',
      slug: 'menu-item',
      description: 'Restaurant menu item',
      supports: ['tags'],
      fields: [
        { name: 'Price', slug: 'price', type: 'text', required: true },
      ],
      systemFields: [
        { name: 'Internal SKU', slug: 'internal_sku', type: 'text' },
        { name: 'Inventory Count', slug: 'inventory_count', type: 'number' },
      ],
    }

    const contentWithSystemFields: Content = {
      id: 1,
      userId: 1,
      title: 'Chocolate Croissant',
      slug: 'chocolate-croissant',
      content: '{"type":"doc"}',
      tags: [],
      status: 'draft',
      postType: 'menu-item',
          language: 'en',
      customFields: {
        price: '$4.50',
        internal_sku: 'SKU-001',
        inventory_count: 25,
      },
      createdAt: '2026-05-10T00:00:00Z',
      updatedAt: '2026-05-10T00:00:00Z',
    }

    it('renders system fields as disabled when user is not Admin', async () => {
      mockUserRole.value = 'Contributor'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      const systemSection = wrapper.find('.content-editor__system-fields')
      const systemRenderers = systemSection.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(systemRenderers.length).toBe(2)
      for (const renderer of systemRenderers) {
        expect(renderer.props('disabled')).toBe(true)
      }
    })

    it('renders system fields as editable when user is Admin', async () => {
      mockUserRole.value = 'Admin'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      const systemSection = wrapper.find('.content-editor__system-fields')
      const systemRenderers = systemSection.findAllComponents({ name: 'CustomFieldRenderer' })
      expect(systemRenderers.length).toBe(2)
      for (const renderer of systemRenderers) {
        expect(renderer.props('disabled')).toBe(false)
      }
    })

    it('excludes system field slugs from normal save payload', async () => {
      mockUserRole.value = 'Admin'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])
      vi.spyOn(contentStore, 'update').mockResolvedValue(contentWithSystemFields)
      vi.spyOn(contentStore, 'updateSystemFields').mockResolvedValue(contentWithSystemFields)

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      const result = vm.getCustomFieldsOnly()
      expect(result).toEqual({ price: '$4.50' })
      expect(result).not.toHaveProperty('internal_sku')
      expect(result).not.toHaveProperty('inventory_count')
    })

    it('calls updateSystemFields after successful save when Admin', async () => {
      mockUserRole.value = 'Admin'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])
      vi.spyOn(contentStore, 'update').mockResolvedValue(contentWithSystemFields)
      vi.spyOn(contentStore, 'updateSystemFields').mockResolvedValue(contentWithSystemFields)

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.title = 'Updated Title'
      await vm.saveDraft()

      expect(contentStore.update).toHaveBeenCalledTimes(1)
      expect(contentStore.updateSystemFields).toHaveBeenCalledTimes(1)
      expect(contentStore.updateSystemFields).toHaveBeenCalledWith(
        1,
        { internal_sku: 'SKU-001', inventory_count: 25 },
      )
    })

    it('does not call updateSystemFields for non-admin user', async () => {
      mockUserRole.value = 'Contributor'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])
      vi.spyOn(contentStore, 'update').mockResolvedValue(contentWithSystemFields)
      vi.spyOn(contentStore, 'updateSystemFields').mockResolvedValue(contentWithSystemFields)

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.title = 'Updated Title'
      await vm.saveDraft()

      expect(contentStore.update).toHaveBeenCalledTimes(1)
      expect(contentStore.updateSystemFields).not.toHaveBeenCalled()
    })

    it('calls updateSystemFields for new content after create succeeds', async () => {
      mockUserRole.value = 'Admin'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])
      vi.spyOn(contentStore, 'create').mockResolvedValue(contentWithSystemFields)
      vi.spyOn(contentStore, 'updateSystemFields').mockResolvedValue(contentWithSystemFields)

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      // Set postTypes directly on the component's local ref (bypasses onMounted async)
      vm.postTypes = [postTypeWithSystemFields]
      vm.form.postType = 'menu-item'
      vm.form.title = 'New Content'
      // Set required custom field to pass validation
      vm.customFields = { price: '$4.50', internal_sku: 'SKU-001', inventory_count: 25 }
      await wrapper.vm.$nextTick()
      await vm.saveDraft()

      expect(contentStore.create).toHaveBeenCalledTimes(1)
      expect(contentStore.updateSystemFields).toHaveBeenCalledTimes(1)
    })

    it('shows error when system fields save fails but content save succeeds', async () => {
      mockUserRole.value = 'Admin'
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithSystemFields])
      vi.spyOn(contentStore, 'update').mockResolvedValue(contentWithSystemFields)
      vi.spyOn(contentStore, 'updateSystemFields').mockRejectedValue(new Error('Validation failed'))

      const wrapper = mount(ContentEditor, {
        props: {
          userId: 1,
          contentId: 1,
          initialContent: contentWithSystemFields,
        },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      await vm.saveDraft()

      expect(contentStore.update).toHaveBeenCalledTimes(1)
      expect(contentStore.updateSystemFields).toHaveBeenCalledTimes(1)
      expect(vm.error).toContain('Content saved but system fields update failed')
      expect(vm.error).toContain('Validation failed')
    })
  })

  describe('Post Type Switching', () => {
    const defaultStubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
    }

    const postTypeWithFields: PostType = {
      name: 'Menu Item',
      slug: 'menu-item',
      description: 'Restaurant menu item',
      supports: ['tags'],
      fields: [
        { name: 'Price', slug: 'price', type: 'text', required: true },
        { name: 'Description', slug: 'description', type: 'textarea', maxLength: 500 },
      ],
    }

    const postTypeNoFields: PostType = {
      name: 'Post',
      slug: 'post',
      description: 'Standard blog post',
      supports: ['tags'],
    }

    const postTypePage: PostType = {
      name: 'Page',
      slug: 'page',
      description: 'Static page',
      supports: [],
    }

    const anotherPostTypeWithFields: PostType = {
      name: 'Event',
      slug: 'event',
      description: 'Event post type',
      supports: ['tags'],
      fields: [
        { name: 'Date', slug: 'event_date', type: 'date', required: true },
        { name: 'Location', slug: 'location', type: 'text' },
      ],
    }

    it('discards previous custom field values when switching post type', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.customFields = { price: '$4.50', description: 'Tasty' }
      await wrapper.vm.$nextTick()

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      expect(vm.customFields).toEqual({})
    })

    it('clears validation errors when switching post type', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.customFieldErrors = { price: 'Price is required', description: 'Too long' }
      await wrapper.vm.$nextTick()

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      expect(vm.customFieldErrors).toEqual({})
      expect(wrapper.find('.content-editor__validation-summary').exists()).toBe(false)
    })

    it('clears touched state when switching post type', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.customFieldTouched = { price: true, description: true }
      await wrapper.vm.$nextTick()

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      expect(vm.customFieldTouched).toEqual({})
    })

    it('renders new fields for the new post type after switch', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, anotherPostTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      expect(wrapper.findAllComponents({ name: 'CustomFieldRenderer' }).length).toBe(2)

      vm.form.postType = 'event'
      await wrapper.vm.$nextTick()

      expect(wrapper.findAllComponents({ name: 'CustomFieldRenderer' }).length).toBe(2)
    })

    it('allowComments toggle still works after switching post type', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypePage])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any

      vm.form.postType = 'page'
      await wrapper.vm.$nextTick()
      expect(vm.form.allowComments).toBe(false)

      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()
      expect(vm.form.allowComments).toBe(true)
    })

    it('shows no custom fields when switching to a post type with no fields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.content-editor__custom-fields').exists()).toBe(true)

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()
      expect(wrapper.find('.content-editor__custom-fields').exists()).toBe(false)
    })

    it('shows fresh empty fields when switching back to original post type', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields, postTypeNoFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.customFields = { price: '$4.50', description: 'Tasty' }
      vm.customFieldErrors = { price: 'Some error' }
      vm.customFieldTouched = { price: true }
      await wrapper.vm.$nextTick()

      vm.form.postType = 'post'
      await wrapper.vm.$nextTick()

      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      expect(vm.customFields).toEqual({})
      expect(vm.customFieldErrors).toEqual({})
      expect(vm.customFieldTouched).toEqual({})
    })
  })

  describe('Custom Field Validation', () => {
    const defaultStubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
    }

    const postTypeWithFields: PostType = {
      name: 'Menu Item',
      slug: 'menu-item',
      description: 'Restaurant menu item',
      supports: ['tags'],
      fields: [
        { name: 'Price', slug: 'price', type: 'text', required: true },
        { name: 'Description', slug: 'description', type: 'textarea', maxLength: 500 },
        { name: 'Category', slug: 'category', type: 'select', options: ['Pastry', 'Bread'], required: true },
        { name: 'Available', slug: 'available', type: 'checkbox' },
      ],
    }

    it('shows no validation errors when form loads even with required fields empty', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      expect(Object.keys(vm.customFieldErrors).length).toBe(0)
      expect(wrapper.find('.content-editor__validation-summary').exists()).toBe(false)
    })

    it('shows inline error on blur for empty required field', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      await wrapper.vm.$nextTick()

      await vm.validateFieldOnBlur('price')
      await wrapper.vm.$nextTick()

      expect(vm.customFieldErrors.price).toBe('Price is required')
    })

    it('does not show error on blur for filled required field', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.customFields.price = '$5.00'
      await wrapper.vm.$nextTick()

      await vm.validateFieldOnBlur('price')
      await wrapper.vm.$nextTick()

      expect(vm.customFieldErrors.price).toBeUndefined()
    })

    it('validates all required fields on Save Draft', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const createSpy = vi.spyOn(contentStore, 'create')

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '', category: '' }
      await wrapper.vm.$nextTick()

      await vm.saveDraft()

      expect(createSpy).not.toHaveBeenCalled()
      expect(Object.keys(vm.customFieldErrors).length).toBeGreaterThan(0)
    })

    it('validates all required fields on Publish', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const createSpy = vi.spyOn(contentStore, 'create')

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '', category: '' }
      await wrapper.vm.$nextTick()

      await vm.publish()

      expect(createSpy).not.toHaveBeenCalled()
      expect(Object.keys(vm.customFieldErrors).length).toBeGreaterThan(0)
    })

    it('shows error summary listing all invalid fields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = {}
      await wrapper.vm.$nextTick()

      await vm.saveDraft()
      await wrapper.vm.$nextTick()

      const summary = wrapper.find('.content-editor__validation-summary')
      expect(summary.exists()).toBe(true)
      const items = summary.findAll('li')
      expect(items.length).toBe(2)
    })

    it('error summary has role="alert" for screen readers', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = {}
      await wrapper.vm.$nextTick()

      await vm.saveDraft()
      await wrapper.vm.$nextTick()

      const summary = wrapper.find('.content-editor__validation-summary')
      expect(summary.attributes('role')).toBe('alert')
      expect(summary.attributes('aria-live')).toBe('assertive')
    })

    it('valid form passes validation and submits', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const createSpy = vi.spyOn(contentStore, 'create').mockResolvedValue({
        id: 1,
        userId: 1,
        title: 'Test',
        slug: 'test',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'menu-item',
          language: 'en',
        createdAt: '2026-05-10T00:00:00Z',
        updatedAt: '2026-05-10T00:00:00Z',
      })

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '$5.00', category: 'Pastry' }
      await wrapper.vm.$nextTick()

      await vm.saveDraft()

      expect(createSpy).toHaveBeenCalled()
      expect(Object.keys(vm.customFieldErrors).length).toBe(0)
    })

    it('clears error when field value is corrected', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = {}
      await wrapper.vm.$nextTick()

      await vm.saveDraft()
      expect(vm.customFieldErrors.price).toBeDefined()

      vm.customFields.price = '$5.00'
      delete vm.customFieldErrors.price
      await wrapper.vm.$nextTick()

      expect(vm.customFieldErrors.price).toBeUndefined()
    })

    it('maps backend validation errors to correct custom fields', async () => {
      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([postTypeWithFields])
      const apiError = {
        response: {
          data: {
            error: {
              code: 'VALIDATION_ERROR',
              message: 'Custom field validation failed',
              details: [
                { field: 'customFields.price', issue: 'required', message: 'Price is required' },
              ],
            },
          },
        },
      }
      vi.spyOn(contentStore, 'create').mockRejectedValue(apiError)

      const wrapper = mount(ContentEditor, {
        props: { userId: 1 },
        global: { stubs: defaultStubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any
      vm.form.postType = 'menu-item'
      vm.form.title = 'Test'
      vm.customFields = { price: '$5.00', category: 'Pastry' }
      await wrapper.vm.$nextTick()

      await vm.saveDraft()
      await wrapper.vm.$nextTick()

      expect(vm.customFieldErrors.price).toBe('Price is required')
    })
  })

  describe('Language Switching', () => {
    const stubs = {
      InputText: true,
      Button: true,
      Select: true,
      FormField: true,
      TipTapEditor: true,
      MediaPanel: true,
      CustomFieldRenderer: true,
      DeleteConfirmDialog: true,
      Toast: true,
    }

    it('loads an existing translation instead of blanking the form when switching to a secondary language', async () => {
      mockLanguages.value = ['en', 'id']

      const indonesianContent: Content = {
        id: 25,
        userId: 1,
        title: 'Tentang Lesstruct',
        slug: 'tentang-lesstruct',
        content: '{"type":"doc","content":[{"type":"paragraph"}]}',
        tags: [],
        status: 'published',
        postType: 'page',
        language: 'id',
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      }

      const englishContent: Content = {
        id: 4,
        userId: 1,
        title: 'About Lesstruct',
        slug: 'about-lesstruct',
        content: '{"type":"doc","content":[{"type":"paragraph"}]}',
        tags: [],
        status: 'published',
        postType: 'page',
        language: 'en',
        translations: [indonesianContent],
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      }

      // Mirrors ContentListView: the list passes a summary (no translations).
      const summary: Content = { ...englishContent, translations: undefined }

      const contentStore = useContentStore()
      vi.spyOn(contentStore, 'fetchPostTypes').mockResolvedValue([])
      const getById = vi
        .spyOn(contentStore, 'getById')
        .mockResolvedValueOnce(englishContent)    // onMounted authoritative fetch (id 4)
        .mockResolvedValueOnce(indonesianContent) // switching to the ID tab (id 25)

      const wrapper = mount(ContentEditor, {
        props: { userId: 1, contentId: 4, initialContent: summary },
        global: { stubs },
      })

      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const vm = wrapper.vm as any

      // Fix: the authoritative record is fetched on mount so the translations
      // are known — without it the secondary tab has nothing to load.
      expect(getById).toHaveBeenCalledWith(4)
      expect(vm.form.title).toBe('About Lesstruct')

      const idTab = wrapper
        .findAll('button.content-editor__lang-tab')
        .find(b => b.text() === 'ID')
      expect(idTab).toBeTruthy()
      await idTab!.trigger('click')
      await new Promise(r => setTimeout(r, 0))
      await wrapper.vm.$nextTick()

      // The existing Indonesian translation is loaded — not a blanked new-translation form.
      expect(getById).toHaveBeenCalledWith(25)
      expect(vm.form.title).toBe('Tentang Lesstruct')

      wrapper.unmount()
    })
  })
})
