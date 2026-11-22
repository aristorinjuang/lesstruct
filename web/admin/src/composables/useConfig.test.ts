import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
  },
}))

describe('useConfig', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
  })

  async function load() {
    const { default: api } = await import('@/utils/request')
    const { useConfig } = await import('./useConfig')
    return { api, config: useConfig() }
  }

  it('should enable generator flags when health reports them enabled', async () => {
    const { api, config } = await load()
    vi.mocked(api.get)
      .mockResolvedValueOnce({ data: { data: { languages: ['en'] } } })
      .mockResolvedValueOnce({
        data: { features: { themeGeneration: true, pluginGeneration: true, moduleGeneration: true } },
      })

    await config.fetchConfig()

    expect(config.themeGeneration.value).toBe(true)
    expect(config.pluginGeneration.value).toBe(true)
    expect(config.moduleGeneration.value).toBe(true)
    expect(config.moduleApply.value).toBe(false)
  })

  it('should disable generator flags when health reports them disabled', async () => {
    const { api, config } = await load()
    vi.mocked(api.get)
      .mockResolvedValueOnce({ data: { data: { languages: ['en'] } } })
      .mockResolvedValueOnce({
        data: {
          features: {
            themeGeneration: false,
            pluginGeneration: false,
            moduleGeneration: false,
            moduleApply: true,
          },
        },
      })

    await config.fetchConfig()

    expect(config.themeGeneration.value).toBe(false)
    expect(config.pluginGeneration.value).toBe(false)
    expect(config.moduleGeneration.value).toBe(false)
    expect(config.moduleApply.value).toBe(true)
  })

  it('should keep generator flags enabled when the health probe fails', async () => {
    const { api, config } = await load()
    vi.mocked(api.get)
      .mockResolvedValueOnce({ data: { data: { languages: ['en'] } } })
      .mockRejectedValueOnce(new Error('offline'))

    await config.fetchConfig()

    expect(config.themeGeneration.value).toBe(true)
    expect(config.pluginGeneration.value).toBe(true)
    expect(config.moduleGeneration.value).toBe(true)
    expect(config.moduleApply.value).toBe(false)
  })

  it('should fetch config and health only once', async () => {
    const { api, config } = await load()
    vi.mocked(api.get)
      .mockResolvedValueOnce({ data: { data: { languages: ['en'] } } })
      .mockResolvedValueOnce({ data: { features: {} } })

    await config.fetchConfig()
    await config.fetchConfig()

    expect(api.get).toHaveBeenCalledTimes(2)
  })
})
