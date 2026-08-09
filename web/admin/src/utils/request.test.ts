import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import api, { ApiError } from './request'

const MOCK_TOKEN = 'mock-jwt-token'

describe('download', () => {
  let createObjectURLSpy: ReturnType<typeof vi.fn>
  let revokeObjectURLSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    localStorage.setItem('auth_token', MOCK_TOKEN)
    createObjectURLSpy = vi.fn(() => 'blob:mock')
    revokeObjectURLSpy = vi.fn()
    vi.stubGlobal('URL', {
      ...globalThis.URL,
      createObjectURL: createObjectURLSpy,
      revokeObjectURL: revokeObjectURLSpy,
    })
  })

  afterEach(() => {
    localStorage.clear()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('downloads the blob with the server-provided filename', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(new Blob(['archive data']), {
        status: 200,
        headers: {
          'Content-Disposition': 'attachment; filename="lesstruct-export-20260808-120000.tar.gz"',
        },
      })
    )
    vi.stubGlobal('fetch', fetchMock)
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const filename = await api.download('/api/admin/export', 'lesstruct-export.tar.gz')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/export',
      expect.objectContaining({
        method: 'GET',
        headers: { Authorization: `Bearer ${MOCK_TOKEN}` },
      })
    )
    expect(filename).toBe('lesstruct-export-20260808-120000.tar.gz')
    expect(createObjectURLSpy).toHaveBeenCalled()
    expect(clickSpy).toHaveBeenCalled()
    await vi.waitFor(() => expect(revokeObjectURLSpy).toHaveBeenCalled())
  })

  it('falls back to the provided filename when Content-Disposition is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(new Blob(['data']), { status: 200 })))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const filename = await api.download('/api/admin/ssg', 'lesstruct-site.tar.gz')

    expect(filename).toBe('lesstruct-site.tar.gz')
  })

  it('throws ApiError with the structured error code and message on failure', async () => {
    const errorBody = { error: { code: 'export_error', message: 'Failed to export content' } }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(errorBody), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        })
      )
    )

    const promise = api.download('/api/admin/export', 'lesstruct-export.tar.gz')

    await expect(promise).rejects.toThrow(ApiError)
    await expect(promise).rejects.toMatchObject({ statusCode: 500, code: 'export_error', message: 'Failed to export content' })
    expect(createObjectURLSpy).not.toHaveBeenCalled()
  })
})
