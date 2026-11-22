import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import ThemesView from './ThemesView.vue'
import { useThemegenStore, type InstalledTheme } from '@/stores/domain/themegen'

const activeTheme: InstalledTheme = {
  name: 'midnight-serif',
  edition: 'blog',
  createdAt: '2026-11-14T00:00:00Z',
  active: true,
  source: 'generated',
}

const inactiveTheme: InstalledTheme = {
  name: 'paper-light',
  edition: 'blog',
  createdAt: '2026-11-14T00:00:00Z',
  active: false,
  source: 'generated',
}

describe('ThemesView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (
    deactivateImpl: () => Promise<boolean> = async () => false,
    previewTheme: string | null = null,
  ) => {
    const store = useThemegenStore()
    vi.spyOn(store, 'fetchThemes').mockImplementation(async () => {
      store.themes = [activeTheme, inactiveTheme]
      return store.themes
    })
    vi.spyOn(store, 'fetchPreviewStatus').mockImplementation(async () => {
      store.previewTheme = previewTheme
      return store.previewTheme
    })
    const deactivate = vi.spyOn(store, 'deactivate').mockImplementation(deactivateImpl)
    const preview = vi.spyOn(store, 'preview').mockImplementation(async (name: string) => {
      store.previewTheme = name
    })
    const stopPreview = vi.spyOn(store, 'stopPreview').mockImplementation(async () => {
      store.previewTheme = null
    })

    const wrapper = mount(ThemesView, {
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

    return { wrapper, store, deactivate, preview, stopPreview }
  }

  const cardButtons = (wrapper: ReturnType<typeof mount>, name: string): string[] => {
    const card = wrapper.findAll('.themes-card').find((c) => c.text().includes(name))
    if (!card) {
      throw new Error(`card for theme ${name} not found`)
    }
    return card.findAll('button').map((b) => b.text())
  }

  const stopButton = (wrapper: ReturnType<typeof mount>) =>
    wrapper.findAll('button').find((b) => b.text().startsWith('Stop preview'))

  it('renders a card per installed theme', async () => {
    const { wrapper } = await createWrapper()

    expect(wrapper.findAll('.themes-card')).toHaveLength(2)
    expect(wrapper.text()).toContain('midnight-serif')
    expect(wrapper.text()).toContain('paper-light')
  })

  it('frames a sandboxed snapshot per theme card', async () => {
    const { wrapper } = await createWrapper()

    const frames = wrapper.findAll('.theme-snapshot iframe')
    expect(frames).toHaveLength(2)
    expect(frames[0]?.attributes('src')).toBe('/api/v1/themegen/themes/midnight-serif/snapshot')
    expect(frames[1]?.attributes('src')).toBe('/api/v1/themegen/themes/paper-light/snapshot')
    for (const frame of frames) {
      expect(frame.attributes('sandbox')).toBe('allow-same-origin')
      expect(frame.attributes('loading')).toBe('lazy')
    }
  })

  it('shows Deactivate only on the active theme', async () => {
    const { wrapper } = await createWrapper()

    expect(cardButtons(wrapper, 'midnight-serif')).toContain('Deactivate')
    expect(cardButtons(wrapper, 'midnight-serif')).not.toContain('Delete')
    expect(cardButtons(wrapper, 'paper-light')).toContain('Activate')
    expect(cardButtons(wrapper, 'paper-light')).toContain('Delete')
    expect(cardButtons(wrapper, 'paper-light')).not.toContain('Deactivate')
  })

  it('deactivates and toasts when the theme applied immediately', async () => {
    const { wrapper, deactivate } = await createWrapper(async () => false)

    const buttons = wrapper
      .findAll('.themes-card')
      .find((c) => c.text().includes('midnight-serif'))!
      .findAll('button')
    await buttons.find((b) => b.text() === 'Deactivate')!.trigger('click')
    await flushPromises()

    expect(deactivate).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Embedded theme applied.')
  })

  it('toasts the restart hint when deactivation needs a restart', async () => {
    const { wrapper } = await createWrapper(async () => true)

    const buttons = wrapper
      .findAll('.themes-card')
      .find((c) => c.text().includes('midnight-serif'))!
      .findAll('button')
    await buttons.find((b) => b.text() === 'Deactivate')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Restart to apply')
  })

  it('toasts an error when deactivation fails', async () => {
    const { wrapper, store } = await createWrapper()
    vi.spyOn(store, 'deactivate').mockRejectedValue(new Error('boom'))

    const buttons = wrapper
      .findAll('.themes-card')
      .find((c) => c.text().includes('midnight-serif'))!
      .findAll('button')
    await buttons.find((b) => b.text() === 'Deactivate')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Deactivation failed')
  })

  it('shows Preview on every theme card', async () => {
    const { wrapper } = await createWrapper()

    expect(cardButtons(wrapper, 'midnight-serif')).toContain('Preview')
    expect(cardButtons(wrapper, 'paper-light')).toContain('Preview')
  })

  it('opens the preview and toasts its expiry', async () => {
    const openSpy = vi.fn().mockReturnValue(null)
    Object.defineProperty(window, 'open', { configurable: true, writable: true, value: openSpy })
    const { wrapper, preview } = await createWrapper()

    const buttons = wrapper
      .findAll('.themes-card')
      .find((c) => c.text().includes('paper-light'))!
      .findAll('button')
    await buttons.find((b) => b.text() === 'Preview')!.trigger('click')
    await flushPromises()

    expect(preview).toHaveBeenCalledWith('paper-light')
    expect(openSpy).toHaveBeenCalledWith('/', '_blank', 'noopener')
    expect(wrapper.text()).toContain('valid 15 minutes')
  })

  it('reveals the toolbar after previewing a theme', async () => {
    const openSpy = vi.fn().mockReturnValue(null)
    Object.defineProperty(window, 'open', { configurable: true, writable: true, value: openSpy })
    const { wrapper, preview } = await createWrapper()

    expect(stopButton(wrapper)).toBeUndefined()

    const buttons = wrapper
      .findAll('.themes-card')
      .find((c) => c.text().includes('paper-light'))!
      .findAll('button')
    await buttons.find((b) => b.text() === 'Preview')!.trigger('click')
    await flushPromises()

    expect(preview).toHaveBeenCalledWith('paper-light')
    expect(stopButton(wrapper)?.text()).toBe('Stop preview (paper-light)')
  })

  it('names the previewed theme in the toolbar and badges its card', async () => {
    const { wrapper } = await createWrapper(async () => false, 'paper-light')

    expect(stopButton(wrapper)?.text()).toBe('Stop preview (paper-light)')
    const paperCard = wrapper.findAll('.themes-card').find((c) => c.text().includes('paper-light'))!
    expect(paperCard.text()).toContain('Previewing in this browser')
    const midnightCard = wrapper.findAll('.themes-card').find((c) => c.text().includes('midnight-serif'))!
    expect(midnightCard.text()).not.toContain('Previewing in this browser')
  })

  it('hides the stop-preview toolbar when no preview is active', async () => {
    const { wrapper } = await createWrapper()

    expect(stopButton(wrapper)).toBeUndefined()
  })

  it('stops the preview from the toolbar', async () => {
    const { wrapper, stopPreview } = await createWrapper(async () => false, 'paper-light')

    await stopButton(wrapper)!.trigger('click')
    await flushPromises()

    expect(stopPreview).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Preview stopped.')
    expect(stopButton(wrapper)).toBeUndefined()
  })
})
