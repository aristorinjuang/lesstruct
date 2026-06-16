import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ApiKeyCreateDialog from './ApiKeyCreateDialog.vue'

const mockCreate = vi.fn()

vi.mock('@/composables/useApiKeys', () => ({
  useApiKeys: () => ({
    create: mockCreate,
  }),
}))

function mountDialog(props: { isOpen?: boolean } = {}) {
  return mount(ApiKeyCreateDialog, {
    props: { isOpen: props.isOpen ?? true },
    global: {
      stubs: {
        Modal: {
          template: '<div v-if="isOpen" class="modal-stub"><slot /></div>',
          props: ['isOpen', 'title'],
        },
      },
    },
  })
}

function flushPromises(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0))
}

describe('ApiKeyCreateDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('blocks submit when the name is empty and shows a validation error', async () => {
    const wrapper = mountDialog()

    const form = wrapper.find('form')
    expect(form.exists()).toBe(true)
    await form.trigger('submit.prevent')
    await flushPromises()

    expect(mockCreate).not.toHaveBeenCalled()
    expect(wrapper.find('.api-key-create-dialog__field-error').text()).toBe(
      'Key name is required',
    )
  })

  it('creates the key and shows the full key once in the success state', async () => {
    mockCreate.mockResolvedValue({
      key: 'lesstruct_3_abcsecret',
      id: 3,
      name: 'Deploy key',
      keyPrefix: 'lesstruct_3',
      createdAt: '2026-06-14T00:00:00Z',
    })

    const wrapper = mountDialog()

    await wrapper.find('#api-key-name').setValue('Deploy key')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(mockCreate).toHaveBeenCalledWith('Deploy key')

    // Success state renders the full key exactly once + a copy button.
    expect(wrapper.find('.api-key-create-dialog__success').exists()).toBe(true)
    expect(wrapper.find('.api-key-create-dialog__key').text()).toBe('lesstruct_3_abcsecret')
    expect(wrapper.find('.api-key-create-dialog__copy-button').exists()).toBe(true)
    expect(wrapper.text()).toContain('This key will not be shown again')

    // Emit created so the parent can toast/refresh.
    expect(wrapper.emitted('created')).toBeTruthy()
  })

  it('resets to the form state and emits close when Done is clicked after success', async () => {
    mockCreate.mockResolvedValue({
      key: 'lesstruct_3_abcsecret',
      id: 3,
      name: 'Deploy key',
      keyPrefix: 'lesstruct_3',
      createdAt: '2026-06-14T00:00:00Z',
    })

    const wrapper = mountDialog()

    await wrapper.find('#api-key-name').setValue('Deploy key')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    // Success state is showing.
    expect(wrapper.find('.api-key-create-dialog__success').exists()).toBe(true)

    // Click Done → handleClose resets state (wipes the transient key) and emits close.
    await wrapper.find('.api-key-create-dialog__button--done').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
    // The success branch is gone (transient key wiped from the DOM).
    expect(wrapper.find('.api-key-create-dialog__success').exists()).toBe(false)
    expect(wrapper.find('.api-key-create-dialog__key').exists()).toBe(false)
  })

  it('maps a network error (no statusCode) to a friendly general message', async () => {
    mockCreate.mockRejectedValue(new Error('Network error'))

    const wrapper = mountDialog()

    await wrapper.find('#api-key-name').setValue('Some key')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.find('.api-key-create-dialog__error').text()).toContain(
      'Unable to connect to server',
    )
    // Form state is restored (not success).
    expect(wrapper.find('.api-key-create-dialog__success').exists()).toBe(false)
  })
})
