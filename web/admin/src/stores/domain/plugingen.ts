import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'

const generateTimeout = 130_000

export interface PluginFileMap {
  [path: string]: string
}

export interface GeneratedPlugin {
  name: string
  language: string
  rationale: string
  hooks: string[]
  capabilities: string[]
  files: PluginFileMap
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

export interface InstalledPlugin {
  name: string
  language: string
  hooks: string[]
  createdAt: string
  source: string
}

export interface LoadedPlugin {
  name: string
  status: string
  hooks: string[]
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
    if (
      candidate.severity !== 'error' ||
      typeof candidate.message !== 'string' ||
      candidate.message === ''
    ) {
      continue
    }
    return candidate.message
  }
  return ''
}

export const usePlugingenStore = defineStore('plugingen', () => {
  const project = ref<GeneratedPlugin | null>(null)
  const projects = ref<InstalledPlugin[]>([])
  const plugins = ref<LoadedPlugin[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function generate(
    description: string,
    hooks: string[],
    capabilities: string[] = [],
    language: string = 'go',
  ): Promise<GeneratedPlugin> {
    isLoading.value = true
    error.value = null
    project.value = null
    try {
      const response = await api.postWithTimeout<{
        data: { project: GeneratedPlugin; validation: ValidationReport }
      }>(
        '/api/v1/plugingen/generate',
        { description, language, hooks, capabilities },
        generateTimeout,
      )
      project.value = response.data.data.project
      if (!project.value) {
        throw new Error('Failed to generate plugin: No data returned')
      }
      return project.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchProjects(): Promise<InstalledPlugin[]> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.get<{ data: { projects: InstalledPlugin[] } }>(
        '/api/v1/plugingen/projects',
      )
      projects.value = response.data.data.projects ?? []
      return projects.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchStatus(): Promise<LoadedPlugin[]> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.get<{ data: { plugins: LoadedPlugin[] } }>('/api/v1/plugins')
      plugins.value = response.data.data.plugins ?? []
      return plugins.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function download(name: string): Promise<void> {
    await api.download(`/api/v1/plugingen/projects/${name}/download`, `${name}.zip`)
  }

  async function build(name: string): Promise<boolean> {
    isLoading.value = true
    error.value = null
    try {
      // Builds run past a minute: use the extended-timeout call like
      // generate does instead of the default 10s request timeout.
      const response = await api.postWithTimeout<{
        data: { installed: boolean; requiresRestart: boolean }
      }>(`/api/v1/plugingen/projects/${name}/build`, {}, 330_000)
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
      await api.delete(`/api/v1/plugingen/projects/${name}`)
      projects.value = projects.value.filter((p) => p.name !== name)
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    project,
    projects,
    plugins,
    isLoading,
    error,
    generate,
    fetchProjects,
    fetchStatus,
    download,
    build,
    remove,
  }
})
