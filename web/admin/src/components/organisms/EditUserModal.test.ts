import { describe, it, expect, beforeEach, vi } from 'vitest'
import { computed, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useUserStore } from '@/stores/domain/user'
import EditUserModal from './EditUserModal.vue'
import type { User } from '@/types/user'
import type { FieldSchema } from '@/types/customfield'

const mockUserRole = ref<string | null>(null)

vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    userId: computed(() => 1),
    isAuthenticated: computed(() => true),
    role: computed(() => mockUserRole.value),
  })),
}))

vi.mock('@/utils/validation', () => ({
  validateCustomField: vi.fn(() => null),
  validateCustomFields: vi.fn(() => ({})),
}))

const mockUser: User = {
  id: '1',
  username: 'johndoe',
  name: 'John Doe',
  email: 'john@example.com',
  role: 'Contributor',
  status: 'Active',
  createdAt: '2026-03-26T14:30:00Z',
  customFields: { job_title: 'Engineer', company: 'Acme' },
}

const defaultUserFields: FieldSchema[] = [
  { name: 'Job Title', slug: 'job_title', type: 'text' },
  { name: 'Company', slug: 'company', type: 'text' },
  { name: 'Website', slug: 'website', type: 'text' },
]

const defaultUserSystemFields: FieldSchema[] = [
  { name: 'Internal Rating', slug: 'internal_rating', type: 'select', options: ['bronze', 'silver', 'gold', 'platinum'] },
]

function mountModal(props: {
  isOpen: boolean
  userId: string
  userFields?: FieldSchema[]
  userSystemFields?: FieldSchema[]
}) {
  return mount(EditUserModal, {
    props: {
      userFields: props.userFields ?? [],
      userSystemFields: props.userSystemFields ?? [],
      ...props,
    },
    global: {
      stubs: {
        Modal: {
          template: '<div v-if="isOpen" class="modal-stub"><slot /></div>',
          props: ['isOpen', 'title'],
        },
        CustomFieldRenderer: {
          name: 'CustomFieldRenderer',
          template: '<div class="custom-field-renderer-stub" />',
          props: ['field', 'modelValue', 'error', 'disabled', 'systemField'],
          emits: ['update:modelValue', 'blur'],
        },
      },
    },
  })
}

function flushPromises(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0))
}

describe('EditUserModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockUserRole.value = null
  })

  describe('custom field rendering', () => {
    it('renders custom field renderers when userFields prop is provided', async () => {
      const store = useUserStore()
      store.users = [mockUser]

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: defaultUserFields,
        userSystemFields: defaultUserSystemFields,
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const renderers = wrapper.findAll('.custom-field-renderer-stub')
      expect(renderers.length).toBe(4)
    })

    it('renders no custom field section when no fields configured', async () => {
      const store = useUserStore()
      store.users = [mockUser]

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: [],
        userSystemFields: [],
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const renderers = wrapper.findAll('.custom-field-renderer-stub')
      expect(renderers.length).toBe(0)
    })

    it('renders system fields with disabled and system-field attributes', async () => {
      const store = useUserStore()
      store.users = [mockUser]

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: defaultUserFields,
        userSystemFields: defaultUserSystemFields,
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const systemRenderers = wrapper.findAllComponents({ name: 'CustomFieldRenderer' })
      // System fields come after custom fields; last renderer is the system field
      const systemRenderer = systemRenderers[systemRenderers.length - 1]
      expect(systemRenderer.props('disabled')).toBe(true)
      expect(systemRenderer.props('systemField')).toBe(true)
    })
  })

  describe('schema fetch', () => {
    it('gracefully handles empty schema props', async () => {
      const store = useUserStore()
      store.users = [mockUser]

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: [],
        userSystemFields: [],
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      expect(wrapper.find('.modal-stub').exists()).toBe(true)
      expect(wrapper.find('select').exists()).toBe(true)

      const renderers = wrapper.findAll('.custom-field-renderer-stub')
      expect(renderers.length).toBe(0)
    })
  })

  describe('submit with custom fields', () => {
    it('includes customFields in updateUser call', async () => {
      const store = useUserStore()
      store.users = [mockUser]
      vi.spyOn(store, 'updateUser').mockResolvedValue(mockUser)

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: defaultUserFields,
        userSystemFields: defaultUserSystemFields,
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const form = wrapper.find('form')
      expect(form.exists()).toBe(true)
      await form.trigger('submit.prevent')
      await flushPromises()

      expect(store.updateUser).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          customFields: expect.any(Object),
        }),
      )
    })

    it('sends undefined customFields when no custom field values present', async () => {
      const store = useUserStore()
      const userWithoutCustomFields = { ...mockUser, customFields: undefined }
      store.users = [userWithoutCustomFields]
      vi.spyOn(store, 'updateUser').mockResolvedValue(userWithoutCustomFields)

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: [],
        userSystemFields: [],
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const form = wrapper.find('form')
      await form.trigger('submit.prevent')
      await flushPromises()

      expect(store.updateUser).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          customFields: undefined,
        }),
      )
    })

    it('excludes system field slugs from submit payload', async () => {
      const store = useUserStore()
      store.users = [mockUser]
      vi.spyOn(store, 'updateUser').mockResolvedValue(mockUser)

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: defaultUserFields,
        userSystemFields: defaultUserSystemFields,
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      const form = wrapper.find('form')
      await form.trigger('submit.prevent')
      await flushPromises()

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const updateCall = (store.updateUser as any).mock.calls[0]
      const payload = updateCall[1] as { customFields?: Record<string, unknown> }
      expect(payload.customFields).not.toHaveProperty('internal_rating')
      expect(payload.customFields).toHaveProperty('job_title')
      expect(payload.customFields).toHaveProperty('company')
    })

    it('does not mutate store user object when populating custom fields', async () => {
      const store = useUserStore()
      store.users = [mockUser]
      const originalCustomFields = { ...mockUser.customFields }

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: defaultUserFields,
        userSystemFields: defaultUserSystemFields,
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      expect(store.users[0].customFields).toEqual(originalCustomFields)
    })
  })

  describe('existing fields', () => {
    it('renders username, name, email, and role fields', async () => {
      const store = useUserStore()
      store.users = [mockUser]

      const wrapper = mountModal({
        isOpen: false,
        userId: '1',
        userFields: [],
        userSystemFields: [],
      })
      await wrapper.setProps({ isOpen: true })
      await flushPromises()

      expect(wrapper.find('.modal-stub').exists()).toBe(true)
      expect(wrapper.find('input[disabled]').exists()).toBe(true)
      expect(wrapper.find('input[type="email"]').exists()).toBe(true)
      expect(wrapper.find('select').exists()).toBe(true)
    })
  })
})