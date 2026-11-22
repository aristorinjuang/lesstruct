import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import ModuleGeneratorView from './ModuleGeneratorView.vue'
import { useModgenStore, type GeneratedModule } from '@/stores/domain/modgen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    postWithTimeout: vi.fn(),
  },
}))

const generatedModule: GeneratedModule = {
  name: 'forum',
  rationale: 'Forum.',
  surfaces: ['routes'],
  files: { 'module.go': 'package forum', 'go.mod': 'module x' },
}

describe('ModuleGeneratorView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (health: 'ok' | 'missing' | 'error' = 'ok', apply = false) => {
    if (health === 'error') {
      vi.mocked(api.get).mockRejectedValue(new Error('offline'))
    } else {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          features: health === 'ok' ? { moduleGeneration: true, moduleApply: apply } : {},
        },
      })
    }

    const store = useModgenStore()
    const generate = vi.spyOn(store, 'generate').mockImplementation(async () => {
      store.project = generatedModule
      return store.project
    })
    const download = vi.spyOn(store, 'download').mockResolvedValue(undefined)
    const remove = vi.spyOn(store, 'remove').mockResolvedValue(undefined)
    const applyProject = vi.spyOn(store, 'apply').mockResolvedValue({
      project: generatedModule,
      apply: {
        applied: true,
        verified: true,
        steps: [],
        appliedModules: ['forum'],
        requiresRestart: true,
      },
    })

    const wrapper = mount(ModuleGeneratorView, {
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

    return { wrapper, store, generate, download, remove, applyProject }
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

    expect(wrapper.text()).toContain('Module generation is not configured')
    expect(wrapper.find('#modgen-description').exists()).toBe(false)
  })

  it('shows the configuration notice when the health check fails', async () => {
    const { wrapper } = await createWrapper('error')

    expect(wrapper.text()).toContain('Module generation is not configured')
  })

  it('generates a module with the selected surfaces', async () => {
    const { wrapper, generate } = await createWrapper()

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith('A forum', ['routes'])
    expect(wrapper.text()).toContain('Generated module forum')
  })

  it('toasts the first finding message on gate failure', async () => {
    const { wrapper, store, generate } = await createWrapper()
    generate.mockImplementation(async () => {
      store.error = Object.assign(new Error('Failed to generate module'), {
        details: {
          valid: false,
          findings: [
            {
              rule: 'module-go-parse',
              severity: 'error',
              message: 'module.go does not parse',
            },
          ],
        },
      })
      throw store.error
    })

    await wrapper.find('#modgen-description').setValue('A broken module')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Generation failed: module.go does not parse')
  })

  it('requires a description before generating', async () => {
    const { wrapper, generate } = await createWrapper()

    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(generate).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Description is required')
  })

  it('downloads and deletes the generated project', async () => {
    const { wrapper, download, remove } = await createWrapper()

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Download').trigger('click')
    await flushPromises()
    expect(download).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Downloaded forum.zip')

    await findButton(wrapper, 'Delete').trigger('click')
    await flushPromises()
    expect(remove).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Deleted forum')
  })

  it('hides the apply button when checkout applies are unavailable', async () => {
    const { wrapper } = await createWrapper()

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text() === 'Apply to checkout')).toBe(false)
  })

  it('applies the generated project when enabled', async () => {
    const { wrapper, applyProject } = await createWrapper('ok', true)

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Apply to checkout').trigger('click')
    await flushPromises()

    expect(applyProject).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Applied forum')
  })

  it('toasts failing checks', async () => {
    const { wrapper, applyProject } = await createWrapper('ok', true)
    applyProject.mockResolvedValue({
      project: generatedModule,
      apply: {
        applied: true,
        verified: false,
        steps: [{ name: 'tests', passed: false, skipped: false, output: 'FAIL' }],
        appliedModules: ['forum'],
        requiresRestart: true,
      },
    })

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Apply to checkout').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Applied with failing checks: tests')
  })

  it('toasts apply failures', async () => {
    const { wrapper, store, applyProject } = await createWrapper('ok', true)
    applyProject.mockImplementation(async () => {
      store.error = new Error('tree dirty')
      throw store.error
    })

    await wrapper.find('#modgen-description').setValue('A forum')
    await findButton(wrapper, 'Generate').trigger('click')
    await flushPromises()

    await findButton(wrapper, 'Apply to checkout').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Apply failed. Please try again.')
  })
})
