import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import HtmlCodeEditor from './HtmlCodeEditor.vue'

vi.mock('vue-codemirror', () => ({
  Codemirror: { name: 'Codemirror', template: '<div />', props: ['modelValue', 'placeholder', 'extensions', 'style'] },
}))

vi.mock('@codemirror/lang-html', () => ({
  html: vi.fn(() => ({})),
}))

vi.mock('@codemirror/theme-one-dark', () => ({
  oneDark: {},
}))

vi.mock('codemirror', () => ({}))

const stubs = {
  Button: { template: '<button @click="$emit(\'click\')"><slot /></button>', props: ['variant', 'size', 'isLoading'] },
}

describe('HtmlCodeEditor', () => {
  it('renders the codemirror editor', () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p>' },
      global: { stubs },
    })

    expect(wrapper.findComponent({ name: 'Codemirror' }).exists()).toBe(true)
  })

  it('emits update:modelValue when content changes', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p>' },
      global: { stubs },
    })

    const cm = wrapper.findComponent({ name: 'Codemirror' })
    await cm.vm.$emit('update:modelValue', '<p>Updated</p>')

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['<p>Updated</p>'])
  })

  it('toggles preview on button click', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p>' },
      global: { stubs },
    })

    expect(wrapper.find('.html-code-editor__preview').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'Codemirror' }).exists()).toBe(true)

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.html-code-editor__preview').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'Codemirror' }).exists()).toBe(false)
  })

  it('toggles back to code editor when preview is hidden', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p>' },
      global: { stubs },
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()
    expect(wrapper.findComponent({ name: 'Codemirror' }).exists()).toBe(false)

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()
    expect(wrapper.findComponent({ name: 'Codemirror' }).exists()).toBe(true)
    expect(wrapper.find('.html-code-editor__preview').exists()).toBe(false)
  })

  it('iframe has sandbox attribute without allow-scripts', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p>' },
      global: { stubs },
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()

    const iframe = wrapper.find('iframe')
    expect(iframe.exists()).toBe(true)
    const sandbox = iframe.attributes('sandbox')
    expect(sandbox).not.toContain('allow-scripts')
    expect(sandbox).toContain('allow-same-origin')
  })

  it('strips script tags from preview srcdoc', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<p>Hello</p><script>alert("xss")</script>' },
      global: { stubs },
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()

    const iframe = wrapper.find('iframe')
    const srcdoc = iframe.attributes('srcdoc')
    expect(srcdoc).not.toContain('<script')
    expect(srcdoc).toContain('<p>Hello</p>')
  })

  it('strips event handlers from preview srcdoc', async () => {
    const wrapper = mount(HtmlCodeEditor, {
      props: { modelValue: '<div onclick="alert(1)">Click</div>' },
      global: { stubs },
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).togglePreview()
    await wrapper.vm.$nextTick()

    const iframe = wrapper.find('iframe')
    const srcdoc = iframe.attributes('srcdoc')
    expect(srcdoc).not.toContain('onclick')
    expect(srcdoc).toContain('Click')
  })
})
