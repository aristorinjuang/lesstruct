import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import PluginGeneratorView from './PluginGeneratorView.vue'
import { usePlugingenStore, type GeneratedPlugin } from '@/stores/domain/plugingen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    postWithTimeout: vi.fn(),
  },
}))

const generatedPlugin: GeneratedPlugin = {
  name: 'my-plugin',
  language: 'go',
  rationale: 'Uppercases titles.',
  hooks: ['before_save'],
  capabilities: ['log_info'],
  files: { 'main.go': 'package main', 'go.mod': 'module x' },
}

describe('PluginGeneratorView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (
    health: 'ok' | 'missing' | 'error' = 'ok',
    build = false,
    omitBuildKey = false,
  ) => {
    if (health === 'error') {
      vi.mocked(api.get).mockRejectedValue(new Error('offline'))
    } else {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          features:
            health === 'ok'
              ? omitBuildKey
                ? { pluginGeneration: true }
                : { pluginGeneration: true, pluginBuild: build }
              : {},
        },
      })
    }

    const store = usePlugingenStore()
    const generate = vi.spyOn(store, 'generate').mockImplementation(async () => {
      store.project = generatedPlugin
      return store.project
    })
    const buildProject = vi.spyOn(store, 'build').mockResolvedValue(false)

    const wrapper = mount(PluginGeneratorView, {
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

    return { wrapper, store, generate, buildProject }
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

    expect(wrapper.text()).toContain('Plugin generation is not configured')
    expect(wrapper.find('#plugingen-description').exists()).toBe(false)
  })

  it('shows the configuration notice when the health check fails', async () => {
    const { wrapper } = await createWrapper('error')

    expect(wrapper.text()).toContain('Plugin generation is not configured')
  })

  it('generates a plugin with the selected hooks', async () => {
    const { wrapper, generate } = await createWrapper()

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith('Uppercase titles', ['before_save'], ['log_info'])
    expect(wrapper.text()).toContain('Generated plugin my-plugin')
  })

  it('toasts the first finding message on gate failure', async () => {
    const { wrapper, store, generate } = await createWrapper()
    generate.mockImplementation(async () => {
      store.error = Object.assign(new Error('Failed to generate plugin'), {
        details: {
          valid: false,
          findings: [
            {
              rule: 'go-hook',
              severity: 'error',
              message: 'main.go must export at least one hook',
            },
          ],
        },
      })
      throw store.error
    })

    await wrapper.find('#plugingen-description').setValue('A broken plugin')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Generation failed: main.go must export at least one hook')
  })

  it('requires a description before generating', async () => {
    const { wrapper, generate } = await createWrapper()

    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Description is required')
  })

  it('hides the build button when server builds are unavailable', async () => {
    const { wrapper } = await createWrapper()

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text() === 'Build & install')).toBe(false)
  })

  it('builds and installs the generated project', async () => {
    const { wrapper, buildProject } = await createWrapper('ok', true)

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Build & install').trigger('click')
    await flushPromises()

    expect(buildProject).toHaveBeenCalledWith('my-plugin')
    expect(wrapper.text()).toContain('Plugin installed and hot-reloaded.')
  })

  it('toasts the restart hint when the host needs a restart', async () => {
    const { wrapper, buildProject } = await createWrapper('ok', true)
    buildProject.mockResolvedValue(true)

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Build & install').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Plugin installed. Restart to apply.')
  })

  it('toasts build failures', async () => {
    const { wrapper, store, buildProject } = await createWrapper('ok', true)
    buildProject.mockImplementation(async () => {
      store.error = new Error('build unavailable')
      throw store.error
    })

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Build & install').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Build failed. Please try again.')
  })

  it('hides the build button when the health check omits the flag', async () => {
    const { wrapper } = await createWrapper('ok', false, true)

    await wrapper.find('#plugingen-description').setValue('Uppercase titles')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text() === 'Build & install')).toBe(false)
  })
})
