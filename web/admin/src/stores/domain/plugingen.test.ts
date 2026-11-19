import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePlugingenStore, firstProblemFromError, type GeneratedPlugin } from './plugingen'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    postWithTimeout: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    download: vi.fn(),
  },
}))

const project: GeneratedPlugin = {
  name: 'my-plugin',
  language: 'go',
  rationale: 'Uppercases titles.',
  hooks: ['before_save'],
  capabilities: ['log_info'],
  files: { 'main.go': 'package main' },
}

describe('Plugingen Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('generate', () => {
    it('posts a brief with hooks and capabilities', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { project, validation: { valid: true, findings: [] } } },
      })

      const store = usePlugingenStore()
      const result = await store.generate('Uppercase titles', ['before_save'], ['log_info'])

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/plugingen/generate',
        {
          description: 'Uppercase titles',
          language: 'go',
          hooks: ['before_save'],
          capabilities: ['log_info'],
        },
        130_000,
      )
      expect(result).toEqual(project)
      expect(store.project).toEqual(project)
    })

    it('clears the previous result before generating again', async () => {
      vi.mocked(api.postWithTimeout).mockRejectedValue(new Error('boom'))

      const store = usePlugingenStore()
      store.project = project

      await expect(store.generate('Uppercase titles', ['before_save'])).rejects.toThrow('boom')
      expect(store.project).toBeNull()
    })

    it('records the error on failure', async () => {
      const failure = Object.assign(new Error('Plugin failed validation'), {
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
      vi.mocked(api.postWithTimeout).mockRejectedValue(failure)

      const store = usePlugingenStore()

      await expect(store.generate('A broken plugin', ['before_save'])).rejects.toThrow(
        'Plugin failed validation',
      )
      expect(store.error).toBe(failure)
      expect(store.project).toBeNull()
    })
  })

  describe('firstProblemFromError', () => {
    it('returns the first error finding message', async () => {
      const failure = Object.assign(new Error('Plugin failed validation'), {
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

      expect(firstProblemFromError(failure)).toBe('main.go must export at least one hook')
    })

    it('skips warnings to find the first error', async () => {
      const failure = Object.assign(new Error('failed'), {
        details: {
          valid: false,
          findings: [
            { rule: 'project-unsafe', severity: 'warning', message: 'uses unsafe' },
            { rule: 'go-parse', severity: 'error', message: 'main.go does not parse' },
          ],
        },
      })

      expect(firstProblemFromError(failure)).toBe('main.go does not parse')
    })

    it('returns empty when no error finding exists', async () => {
      const failure = Object.assign(new Error('failed'), {
        details: {
          valid: false,
          findings: [{ rule: 'project-unsafe', severity: 'warning', message: 'uses unsafe' }],
        },
      })

      expect(firstProblemFromError(failure)).toBe('')
    })

    it('returns empty for unrelated error shapes', async () => {
      expect(firstProblemFromError(new Error('boom'))).toBe('')
      expect(
        firstProblemFromError(Object.assign(new Error('busy'), { details: { retryAfter: 5 } })),
      ).toBe('')
      expect(firstProblemFromError(Object.assign(new Error('bad'), { details: 'nope' }))).toBe('')
    })
  })

  describe('fetchProjects', () => {
    it('stores the installed projects', async () => {
      const installed = [
        {
          name: 'my-plugin',
          language: 'go',
          hooks: ['before_save'],
          createdAt: '2026-01-01T00:00:00Z',
          source: 'generated',
        },
      ]
      vi.mocked(api.get).mockResolvedValue({ data: { data: { projects: installed } } })

      const store = usePlugingenStore()
      const result = await store.fetchProjects()

      expect(api.get).toHaveBeenCalledWith('/api/v1/plugingen/projects')
      expect(result).toEqual(installed)
      expect(store.projects).toEqual(installed)
    })
  })

  describe('fetchStatus', () => {
    it('stores the loaded plugins', async () => {
      const loaded = [{ name: 'hello-world', status: 'loaded', hooks: ['before_save'] }]
      vi.mocked(api.get).mockResolvedValue({ data: { data: { plugins: loaded } } })

      const store = usePlugingenStore()
      const result = await store.fetchStatus()

      expect(api.get).toHaveBeenCalledWith('/api/v1/plugins')
      expect(result).toEqual(loaded)
      expect(store.plugins).toEqual(loaded)
    })
  })

  describe('remove', () => {
    it('removes the project from the list', async () => {
      vi.mocked(api.delete).mockResolvedValue({ data: { data: { deleted: true } } })

      const store = usePlugingenStore()
      store.projects = [
        {
          name: 'my-plugin',
          language: 'go',
          hooks: ['before_save'],
          createdAt: '2026-01-01T00:00:00Z',
          source: 'generated',
        },
      ]
      await store.remove('my-plugin')

      expect(api.delete).toHaveBeenCalledWith('/api/v1/plugingen/projects/my-plugin')
      expect(store.projects).toEqual([])
    })
  })

  describe('build', () => {
    it('posts with the extended timeout', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { installed: true, requiresRestart: true } },
      })

      const store = usePlugingenStore()
      const result = await store.build('my-plugin')

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/plugingen/projects/my-plugin/build',
        {},
        330_000,
      )
      expect(result).toBe(true)
    })

    it('records the error on failure', async () => {
      const failure = new Error('build unavailable')
      vi.mocked(api.postWithTimeout).mockRejectedValue(failure)

      const store = usePlugingenStore()

      await expect(store.build('my-plugin')).rejects.toThrow('build unavailable')
      expect(store.error).toBe(failure)
    })
  })
})
