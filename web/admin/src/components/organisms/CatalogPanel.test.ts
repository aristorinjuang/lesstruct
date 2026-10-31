import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CatalogPanel from './CatalogPanel.vue'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    put: vi.fn()
  }
}))

import api from '@/utils/request'

const mockedGet = vi.mocked(api.get)
const mockedPut = vi.mocked(api.put)

function mountPanel(props: Record<string, unknown> = {}, stubs: Record<string, any> = {}) {
  return mount(CatalogPanel, {
    props: { contentId: 7, ...props },
    global: { stubs },
  })
}

describe('CatalogPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockedGet.mockReset()
    mockedPut.mockReset()
  })

  it('offers the legacy import when price and sku custom fields exist', async () => {
    mockedGet.mockRejectedValue(new Error('not found'))

    const wrapper = mountPanel({ legacyFields: { price: 299000, sku: 'TSHIRT' } })
    await flushPromises()

    expect(wrapper.text()).toContain('Import 299000 · TSHIRT into the catalog')
  })

  it('imports legacy fields into the catalog on click', async () => {
    mockedGet.mockRejectedValue(new Error('not found'))
    mockedPut.mockResolvedValue({
      data: { data: { contentId: 7, price: 299000, sku: 'TSHIRT', trackInventory: true, stock: 0, options: {}, variants: [] } }
    })

    const wrapper = mountPanel({ legacyFields: { price: 299000, sku: 'TSHIRT' } })
    await flushPromises()

    const importBtn = wrapper.findAll('button').find((b) => b.text().startsWith('Import'))
    await importBtn!.trigger('click')
    await flushPromises()

    expect(mockedPut).toHaveBeenCalledWith('/api/v1/catalog/products/7', expect.objectContaining({
      price: 299000,
      sku: 'TSHIRT'
    }))
    expect(wrapper.text()).not.toContain('Import 299000')
  })

  it('populates fields from stored catalog data', async () => {
    mockedGet.mockResolvedValue({
      data: {
        data: {
          contentId: 7, price: 299000, salePrice: 249000, sku: 'TSHIRT',
          trackInventory: true, stock: 7, options: { size: ['S', 'M'] },
          variants: [{ id: 1, options: { size: 'S' }, sku: 'TSHIRT-S', price: null, stock: 2 }]
        }
      }
    })

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('S')
    const values = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(values).toContain('299000')
    expect(wrapper.text()).not.toContain('Import')
  })

  it('generates the variant cross-product from the axes', async () => {
    mockedGet.mockResolvedValue({ data: { data: { contentId: 7, price: 0, trackInventory: true, stock: 0, options: {}, variants: [] } } })

    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.find('.catalog-panel__axes').setValue('size: S, M\ncolor: Red')
    const genBtn = wrapper.findAll('button').find((b) => b.text() === 'Generate variants')
    await genBtn!.trigger('click')

    const rows = wrapper.findAll('.catalog-panel__variant')
    expect(rows).toHaveLength(2)
  })

  it('marks the panel as saved separately from the content', async () => {
    mockedGet.mockResolvedValue({ data: { data: { contentId: 7, price: 0, trackInventory: true, stock: 0, options: {}, variants: [] } } })

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('saved separately')
  })

  it('shows a retry and disables save when loading fails', async () => {
    mockedGet.mockRejectedValue(new Error('network down'))

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('network down')
    expect(wrapper.text()).toContain('Retry')
    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Save catalog data')
    expect(saveBtn!.attributes('disabled')).toBeDefined()
  })

  it('recovers through retry after a load failure', async () => {
    mockedGet.mockRejectedValue(new Error('network down'))

    // The embedded media panel fires its own request on mount, so it is
    // stubbed here to keep the catalog request ordering deterministic.
    const wrapper = mountPanel({}, { MediaPanel: true })
    await flushPromises()
    expect(wrapper.text()).toContain('Retry')

    mockedGet.mockReset()
    mockedGet.mockResolvedValue({ data: { data: { contentId: 7, price: 299000, trackInventory: true, stock: 0, options: {}, variants: [] } } })

    const retryBtn = wrapper.findAll('button').find((b) => b.text() === 'Retry')
    await retryBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('Retry')
    const values = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(values).toContain('299000')
  })

  it('renders the live variant sum read-only with the threshold helper', async () => {
    mockedGet.mockResolvedValue({
      data: {
        data: {
          contentId: 7, price: 299000, sku: 'TSHIRT',
          trackInventory: true, stock: 7, lowStockThreshold: 5,
          options: { size: ['S', 'M'] },
          variants: [
            { id: 1, options: { size: 'S' }, sku: 'TSHIRT-S', price: null, stock: 2, imageUrl: '' },
            { id: 2, options: { size: 'M' }, sku: 'TSHIRT-M', price: null, stock: 3, imageUrl: '' }
          ]
        }
      }
    })

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('Sum of variant stock: 5')
    expect(wrapper.text()).toContain('Low-stock alert when the variant sum reaches 5.')
    expect(wrapper.find('[placeholder="Image URL"]').exists()).toBe(false)
  })

  it('picks a variant image through the media panel and emits it on save', async () => {
    const stored = {
      contentId: 7, price: 299000, sku: 'TSHIRT',
      trackInventory: true, stock: 0,
      options: { size: ['S'] },
      variants: [{ id: 1, options: { size: 'S' }, sku: 'TSHIRT-S', price: null, stock: 2, imageUrl: '' }]
    }
    mockedGet.mockResolvedValue({ data: { data: stored } })
    mockedPut.mockResolvedValue({ data: { data: stored } })

    const wrapper = mountPanel()
    await flushPromises()

    const addImageBtn = wrapper.findAll('button').find((b) => b.text() === 'Add image')
    await addImageBtn!.trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)

    await wrapper.findComponent({ name: 'MediaPanel' }).vm.$emit('insert-image', {
      id: 1,
      url: '/uploads/media/picked.webp',
      altText: 'Picked'
    })
    await flushPromises()

    expect(wrapper.find('.catalog-panel__variant-thumb').attributes('src')).toBe('/uploads/media/picked.webp')

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Save catalog data')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(mockedPut).toHaveBeenCalledWith('/api/v1/catalog/products/7', expect.objectContaining({
      variants: [expect.objectContaining({ imageUrl: '/uploads/media/picked.webp' })]
    }))
  })

  it('toggles the variant image picker closed without picking', async () => {
    const stored = {
      contentId: 7, price: 299000, trackInventory: true, stock: 0,
      options: { size: ['S'] },
      variants: [{ id: 1, options: { size: 'S' }, sku: 'TSHIRT-S', price: null, stock: 2, imageUrl: '' }]
    }
    mockedGet.mockResolvedValue({ data: { data: stored } })

    const wrapper = mountPanel()
    await flushPromises()

    const addImageBtn = () => wrapper.findAll('button').find((b) => /^(Add image|Change|Hide media)$/.test(b.text()))!
    await addImageBtn().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)

    await addImageBtn().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(false)
    expect(mockedPut).not.toHaveBeenCalled()
  })

  it('clears the variant image through Remove and saves it empty', async () => {
    const stored = {
      contentId: 7, price: 299000, trackInventory: true, stock: 0,
      options: { size: ['S'] },
      variants: [{ id: 1, options: { size: 'S' }, sku: 'TSHIRT-S', price: null, stock: 2, imageUrl: '/uploads/media/old.webp' }]
    }
    mockedGet.mockResolvedValue({ data: { data: stored } })
    mockedPut.mockResolvedValue({ data: { data: stored } })

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('.catalog-panel__variant-thumb').attributes('src')).toBe('/uploads/media/old.webp')

    // The image Remove button renders before the row Remove button.
    const removeBtn = wrapper.findAll('button').find((b) => b.text() === 'Remove')
    await removeBtn!.trigger('click')
    expect(wrapper.find('.catalog-panel__variant-thumb').exists()).toBe(false)

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Save catalog data')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(mockedPut).toHaveBeenCalledWith('/api/v1/catalog/products/7', expect.objectContaining({
      variants: [expect.objectContaining({ imageUrl: '' })]
    }))
  })

  it('refetches the stored values after saving', async () => {
    const initial = { contentId: 7, price: 299000, trackInventory: true, stock: 0, options: {}, variants: [] }
    const updated = { contentId: 7, price: 310000, trackInventory: true, stock: 0, options: {}, variants: [] }
    // The embedded media panel fires its own request on mount, so it is
    // stubbed here to keep the catalog request ordering deterministic.
    let catalogCalls = 0
    mockedGet.mockImplementation(((path: string) => {
      if (path.startsWith('/api/v1/catalog/products/')) {
        catalogCalls += 1
        const product = catalogCalls === 1 ? initial : updated
        return Promise.resolve({ data: { data: product } })
      }
      return Promise.resolve({ data: { data: [] } })
    }) as typeof mockedGet)
    mockedPut.mockResolvedValue({ data: { data: initial } })

    const wrapper = mountPanel({}, { MediaPanel: true })
    await flushPromises()

    const saveBtn = wrapper.findAll('button').find((b) => b.text() === 'Save catalog data')
    await saveBtn!.trigger('click')
    await flushPromises()

    const values = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(values).toContain('310000')
  })
})
