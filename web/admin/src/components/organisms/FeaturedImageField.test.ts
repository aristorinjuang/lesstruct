import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import FeaturedImageField from './FeaturedImageField.vue'

describe('FeaturedImageField', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('shows the set button when empty', () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '' }
    })

    expect(wrapper.text()).toContain('Set featured image')
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('shows the preview with change and remove actions when set', () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '/uploads/media/abc.webp' }
    })

    expect(wrapper.find('img').attributes('src')).toBe('/uploads/media/abc.webp')
    expect(wrapper.text()).toContain('Change')
    expect(wrapper.text()).toContain('Remove')
  })

  it('emits the picked media url', async () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '' }
    })

    const panel = wrapper.findComponent({ name: 'MediaPanel' })
    await panel.vm.$emit('insert-image', {
      id: 1,
      url: '/uploads/media/picked.webp',
      altText: 'Cover'
    })

    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['/uploads/media/picked.webp'])
  })

  it('closes the panel after picking', async () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '' }
    })

    await wrapper.findAll('button').filter((b) => b.text() === 'Set featured image')[0].trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)

    const panel = wrapper.findComponent({ name: 'MediaPanel' })
    await panel.vm.$emit('insert-image', { id: 1, url: '/x.webp', altText: '' })
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(false)
  })

  it('clears the value on remove', async () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '/uploads/media/abc.webp' }
    })

    await wrapper.findAll('button').filter((b) => b.text() === 'Remove')[0].trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([''])
  })

  it('toggles the media panel with a flipping label when empty', async () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '' }
    })

    const toggle = () => wrapper.findAll('button').filter((b) => /Set featured image|Hide media/.test(b.text()))[0]!
    expect(toggle().text()).toBe('Set featured image')

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)
    expect(toggle().text()).toBe('Hide media')

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(false)
    expect(toggle().text()).toBe('Set featured image')
  })

  it('toggles the media panel with a flipping change label when set', async () => {
    const wrapper = mount(FeaturedImageField, {
      props: { modelValue: '/uploads/media/abc.webp' }
    })

    const toggle = () => wrapper.findAll('button').filter((b) => /^(Change|Hide media)$/.test(b.text()))[0]!
    expect(toggle().text()).toBe('Change')

    await toggle().trigger('click')
    expect(wrapper.findComponent({ name: 'MediaPanel' }).props('isOpen')).toBe(true)
    expect(toggle().text()).toBe('Hide media')
  })
})
