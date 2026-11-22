import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useModgenStore, firstProblemFromError, type GeneratedModule } from './modgen'
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

const project: GeneratedModule = {
  name: 'forum',
  rationale: 'Forum.',
  surfaces: ['routes'],
  files: { 'module.go': 'package forum' },
}

describe('Modgen Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('generate', () => {
    it('posts a brief with surfaces', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { project, validation: { valid: true, findings: [] } } },
      })

      const store = useModgenStore()
      const result = await store.generate('A forum', ['routes'])

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/modgen/generate',
        {
          description: 'A forum',
          surfaces: ['routes'],
        },
        320_000,
      )
      expect(result).toEqual(project)
      expect(store.project).toEqual(project)
    })

    it('clears the previous result before generating again', async () => {
      vi.mocked(api.postWithTimeout).mockRejectedValue(new Error('boom'))

      const store = useModgenStore()
      store.project = project

      await expect(store.generate('A forum')).rejects.toThrow('boom')
      expect(store.project).toBeNull()
    })

    it('records the error on failure', async () => {
      const failure = Object.assign(new Error('Module failed validation'), {
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
      vi.mocked(api.postWithTimeout).mockRejectedValue(failure)

      const store = useModgenStore()

      await expect(store.generate('A forum')).rejects.toThrow('Module failed validation')
      expect(store.error).toEqual(failure)
    })
  })

  describe('fetchProjects', () => {
    it('loads the staged projects', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: {
            projects: [
              {
                name: 'forum',
                surfaces: ['routes'],
                fileCount: 9,
                createdAt: '2026-01-01T00:00:00Z',
                source: 'generated',
              },
            ],
          },
        },
      })

      const store = useModgenStore()
      const result = await store.fetchProjects()

      expect(api.get).toHaveBeenCalledWith('/api/v1/modgen/projects')
      expect(result).toHaveLength(1)
      expect(store.projects).toHaveLength(1)
    })

    it('records the error on failure', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('boom'))

      const store = useModgenStore()

      await expect(store.fetchProjects()).rejects.toThrow('boom')
      expect(store.error).toBeInstanceOf(Error)
    })
  })

  describe('download', () => {
    it('downloads the project zip', async () => {
      vi.mocked(api.download).mockResolvedValue(undefined)

      const store = useModgenStore()
      await store.download('forum')

      expect(api.download).toHaveBeenCalledWith(
        '/api/v1/modgen/projects/forum/download',
        'forum.zip',
      )
    })
  })

  describe('remove', () => {
    it('deletes the project and drops it from the list', async () => {
      vi.mocked(api.delete).mockResolvedValue({})
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: {
            projects: [{ name: 'forum', surfaces: [], fileCount: 3, createdAt: '', source: 'generated' }],
          },
        },
      })

      const store = useModgenStore()
      await store.fetchProjects()
      await store.remove('forum')

      expect(api.delete).toHaveBeenCalledWith('/api/v1/modgen/projects/forum')
      expect(store.projects).toHaveLength(0)
    })

    it('records the error on failure', async () => {
      vi.mocked(api.delete).mockRejectedValue(new Error('boom'))

      const store = useModgenStore()

      await expect(store.remove('forum')).rejects.toThrow('boom')
      expect(store.error).toBeInstanceOf(Error)
    })
  })

  describe('apply', () => {
    it('marks the project applied on success', async () => {
      const apply = {
        applied: true,
        verified: true,
        steps: [],
        appliedModules: ['forum'],
        requiresRestart: true,
      }
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { project, apply } },
      })
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: {
            projects: [
              {
                name: 'forum',
                surfaces: ['routes'],
                fileCount: 4,
                createdAt: '2026-01-01T00:00:00Z',
                source: 'generated',
                applied: false,
              },
            ],
          },
        },
      })

      const store = useModgenStore()
      await store.fetchProjects()
      const result = await store.apply('forum')

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/modgen/projects/forum/apply',
        { replace: false, confirm: false },
        920_000,
      )
      expect(result.apply).toEqual(apply)
      expect(store.projects[0]?.applied).toBe(true)
    })

    it('records the error on failure', async () => {
      vi.mocked(api.postWithTimeout).mockRejectedValue(new Error('boom'))

      const store = useModgenStore()

      await expect(store.apply('forum')).rejects.toThrow('boom')
      expect(store.error).toBeInstanceOf(Error)
    })
  })

  describe('unapply', () => {
    it('removes the applied tree with confirm', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: {
          data: {
            unapply: {
              removed: true,
              verified: true,
              steps: [],
              appliedModules: [],
              moduleDirRemoved: true,
            },
          },
        },
      })

      const store = useModgenStore()
      const result = await store.unapply('forum')

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/modgen/projects/forum/unapply',
        { confirm: true },
        920_000,
      )
      expect(result.removed).toBe(true)
      expect(result.moduleDirRemoved).toBe(true)
    })

    it('records the error on failure', async () => {
      vi.mocked(api.postWithTimeout).mockRejectedValue(new Error('boom'))

      const store = useModgenStore()

      await expect(store.unapply('forum')).rejects.toThrow('boom')
      expect(store.error).toBeInstanceOf(Error)
    })
  })

  describe('firstProblemFromError', () => {
    it('returns the first error finding message', () => {
      expect(
        firstProblemFromError({
          details: {
            findings: [
              { severity: 'warning', message: 'a warning' },
              { severity: 'error', message: 'module.go does not parse' },
            ],
          },
        }),
      ).toBe('module.go does not parse')
    })

    it('returns empty for missing details', () => {
      expect(firstProblemFromError(new Error('boom'))).toBe('')
      expect(firstProblemFromError(null)).toBe('')
      expect(firstProblemFromError({ details: 'nope' })).toBe('')
      expect(firstProblemFromError({ details: { findings: 'nope' } })).toBe('')
      expect(firstProblemFromError({ details: { findings: [null, { severity: 'error' }] } })).toBe(
        '',
      )
    })
  })
})
