import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'

const generateTimeout = 130_000
const generateTemplateTimeout = 190_000

export interface ThemeFileMap {
  [path: string]: string
}

export interface GeneratedTheme {
  name: string
  edition: string
  rationale: string
  files: ThemeFileMap
}

export interface ValidationFinding {
  rule: string
  severity: string
  message: string
}

export interface ValidationReport {
  valid: boolean
  findings: ValidationFinding[]
}

export interface InstalledTheme {
  name: string
  edition: string
  createdAt: string
  active: boolean
  source: string
}

// firstProblemFromError condenses a generation failure to its first
// error-severity finding message for the failure toast. The API reports
// gate failures in error.details; anything else yields an empty string.
export function firstProblemFromError(err: unknown): string {
  const details = (err as { details?: unknown } | null)?.details
  if (typeof details !== 'object' || details === null) {
    return ''
  }
  const findings = (details as { findings?: unknown }).findings
  if (!Array.isArray(findings)) {
    return ''
  }
  for (const finding of findings) {
    if (typeof finding !== 'object' || finding === null) {
      continue
    }
    const candidate = finding as { severity?: unknown; message?: unknown }
    if (candidate.severity !== 'error' || typeof candidate.message !== 'string' || candidate.message === '') {
      continue
    }
    return candidate.message
  }
  return ''
}

export const useThemegenStore = defineStore('themegen', () => {
  const theme = ref<GeneratedTheme | null>(null)
  const themes = ref<InstalledTheme[]>([])
  const previewTheme = ref<string | null>(null)
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function generate(
    description: string,
    style: string = '',
    templateOverrides: boolean = false,
    edition: string = '',
  ): Promise<GeneratedTheme> {
    isLoading.value = true
    error.value = null
    theme.value = null
    try {
      const response = await api.postWithTimeout<{ data: { theme: GeneratedTheme; validation: ValidationReport } }>(
        '/api/v1/themegen/generate',
        { description, style, edition, templateOverrides },
        templateOverrides ? generateTemplateTimeout : generateTimeout,
      )
      theme.value = response.data.data.theme
      if (!theme.value) {
        throw new Error('Failed to generate theme: No data returned')
      }
      return theme.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchThemes(): Promise<InstalledTheme[]> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.get<{ data: { themes: InstalledTheme[] } }>('/api/v1/themegen/themes')
      themes.value = response.data.data.themes ?? []
      return themes.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function download(name: string): Promise<void> {
    await api.download(`/api/v1/themegen/themes/${name}/download`, `${name}.zip`)
  }

  async function activate(name: string): Promise<boolean> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.post<{ data: { activated: boolean; requiresRestart: boolean } }>(
        `/api/v1/themegen/themes/${name}/activate`,
        {},
      )
      await fetchThemes()
      return response.data.data.requiresRestart
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function deactivate(): Promise<boolean> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.post<{ data: { activated: boolean; requiresRestart: boolean } }>(
        '/api/v1/themegen/themes/deactivate',
        {},
      )
      await fetchThemes()
      return response.data.data.requiresRestart
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function remove(name: string): Promise<void> {
    isLoading.value = true
    error.value = null
    try {
      await api.delete(`/api/v1/themegen/themes/${name}`)
      themes.value = themes.value.filter((t) => t.name !== name)
      // Deleting the previewed theme clears its cookie server-side, so the
      // local selection dies with it.
      if (previewTheme.value === name) {
        previewTheme.value = null
      }
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function preview(name: string): Promise<void> {
    isLoading.value = true
    error.value = null
    try {
      await api.post(`/api/v1/themegen/themes/${name}/preview`, {})
      previewTheme.value = name
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function stopPreview(): Promise<void> {
    isLoading.value = true
    error.value = null
    try {
      await api.delete('/api/v1/themegen/preview')
      previewTheme.value = null
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  // fetchPreviewStatus reads the server-side preview selection (the cookie is
  // HttpOnly, so the UI cannot inspect it directly). It never throws: a
  // failure simply reports no active preview.
  async function fetchPreviewStatus(): Promise<string | null> {
    try {
      const response = await api.get<{ data: { preview: boolean; theme?: string } }>(
        '/api/v1/themegen/preview',
      )
      previewTheme.value = response.data.data.preview ? (response.data.data.theme ?? null) : null
    } catch {
      previewTheme.value = null
    }
    return previewTheme.value
  }

  return {
    theme,
    themes,
    previewTheme,
    isLoading,
    error,
    generate,
    fetchThemes,
    fetchPreviewStatus,
    download,
    activate,
    deactivate,
    remove,
    preview,
    stopPreview,
  }
})
