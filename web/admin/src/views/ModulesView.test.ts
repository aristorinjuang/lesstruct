import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import ModulesView from './ModulesView.vue'
import { useModgenStore, type InstalledModule } from '@/stores/domain/modgen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
  },
}))

const stagedProject: InstalledModule = {
  name: 'forum',
  surfaces: ['routes'],
  fileCount: 9,
  createdAt: '2026-11-14T00:00:00Z',
  source: 'generated',
  applied: false,
}

describe('ModulesView', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
  })

  const createWrapper = async (applyEnabled = false, applied = false) => {
    vi.mocked(api.get).mockResolvedValue({
      data: { features: { moduleApply: applyEnabled } },
    })
    const store = useModgenStore()
    vi.spyOn(store, 'fetchProjects').mockImplementation(async () => {
      store.projects = [{ ...stagedProject, applied }]
      return store.projects
    })
    const download = vi.spyOn(store, 'download').mockResolvedValue(undefined)
    const remove = vi.spyOn(store, 'remove').mockImplementation(async (name: string) => {
      store.projects = store.projects.filter((p) => p.name !== name)
    })
    const apply = vi.spyOn(store, 'apply').mockResolvedValue({
      project: {
        name: 'forum',
        rationale: '',
        surfaces: ['routes'],
        files: {},
      },
      apply: {
        applied: true,
        verified: true,
        steps: [],
        appliedModules: ['forum'],
        requiresRestart: true,
      },
    })
    const unapply = vi.spyOn(store, 'unapply').mockResolvedValue({
      removed: true,
      verified: true,
      steps: [],
      appliedModules: [],
      moduleDirRemoved: true,
    })

    const wrapper = mount(ModulesView, {
      global: {
        plugins: [pinia],
        stubs: {
          Button: {
            emits: ['click'],
            template:
              '<button class="btn-stub" v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>',
          },
          DeleteConfirmDialog: {
            emits: ['confirm', 'cancel'],
            props: ['title', 'itemName', 'isOpen', 'warning'],
            template:
              '<div v-if="isOpen" class="confirm-stub"><button class="confirm-btn" @click="$emit(\'confirm\')">Confirm</button><button class="cancel-btn" @click="$emit(\'cancel\')">Cancel</button></div>',
          },
        },
      },
    })
    await flushPromises()

    return { wrapper, store, download, remove, apply, unapply }
  }

  const cardButtons = (wrapper: ReturnType<typeof mount>, name: string): string[] => {
    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes(name))
    if (!card) {
      throw new Error(`card for project ${name} not found`)
    }
    return card.findAll('button').map((b) => b.text())
  }

  it('renders a card per staged project', async () => {
    const { wrapper } = await createWrapper()

    expect(cardButtons(wrapper, 'forum')).toContain('Download')
    expect(wrapper.text()).toContain('routes · 9 files')
  })

  it('downloads a project', async () => {
    const { wrapper, download } = await createWrapper()

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const downloadButton = card?.findAll('button').find((b) => b.text() === 'Download')
    await downloadButton?.trigger('click')
    await flushPromises()

    expect(download).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Downloaded forum.zip')
  })

  it('deletes a project', async () => {
    const { wrapper, remove } = await createWrapper()

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const deleteButton = card?.findAll('button').find((b) => b.text() === 'Delete')
    await deleteButton?.trigger('click')
    await flushPromises()

    expect(remove).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Deleted forum')
  })

  it('warns when deleting a staged copy of an applied module', async () => {
    const { wrapper, remove } = await createWrapper(false, true)

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const deleteButton = card?.findAll('button').find((b) => b.text() === 'Delete')
    await deleteButton?.trigger('click')
    await flushPromises()

    expect(remove).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('stays applied in your checkout')
  })

  it('hides the remove button when checkout applies are unavailable', async () => {
    const { wrapper } = await createWrapper(false, true)

    expect(cardButtons(wrapper, 'forum')).not.toContain('Remove from checkout')
  })

  it('hides the remove button for unapplied projects', async () => {
    const { wrapper } = await createWrapper(true, false)

    expect(cardButtons(wrapper, 'forum')).not.toContain('Remove from checkout')
  })

  it('removes an applied module after confirming', async () => {
    const { wrapper, unapply } = await createWrapper(true, true)

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const removeButton = card?.findAll('button').find((b) => b.text() === 'Remove from checkout')
    await removeButton?.trigger('click')
    await flushPromises()

    expect(unapply).not.toHaveBeenCalled()

    const confirmButton = wrapper.find('.confirm-btn')
    await confirmButton.trigger('click')
    await flushPromises()

    expect(unapply).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Removed forum from the checkout')
  })

  it('cancels the remove dialog', async () => {
    const { wrapper, unapply } = await createWrapper(true, true)

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const removeButton = card?.findAll('button').find((b) => b.text() === 'Remove from checkout')
    await removeButton?.trigger('click')
    await flushPromises()

    const cancelButton = wrapper.find('.cancel-btn')
    await cancelButton.trigger('click')
    await flushPromises()

    expect(unapply).not.toHaveBeenCalled()
  })

  it('replaces an applied module after confirming', async () => {
    const { wrapper, apply } = await createWrapper(true, true)
    apply.mockResolvedValue({
      project: { name: 'forum', rationale: '', surfaces: ['routes'], files: {} },
      apply: {
        applied: true,
        verified: true,
        steps: [],
        appliedModules: ['forum'],
        requiresRestart: true,
      },
    })

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const replaceButton = card?.findAll('button').find((b) => b.text() === 'Replace applied')
    await replaceButton?.trigger('click')
    await flushPromises()

    expect(apply).not.toHaveBeenCalled()

    await wrapper.find('.confirm-btn').trigger('click')
    await flushPromises()

    expect(apply).toHaveBeenCalledWith('forum', { replace: true, confirm: true })
    expect(wrapper.text()).toContain('Replaced forum')
  })

  it('toasts replace failures', async () => {
    const { wrapper, store, apply } = await createWrapper(true, true)
    apply.mockImplementation(async () => {
      store.error = new Error('tree dirty')
      throw store.error
    })

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const replaceButton = card?.findAll('button').find((b) => b.text() === 'Replace applied')
    await replaceButton?.trigger('click')
    await flushPromises()

    await wrapper.find('.confirm-btn').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Replace failed')
    expect(store.error).toBeInstanceOf(Error)
  })

  it('toasts remove failures', async () => {
    const { wrapper, store, unapply } = await createWrapper(true, true)
    unapply.mockImplementation(async () => {
      store.error = new Error('tree dirty')
      throw store.error
    })

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const removeButton = card?.findAll('button').find((b) => b.text() === 'Remove from checkout')
    await removeButton?.trigger('click')
    await flushPromises()

    const confirmButton = wrapper.find('.confirm-btn')
    await confirmButton.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Remove failed')
  })

  it('hides the apply button when checkout applies are unavailable', async () => {
    const { wrapper } = await createWrapper()

    expect(cardButtons(wrapper, 'forum')).not.toContain('Apply to checkout')
  })

  it('applies a project when enabled', async () => {
    const { wrapper, apply } = await createWrapper(true)

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const applyButton = card?.findAll('button').find((b) => b.text() === 'Apply to checkout')
    await applyButton?.trigger('click')
    await flushPromises()

    expect(apply).toHaveBeenCalledWith('forum')
    expect(wrapper.text()).toContain('Applied forum')
  })

  it('toasts apply failures', async () => {
    const { wrapper, store, apply } = await createWrapper(true)
    apply.mockImplementation(async () => {
      store.error = new Error('tree dirty')
      throw store.error
    })

    const card = wrapper.findAll('.modules-card').find((c) => c.text().includes('forum'))
    const applyButton = card?.findAll('button').find((b) => b.text() === 'Apply to checkout')
    await applyButton?.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Apply failed')
  })
})
