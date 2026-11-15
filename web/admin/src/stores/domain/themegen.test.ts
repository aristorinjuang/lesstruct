import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useThemegenStore, firstProblemFromError, type GeneratedTheme, type ValidationReport } from './themegen'
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

const theme: GeneratedTheme = {
  name: 'midnight-serif',
  edition: 'blog',
  rationale: 'A dark editorial theme.',
  files: { 'static/base.css': ':root{}' },
}

const report: ValidationReport = {
  valid: true,
  findings: [{ rule: 'css-url-external', severity: 'warning', message: 'external URL' }],
}

describe('Themegen Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('generate', () => {
    it('posts a stylesheet-only brief with the short timeout', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { theme, validation: report } },
      })

      const store = useThemegenStore()
      const result = await store.generate('A dark theme', 'dark')

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/themegen/generate',
        { description: 'A dark theme', style: 'dark', edition: '', templateOverrides: false },
        130_000,
      )
      expect(result).toEqual(theme)
      expect(store.theme).toEqual(theme)
    })

    it('requests template overrides with the long timeout', async () => {
      vi.mocked(api.postWithTimeout).mockResolvedValue({
        data: { data: { theme, validation: report } },
      })

      const store = useThemegenStore()
      await store.generate('A dark theme', '', true)

      expect(api.postWithTimeout).toHaveBeenCalledWith(
        '/api/v1/themegen/generate',
        { description: 'A dark theme', style: '', edition: '', templateOverrides: true },
        190_000,
      )
    })

    it('clears the previous result before generating again', async () => {
      vi.mocked(api.postWithTimeout).mockRejectedValue(new Error('boom'))

      const store = useThemegenStore()
      store.theme = theme

      await expect(store.generate('A dark theme')).rejects.toThrow('boom')
      expect(store.theme).toBeNull()
    })

    it('records the error on failure', async () => {
      const failure = Object.assign(new Error('Theme failed validation after 2 repair attempts'), {
        details: {
          valid: false,
          findings: [{ rule: 'template-render', severity: 'error', message: 'index.html: unknown field' }],
        },
      })
      vi.mocked(api.postWithTimeout).mockRejectedValue(failure)

      const store = useThemegenStore()

      await expect(store.generate('A broken theme', '', true)).rejects.toThrow(
        'Theme failed validation after 2 repair attempts',
      )
      expect(store.error).toBe(failure)
      expect(store.theme).toBeNull()
    })
  })

  describe('firstProblemFromError', () => {
    it('returns the first error finding message', async () => {
      const failure = Object.assign(new Error('Theme failed validation after 2 repair attempts'), {
        details: {
          valid: false,
          findings: [{ rule: 'css-parse', severity: 'error', message: 'static/style.css has unbalanced braces' }],
        },
      })

      expect(firstProblemFromError(failure)).toBe('static/style.css has unbalanced braces')
    })

    it('skips warnings to find the first error', async () => {
      const failure = Object.assign(new Error('failed'), {
        details: {
          valid: false,
          findings: [
            { rule: 'css-url-external', severity: 'warning', message: 'external URL' },
            { rule: 'template-render', severity: 'error', message: 'index.html: unknown field' },
          ],
        },
      })

      expect(firstProblemFromError(failure)).toBe('index.html: unknown field')
    })

    it('returns empty when no error finding exists', async () => {
      const failure = Object.assign(new Error('failed'), {
        details: {
          valid: false,
          findings: [{ rule: 'css-url-external', severity: 'warning', message: 'external URL' }],
        },
      })

      expect(firstProblemFromError(failure)).toBe('')
    })

    it('returns empty for unrelated error shapes', async () => {
      expect(firstProblemFromError(new Error('boom'))).toBe('')
      expect(firstProblemFromError(Object.assign(new Error('busy'), { details: { retryAfter: 5 } }))).toBe('')
      expect(firstProblemFromError(Object.assign(new Error('bad'), { details: 'nope' }))).toBe('')
    })
  })

  describe('preview', () => {
    it('records the previewed theme on success', async () => {
      vi.mocked(api.post).mockResolvedValue({ data: { data: { preview: true } } })

      const store = useThemegenStore()
      await store.preview('paper-light')

      expect(api.post).toHaveBeenCalledWith('/api/v1/themegen/themes/paper-light/preview', {})
      expect(store.previewTheme).toBe('paper-light')
    })

    it('leaves no preview recorded on failure', async () => {
      vi.mocked(api.post).mockRejectedValue(new Error('boom'))

      const store = useThemegenStore()

      await expect(store.preview('paper-light')).rejects.toThrow('boom')
      expect(store.previewTheme).toBeNull()
    })
  })

  describe('stopPreview', () => {
    it('clears the recorded preview on success', async () => {
      vi.mocked(api.delete).mockResolvedValue({ data: { data: { preview: false } } })

      const store = useThemegenStore()
      store.previewTheme = 'paper-light'
      await store.stopPreview()

      expect(api.delete).toHaveBeenCalledWith('/api/v1/themegen/preview')
      expect(store.previewTheme).toBeNull()
    })

    it('keeps the recorded preview on failure', async () => {
      vi.mocked(api.delete).mockRejectedValue(new Error('boom'))

      const store = useThemegenStore()
      store.previewTheme = 'paper-light'

      await expect(store.stopPreview()).rejects.toThrow('boom')
      expect(store.previewTheme).toBe('paper-light')
    })
  })

  describe('remove', () => {
    it('clears the recorded preview when removing the previewed theme', async () => {
      vi.mocked(api.delete).mockResolvedValue({ data: { data: { deleted: true } } })

      const store = useThemegenStore()
      store.themes = [
        { name: 'paper-light', edition: 'blog', createdAt: '2026-11-14T00:00:00Z', active: false, source: 'generated' },
      ]
      store.previewTheme = 'paper-light'
      await store.remove('paper-light')

      expect(store.themes).toEqual([])
      expect(store.previewTheme).toBeNull()
    })

    it('keeps the recorded preview when removing another theme', async () => {
      vi.mocked(api.delete).mockResolvedValue({ data: { data: { deleted: true } } })

      const store = useThemegenStore()
      store.themes = [
        { name: 'paper-light', edition: 'blog', createdAt: '2026-11-14T00:00:00Z', active: false, source: 'generated' },
        { name: 'midnight-serif', edition: 'blog', createdAt: '2026-11-14T00:00:00Z', active: false, source: 'generated' },
      ]
      store.previewTheme = 'paper-light'
      await store.remove('midnight-serif')

      expect(store.themes.map((t) => t.name)).toEqual(['paper-light'])
      expect(store.previewTheme).toBe('paper-light')
    })
  })

  describe('fetchPreviewStatus', () => {
    it('records the previewed theme when a preview is active', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: { data: { preview: true, theme: 'paper-light' } },
      })

      const store = useThemegenStore()
      const result = await store.fetchPreviewStatus()

      expect(api.get).toHaveBeenCalledWith('/api/v1/themegen/preview')
      expect(result).toBe('paper-light')
      expect(store.previewTheme).toBe('paper-light')
    })

    it('records no preview when none is active', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: { data: { preview: false } },
      })

      const store = useThemegenStore()
      store.previewTheme = 'paper-light'
      const result = await store.fetchPreviewStatus()

      expect(result).toBeNull()
      expect(store.previewTheme).toBeNull()
    })

    it('records no preview without throwing on failure', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('offline'))

      const store = useThemegenStore()
      store.previewTheme = 'paper-light'
      const result = await store.fetchPreviewStatus()

      expect(result).toBeNull()
      expect(store.previewTheme).toBeNull()
    })
  })
})
