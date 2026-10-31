import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import GalleryField from './GalleryField.vue'
import type { GalleryItem } from '@/types/customfield'

const items: GalleryItem[] = [
  { url: '/uploads/media/abc.webp', alt: 'Front' },
  { url: '/uploads/media/def.webp', alt: 'Back' }
]

describe('GalleryField', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders one row per item with thumbnail and alt input', () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: items }
    })

    expect(wrapper.findAll('.gallery-field__item')).toHaveLength(2)
    const thumb = wrapper.find('img')
    expect(thumb.attributes('src')).toBe('/uploads/media/abc.webp')
    expect(thumb.attributes('alt')).toBe('Front')
    const altInputs = wrapper.findAll('.gallery-field__alt')
    expect((altInputs[0].element as HTMLInputElement).value).toBe('Front')
  })

  it('emits the updated alt text', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: items }
    })

    await wrapper.findAll('.gallery-field__alt')[0].setValue('New alt')

    expect(wrapper.emitted('update:modelValue')![0][0]).toEqual([
      { url: '/uploads/media/abc.webp', alt: 'New alt' },
      items[1]
    ])
  })

  it('removes an item', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: items }
    })

    await wrapper.findAll('.gallery-field__btn--remove')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0][0]).toEqual([items[1]])
  })

  it('reorders an item up', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: items }
    })

    await wrapper.findAll('.gallery-field__btn[aria-label="Move up"]')[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0][0]).toEqual([items[1], items[0]])
  })

  it('appends a picked media item from the panel', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: [] }
    })

    const panel = wrapper.findComponent({ name: 'MediaPanel' })
    await panel.vm.$emit('insert-image', {
      id: 1,
      url: '/uploads/media/new.webp',
      altText: 'New'
    })

    expect(wrapper.emitted('update:modelValue')![0][0]).toEqual([
      { url: '/uploads/media/new.webp', alt: 'New' }
    ])
  })

  it('disables adding at maxItems and shows the limit', () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: items, maxItems: 2 }
    })

    const addBtn = wrapper.findAll('button').filter((b) => b.text() === 'Add images')
    expect(addBtn[0].attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Maximum of 2 images reached.')
  })

  it('toggles the media panel with a flipping label', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: [] }
    })

    const toggle = () => wrapper.findAll('button').filter((b) => /Add images|Hide media/.test(b.text()))[0]!
    expect(toggle().text()).toBe('Add images')

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)
    expect(toggle().text()).toBe('Hide media')

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(false)
    expect(toggle().text()).toBe('Add images')
  })

  it('does not open the media panel when disabled', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: [], disabled: true }
    })

    const toggle = () => wrapper.findAll('button').filter((b) => /Add images|Hide media/.test(b.text()))[0]
    expect(toggle().attributes('disabled')).toBeDefined()

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(false)
  })

  it('coerces a legacy newline-separated string into one row per URL', () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: '/uploads/media/a.webp\n\n/uploads/media/b.webp\n' as unknown as GalleryItem[] }
    })

    expect(wrapper.findAll('.gallery-field__item')).toHaveLength(2)
    expect(wrapper.find('img').attributes('src')).toBe('/uploads/media/a.webp')
  })

  it('drops junk entries and emits a canonical array', async () => {
    const wrapper = mount(GalleryField, {
      props: { modelValue: [{ url: '  ', alt: 'blank' }, { nope: 1 }, '/uploads/media/c.webp'] as unknown as GalleryItem[] }
    })

    expect(wrapper.findAll('.gallery-field__item')).toHaveLength(1)
    await wrapper.findAll('.gallery-field__alt')[0]!.setValue('C')

    expect(wrapper.emitted('update:modelValue')![0]![0]).toEqual([
      { url: '/uploads/media/c.webp', alt: 'C' }
    ])
  })
})
