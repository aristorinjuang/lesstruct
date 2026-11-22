import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'

const generateTimeout = 320_000

// applyTimeout bounds one checkout apply or unapply from the browser: the
// server budget is 15 minutes (module tests plus the custom-binary build),
// so the client must outlive it.
const applyTimeout = 920_000

export interface ModuleFileMap {
  [path: string]: string
}

export interface GeneratedModule {
  name: string
  surfaces: string[]
  rationale: string
  files: ModuleFileMap
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

export interface InstalledModule {
  name: string
  surfaces: string[]
  fileCount: number
  createdAt: string
  source: string
  applied: boolean
}

export interface ApplyStep {
  name: string
  passed: boolean
  skipped: boolean
  output?: string
}

export interface ApplyResult {
  applied: boolean
  verified: boolean
  steps: ApplyStep[]
  appliedModules: string[]
  requiresRestart: boolean
}

// UnapplyResult is the outcome of removing an applied module: whether
// anything was removed, whether the post-removal build passed, and whether
// the module tree itself was deleted.
export interface UnapplyResult {
  removed: boolean
  verified: boolean
  steps: ApplyStep[]
  appliedModules: string[]
  moduleDirRemoved: boolean
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

export const useModgenStore = defineStore('modgen', () => {
  const project = ref<GeneratedModule | null>(null)
  const projects = ref<InstalledModule[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function generate(description: string, surfaces: string[] = []): Promise<GeneratedModule> {
    isLoading.value = true
    error.value = null
    project.value = null
    try {
      const response = await api.postWithTimeout<{
        data: { project: GeneratedModule; validation: ValidationReport }
      }>('/api/v1/modgen/generate', { description, surfaces }, generateTimeout)
      project.value = response.data.data.project
      if (!project.value) {
        throw new Error('Failed to generate module: No data returned')
      }
      return project.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchProjects(): Promise<InstalledModule[]> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.get<{ data: { projects: InstalledModule[] } }>(
        '/api/v1/modgen/projects',
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

  async function download(name: string): Promise<void> {
    await api.download(`/api/v1/modgen/projects/${name}/download`, `${name}.zip`)
  }

  async function remove(name: string): Promise<void> {
    isLoading.value = true
    error.value = null
    try {
      await api.delete(`/api/v1/modgen/projects/${name}`)
      projects.value = projects.value.filter((p) => p.name !== name)
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function apply(
    name: string,
    opts: { replace?: boolean; confirm?: boolean } = {},
  ): Promise<{ project: GeneratedModule; apply: ApplyResult }> {
    isLoading.value = true
    error.value = null
    try {
      // Applies run verification server-side (module tests plus the
      // custom-binary build) under the server's 15-minute budget, so the
      // client outlives it.
      const response = await api.postWithTimeout<{
        data: { project: GeneratedModule; apply: ApplyResult }
      }>(
        `/api/v1/modgen/projects/${name}/apply`,
        { replace: opts.replace ?? false, confirm: opts.confirm ?? false },
        applyTimeout,
      )
      const applied = response.data.data.apply
      projects.value = projects.value.map((p) =>
        p.name === name ? { ...p, applied: applied.applied } : p,
      )
      return response.data.data
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function unapply(name: string): Promise<UnapplyResult> {
    isLoading.value = true
    error.value = null
    try {
      const response = await api.postWithTimeout<{ data: { unapply: UnapplyResult } }>(
        `/api/v1/modgen/projects/${name}/unapply`,
        { confirm: true },
        applyTimeout,
      )
      const result = response.data.data.unapply
      projects.value = projects.value.map((p) =>
        p.name === name ? { ...p, applied: false } : p,
      )
      return result
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
    isLoading,
    error,
    generate,
    fetchProjects,
    download,
    remove,
    apply,
    unapply,
  }
})
