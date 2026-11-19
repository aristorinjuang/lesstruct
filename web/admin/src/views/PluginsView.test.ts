import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import PluginsView from './PluginsView.vue'
import {
  usePlugingenStore,
  type InstalledPlugin,
  type LoadedPlugin,
} from '@/stores/domain/plugingen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
  },
}))

const generatedProject: InstalledPlugin = {
  name: 'my-plugin',
  language: 'go',
  hooks: ['before_save'],
  createdAt: '2026-11-14T00:00:00Z',
  source: 'generated',
}

const loadedPlugin: LoadedPlugin = {
  name: 'hello-world',
  status: 'loaded',
  hooks: ['before_save'],
}

describe('PluginsView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (buildEnabled = false, omitBuildKey = false) => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        features: omitBuildKey ? { pluginGeneration: true } : { pluginBuild: buildEnabled },
      },
    })
    const store = usePlugingenStore()
    vi.spyOn(store, 'fetchProjects').mockImplementation(async () => {
      store.projects = [generatedProject]
      return store.projects
    })
    vi.spyOn(store, 'fetchStatus').mockImplementation(async () => {
      store.plugins = [loadedPlugin]
      return store.plugins
    })
    const remove = vi.spyOn(store, 'remove').mockImplementation(async (name: string) => {
      store.projects = store.projects.filter((p) => p.name !== name)
    })
    const build = vi.spyOn(store, 'build').mockResolvedValue(false)

    const wrapper = mount(PluginsView, {
      global: {
        plugins: [pinia],
        stubs: {
          Button: {
            emits: ['click'],
            template:
              '<button class="btn-stub" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
          },
        },
      },
    })
    await flushPromises()

    return { wrapper, store, remove, build }
  }

  const cardButtons = (wrapper: ReturnType<typeof mount>, name: string): string[] => {
    const card = wrapper.findAll('.plugins-card').find((c) => c.text().includes(name))
    if (!card) {
      throw new Error(`card for project ${name} not found`)
    }
    return card.findAll('button').map((b) => b.text())
  }

  it('renders a card per installed project and loaded plugin', async () => {
    const { wrapper } = await createWrapper()

    expect(cardButtons(wrapper, 'my-plugin')).toContain('Download')
    expect(wrapper.text()).toContain('hello-world')
  })

  it('deletes a project', async () => {
    const { wrapper, remove } = await createWrapper()

    const card = wrapper.findAll('.plugins-card').find((c) => c.text().includes('my-plugin'))
    const deleteButton = card?.findAll('button').find((b) => b.text() === 'Delete')
    await deleteButton?.trigger('click')
    await flushPromises()

    expect(remove).toHaveBeenCalledWith('my-plugin')
    expect(wrapper.text()).toContain('Deleted my-plugin')
  })

  it('marks projects that are installed on the host', async () => {
    const { wrapper, store } = await createWrapper(true)
    store.plugins = [{ name: 'my-plugin', status: 'loaded', hooks: ['before_save'] }]
    await flushPromises()

    expect(wrapper.text()).toContain('installed')
  })

  it('builds a project when server builds are enabled', async () => {
    const { wrapper, build } = await createWrapper(true)

    const card = wrapper.findAll('.plugins-card').find((c) => c.text().includes('my-plugin'))
    const buildButton = card?.findAll('button').find((b) => b.text() === 'Build & install')
    await buildButton?.trigger('click')
    await flushPromises()

    expect(build).toHaveBeenCalledWith('my-plugin')
    expect(wrapper.text()).toContain('Plugin installed and hot-reloaded.')
  })

  it('hides the build button when server builds are unavailable', async () => {
    const { wrapper } = await createWrapper(false)

    expect(cardButtons(wrapper, 'my-plugin')).not.toContain('Build & install')
  })

  it('hides the build button when the health check omits the flag', async () => {
    const { wrapper } = await createWrapper(false, true)

    expect(cardButtons(wrapper, 'my-plugin')).not.toContain('Build & install')
  })
})
