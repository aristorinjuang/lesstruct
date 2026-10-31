import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import InvoiceSettingsView from './InvoiceSettingsView.vue'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    put: vi.fn(),
  },
  ApiError: class ApiError extends Error {
    statusCode: number
    code?: string

    constructor(message: string, statusCode = 0, code?: string) {
      super(message)
      this.statusCode = statusCode
      this.code = code
    }
  },
}))

const loaded: {
  sellerName: string
  sellerAddress: string
  sellerTaxId: string
  taxRate: number
} = {
  sellerName: 'PT Lesstruct Demo',
  sellerAddress: 'Jl. Example No. 1',
  sellerTaxId: '01.234.567.8-901.234',
  taxRate: 12,
}

function mountView() {
  return mount(InvoiceSettingsView)
}

describe('InvoiceSettingsView', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.put).mockReset()
    vi.mocked(api.get).mockResolvedValue({ data: { data: { ...loaded } } })
    vi.mocked(api.put).mockResolvedValue({ data: { data: { ...loaded } } })
  })

  it('renders the loading state before the settings arrive', () => {
    const wrapper = mountView()

    expect(wrapper.text()).toContain('Loading invoice settings...')
  })

  it('loads the current settings into the form', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/api/v1/orders/invoice-settings')
    expect((wrapper.find('input[placeholder="PT Example Indonesia"]').element as HTMLInputElement).value).toBe(
      loaded.sellerName
    )
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe(loaded.sellerAddress)
    expect((wrapper.find('input[placeholder="01.234.567.8-901.234"]').element as HTMLInputElement).value).toBe(
      loaded.sellerTaxId
    )
    expect((wrapper.find('input[type="number"]').element as HTMLInputElement).value).toBe('12')
  })

  it('submits the edited values and keeps an explicit zero rate', async () => {
    vi.mocked(api.put).mockResolvedValue({
      data: { data: { ...loaded, sellerName: 'New Seller', taxRate: 0 } },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('input[placeholder="PT Example Indonesia"]').setValue('New Seller')
    await wrapper.find('input[type="number"]').setValue('0')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(api.put).toHaveBeenCalledWith('/api/v1/orders/invoice-settings', {
      sellerName: 'New Seller',
      sellerAddress: loaded.sellerAddress,
      sellerTaxId: loaded.sellerTaxId,
      taxRate: 0,
    })
    expect(wrapper.text()).toContain('Invoice settings saved')
  })

  it('shows the server error when saving fails', async () => {
    vi.mocked(api.put).mockRejectedValue(new Error('Failed to save invoice settings'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to save invoice settings')
  })

  it('shows an error when the settings cannot be loaded', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('Network down'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Network down')
    expect(wrapper.find('form').exists()).toBe(true)
  })
})
