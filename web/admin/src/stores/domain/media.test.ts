import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useMediaStore, type Media } from './media'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    postWithTimeout: vi.fn(),
  },
}))

const mockMedia: Media = {
  id: 100,
  userId: 1,
  filename: 'abc123.webp',
  originalFilename: 'ai-generated-20260605-120000.webp',
  mimeType: 'image/webp',
  fileSize: 102400,
  width: 1024,
  height: 1024,
  altText: 'A beautiful sunset',
  isWebp: true,
  filePath: '/uploads/media/abc123.webp',
  url: 'http://localhost:8080/uploads/media/abc123.webp',
  hash: 'sha256hash',
  uploadedBy: 'admin',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('useMediaStore generateImage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('sends a JSON payload when no references are given', async () => {
    const postWithTimeout = vi.mocked(api.postWithTimeout)
    postWithTimeout.mockResolvedValue({ data: { data: mockMedia } })

    const store = useMediaStore()
    const result = await store.generateImage('A beautiful sunset')

    expect(postWithTimeout).toHaveBeenCalledWith(
      '/api/v1/media/generate',
      { prompt: 'A beautiful sunset' },
      130_000,
    )
    expect(result).toEqual(mockMedia)
    expect(store.media[0]).toEqual(mockMedia)
  })

  it('sends a JSON payload when references are an empty array', async () => {
    const postWithTimeout = vi.mocked(api.postWithTimeout)
    postWithTimeout.mockResolvedValue({ data: { data: mockMedia } })

    const store = useMediaStore()
    await store.generateImage('A beautiful sunset', [])

    expect(postWithTimeout).toHaveBeenCalledWith(
      '/api/v1/media/generate',
      { prompt: 'A beautiful sunset' },
      130_000,
    )
  })

  it('sends multipart form data when references are given', async () => {
    const postWithTimeout = vi.mocked(api.postWithTimeout)
    postWithTimeout.mockResolvedValue({ data: { data: mockMedia } })

    const store = useMediaStore()
    const file = new File(['bytes'], 'ref.png', { type: 'image/png' })
    await store.generateImage('A beautiful sunset', [file])

    expect(postWithTimeout).toHaveBeenCalledTimes(1)
    const [url, payload, timeout] = postWithTimeout.mock.calls[0] as unknown as [string, unknown, number]
    expect(url).toBe('/api/v1/media/generate')
    expect(timeout).toBe(130_000)
    expect(payload).toBeInstanceOf(FormData)
    const formData = payload as FormData
    expect(formData.get('prompt')).toBe('A beautiful sunset')
    expect(formData.getAll('references')).toEqual([file])
  })

  it('sends the og flag in the JSON payload', async () => {
    const postWithTimeout = vi.mocked(api.postWithTimeout)
    postWithTimeout.mockResolvedValue({ data: { data: mockMedia } })

    const store = useMediaStore()
    await store.generateImage('A beautiful sunset', [], { og: true })

    expect(postWithTimeout).toHaveBeenCalledWith(
      '/api/v1/media/generate',
      { prompt: 'A beautiful sunset', og: true },
      130_000,
    )
  })

  it('sends the og flag in multipart form data', async () => {
    const postWithTimeout = vi.mocked(api.postWithTimeout)
    postWithTimeout.mockResolvedValue({ data: { data: mockMedia } })

    const store = useMediaStore()
    const file = new File(['bytes'], 'ref.png', { type: 'image/png' })
    await store.generateImage('A beautiful sunset', [file], { og: true })

    expect(postWithTimeout).toHaveBeenCalledTimes(1)
    const [, payload] = postWithTimeout.mock.calls[0] as unknown as [string, unknown]
    expect(payload).toBeInstanceOf(FormData)
    expect((payload as FormData).get('og')).toBe('true')
  })
})
