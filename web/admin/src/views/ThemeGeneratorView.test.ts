import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import ThemeGeneratorView from './ThemeGeneratorView.vue'
import { useThemegenStore, type GeneratedTheme } from '@/stores/domain/themegen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    postWithTimeout: vi.fn(),
  },
}))

const generatedTheme: GeneratedTheme = {
  name: 'midnight-serif',
  edition: 'blog',
  rationale: 'A dark editorial theme.',
  files: { 'static/base.css': ':root{}' },
}

describe('ThemeGeneratorView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (health: 'ok' | 'missing' | 'error' = 'ok') => {
    if (health === 'error') {
      vi.mocked(api.get).mockRejectedValue(new Error('offline'))
    } else {
      vi.mocked(api.get).mockResolvedValue({
        data: { features: { themeGeneration: health === 'ok' } },
      })
    }

    const store = useThemegenStore()
    const generate = vi.spyOn(store, 'generate').mockImplementation(async () => {
      store.theme = generatedTheme
      return store.theme
    })
    vi.spyOn(store, 'fetchPreviewStatus').mockImplementation(async () => {
      store.previewTheme = null
      return null
    })

    const wrapper = mount(ThemeGeneratorView, {
      global: {
        plugins: [pinia],
        stubs: {
          Button: {
            emits: ['click'],
            template: '<button class="btn-stub" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
          },
        },
      },
    })
    await flushPromises()

    return { wrapper, store, generate }
  }

  const findButton = (wrapper: ReturnType<typeof mount>, label: string) => {
    const button = wrapper.findAll('button').find((b) => b.text() === label)
    if (!button) {
      throw new Error(`button ${label} not found`)
    }
    return button
  }

  it('shows the configuration notice when generation is unavailable', async () => {
    const { wrapper } = await createWrapper('missing')

    expect(wrapper.text()).toContain('Theme generation is not configured')
    expect(wrapper.find('#themegen-description').exists()).toBe(false)
  })

  it('shows the configuration notice when the health check fails', async () => {
    const { wrapper } = await createWrapper('error')

    expect(wrapper.text()).toContain('Theme generation is not configured')
  })

  it('generates a stylesheet-only theme by default', async () => {
    const { wrapper, generate } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('A clean blog theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith('A clean blog theme', '', false)
    expect(wrapper.text()).toContain('Generated theme midnight-serif')
  })

  it('forwards the template-overrides opt-in', async () => {
    const { wrapper, generate } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('An editorial theme with custom templates')
    await wrapper.find('#themegen-template-overrides').setValue(true)
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith('An editorial theme with custom templates', '', true)
  })

  it('toasts the first finding message on gate failure', async () => {
    const { wrapper, store, generate } = await createWrapper()
    generate.mockImplementation(async () => {
      store.error = Object.assign(new Error('Failed to generate theme'), {
        details: {
          valid: false,
          findings: [{ rule: 'css-parse', severity: 'error', message: 'static/style.css has unbalanced braces' }],
        },
      })
      throw store.error
    })

    await wrapper.find('#themegen-description').setValue('A broken theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Generation failed: static/style.css has unbalanced braces')
    expect(wrapper.text()).not.toContain('Validation')
  })

  it('falls back to the error message without finding details', async () => {
    const { wrapper, store, generate } = await createWrapper()
    generate.mockImplementation(async () => {
      store.error = new Error('Theme generation is busy')
      throw store.error
    })

    await wrapper.find('#themegen-description').setValue('A broken theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Theme generation is busy')
  })

  it('highlights the auto preset by default and moves the selection on click', async () => {
    const { wrapper } = await createWrapper()

    expect(findButton(wrapper, 'auto').attributes('aria-pressed')).toBe('true')
    expect(findButton(wrapper, 'dark').attributes('aria-pressed')).toBe('false')

    await findButton(wrapper, 'dark').trigger('click')

    expect(findButton(wrapper, 'dark').attributes('aria-pressed')).toBe('true')
    expect(findButton(wrapper, 'auto').attributes('aria-pressed')).toBe('false')
  })

  it('toggles the preset off when clicking the selected chip', async () => {
    const { wrapper } = await createWrapper()

    await findButton(wrapper, 'dark').trigger('click')
    expect(findButton(wrapper, 'dark').attributes('aria-pressed')).toBe('true')

    await findButton(wrapper, 'dark').trigger('click')
    expect(findButton(wrapper, 'dark').attributes('aria-pressed')).toBe('false')
    expect(findButton(wrapper, 'auto').attributes('aria-pressed')).toBe('true')
  })

  it('generates with the selected style preset', async () => {
    const { wrapper, generate } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('A dark theme')
    await findButton(wrapper, 'dark').trigger('click')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith('A dark theme', 'dark', false)
  })

  it('hides the stop-preview button when no preview is active', async () => {
    const { wrapper } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('A clean blog theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().startsWith('Stop preview'))).toBe(false)
  })

  it('frames the generated theme snapshot', async () => {
    const { wrapper } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('A clean blog theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    const frame = wrapper.find('.themegen-snapshot-frame iframe')
    expect(frame.exists()).toBe(true)
    expect(frame.attributes('src')).toBe('/api/v1/themegen/themes/midnight-serif/snapshot')
    expect(frame.attributes('sandbox')).toBe('allow-same-origin')
  })

  it('names the previewed theme in the stop-preview button', async () => {
    const { wrapper, store } = await createWrapper()

    await wrapper.find('#themegen-description').setValue('A clean blog theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    store.previewTheme = 'midnight-serif'
    await flushPromises()

    const stopButton = wrapper.findAll('button').find((b) => b.text().startsWith('Stop preview'))
    expect(stopButton?.text()).toBe('Stop preview (midnight-serif)')
  })

  it('clears the stop-preview button after stopping', async () => {
    const { wrapper, store } = await createWrapper()
    vi.spyOn(store, 'stopPreview').mockImplementation(async () => {
      store.previewTheme = null
    })

    await wrapper.find('#themegen-description').setValue('A clean blog theme')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    store.previewTheme = 'midnight-serif'
    await flushPromises()

    await findButton(wrapper, 'Stop preview (midnight-serif)').trigger('click')
    await flushPromises()

    expect(store.stopPreview).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('button').some((b) => b.text().startsWith('Stop preview'))).toBe(false)
  })
})
