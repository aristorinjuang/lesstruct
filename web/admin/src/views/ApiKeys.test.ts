import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import ApiKeys from './ApiKeys.vue'
import type { ApiKey } from '@/types/apiKey'

const keysRef = ref<ApiKey[]>([])
const isLoadingRef = ref(false)
const errorRef = ref<Error | null>(null)
const fetchKeysMock = vi.fn()
const revokeMock = vi.fn()

vi.mock('@/composables/useApiKeys', () => ({
  useApiKeys: () => ({
    keys: keysRef,
    isLoading: isLoadingRef,
    error: errorRef,
    fetchKeys: fetchKeysMock,
    revoke: revokeMock,
  }),
}))

const activeKey: ApiKey = {
  id: 5,
  name: 'Production',
  prefix: 'lesstruct_5••••',
  createdAt: '2026-06-14T00:00:00Z',
  lastUsedAt: null,
  expiresAt: null,
  revokedAt: null,
}

function mountView() {
  return mount(ApiKeys, {
    global: {
      stubs: {
        ApiKeyCreateDialog: {
          name: 'ApiKeyCreateDialog',
          template: '<div class="create-dialog-stub" />',
          props: ['isOpen'],
        },
        ConfirmationDialog: {
          name: 'ConfirmationDialog',
          template: '<div class="confirmation-dialog-stub" />',
          props: ['isOpen', 'title', 'message', 'confirmButtonText', 'cancelButtonText'],
        },
      },
    },
  })
}

function flushPromises(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0))
}

describe('ApiKeys view', () => {
  beforeEach(() => {
    keysRef.value = []
    isLoadingRef.value = false
    errorRef.value = null
    fetchKeysMock.mockResolvedValue([])
    revokeMock.mockResolvedValue(undefined)
    vi.clearAllMocks()
    // Re-apply default resolutions after clearAllMocks.
    fetchKeysMock.mockResolvedValue([])
    revokeMock.mockResolvedValue(undefined)
  })

  it('renders the loading state while fetching', () => {
    isLoadingRef.value = true
    keysRef.value = []

    const wrapper = mountView()

    expect(wrapper.text()).toContain('Loading API keys...')
  })

  it('renders the empty state with a create call-to-action when no keys exist', () => {
    const wrapper = mountView()

    expect(wrapper.text()).toContain('No API keys yet')
    expect(wrapper.find('.api-key-list__empty-button').exists()).toBe(true)
  })

  it('renders the populated list with a revoke button when keys exist', () => {
    keysRef.value = [activeKey]

    const wrapper = mountView()

    expect(wrapper.text()).toContain('lesstruct_5••••')
    expect(wrapper.find('.api-key-list__revoke-button').exists()).toBe(true)
  })

  it('opens the confirmation dialog when revoke is clicked', async () => {
    keysRef.value = [activeKey]

    const wrapper = mountView()

    await wrapper.find('.api-key-list__revoke-button').trigger('click')

    const confirmation = wrapper.findComponent({ name: 'ConfirmationDialog' })
    expect(confirmation.props('isOpen')).toBe(true)
    expect(confirmation.props('confirmButtonText')).toBe('Revoke')
  })

  it('calls revoke and closes the dialog on confirm', async () => {
    revokeMock.mockResolvedValue(undefined)
    keysRef.value = [activeKey]

    const wrapper = mountView()

    await wrapper.find('.api-key-list__revoke-button').trigger('click')

    const confirmation = wrapper.findComponent({ name: 'ConfirmationDialog' })
    expect(confirmation.props('isOpen')).toBe(true)

    confirmation.vm.$emit('confirm')
    await flushPromises()

    expect(revokeMock).toHaveBeenCalledWith(5)
    // Dialog closes (revokeTargetId reset) after a successful revoke.
    expect(wrapper.findComponent({ name: 'ConfirmationDialog' }).props('isOpen')).toBe(false)
  })

  it('renders the error state with a retry button when fetch fails', () => {
    errorRef.value = new Error('Boom')
    keysRef.value = []

    const wrapper = mountView()

    expect(wrapper.find('.api-keys-view__error').exists()).toBe(true)
    expect(wrapper.find('.api-keys-view__retry-button').exists()).toBe(true)
  })
})
