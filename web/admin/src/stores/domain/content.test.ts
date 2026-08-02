import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useContentStore } from './content'
import type { Content, CreateContentRequest, UpdateContentRequest } from '@/types/content'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    postWithTimeout: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('Content Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('update', () => {
    it('should update content successfully', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Updated Title',
        slug: 'updated-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Original Title',
        slug: 'original-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Updated Title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
      }

      const result = await store.update(1, updateData)

      expect(api.put).toHaveBeenCalledWith('/api/v1/content_items/1', updateData)
      expect(result).toEqual(mockContent)
      expect(store.content).toEqual(mockContent)
      expect(store.isLoading).toBe(false)
      expect(store.error).toBe(null)
    })

    it('should rollback to previous content on error', async () => {
      const previousContent: Content = {
        id: 1,
        userId: 1,
        title: 'Original Title',
        slug: 'original-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      vi.mocked(api.put).mockRejectedValue(new Error('Network error'))

      const store = useContentStore()
      store.content = { ...previousContent }

      const updateData: UpdateContentRequest = {
        title: 'Updated Title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
      }

      await expect(store.update(1, updateData)).rejects.toThrow('Network error')

      expect(store.content).toEqual(previousContent)
      expect(store.error).toBeInstanceOf(Error)
      expect(store.isLoading).toBe(false)
    })

    it('should handle unpublish status change', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Title',
        slug: 'title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Title',
        slug: 'title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
      }

      const result = await store.update(1, updateData)

      expect(result.status).toBe('draft')
      expect(store.content?.status).toBe('draft')
    })

    it('should handle publish status change', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Title',
        slug: 'title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Title',
        slug: 'title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
      }

      const result = await store.update(1, updateData)

      expect(result.status).toBe('published')
      expect(store.content?.status).toBe('published')
    })
  })

  describe('generateSlug', () => {
    it('should generate slug from title successfully', async () => {
      vi.mocked(api.post).mockResolvedValue({
        data: {
          data: { slug: 'my-article-title' },
          error: null,
          meta: { timestamp: '2026-04-08T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.post>>)

      const store = useContentStore()

      const result = await store.generateSlug('My Article Title')

      expect(api.post).toHaveBeenCalledWith('/api/v1/content/slug', { title: 'My Article Title' })
      expect(result).toEqual({ slug: 'my-article-title' })
    })

    it('should propagate errors from slug generation', async () => {
      vi.mocked(api.post).mockRejectedValue(new Error('Slug generation failed'))

      const store = useContentStore()

      await expect(store.generateSlug('test')).rejects.toThrow('Slug generation failed')
    })
  })

  describe('getBySlug', () => {
    it('should fetch content by slug successfully', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'My Article',
        slug: 'my-article',
        content: '{"type":"doc"}',
        tags: [],
        status: 'published',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: mockContent,
          error: null,
          meta: { timestamp: '2026-04-08T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      const result = await store.getBySlug('my-article')

      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items/slug/my-article')
      expect(result).toEqual(mockContent)
      expect(store.content).toEqual(mockContent)
      expect(store.isLoading).toBe(false)
    })

    it('should handle getBySlug errors', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Content not found'))

      const store = useContentStore()

      await expect(store.getBySlug('nonexistent')).rejects.toThrow('Content not found')
      expect(store.error).toBeInstanceOf(Error)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('create', () => {
    it('should create content successfully', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'New Title',
        slug: 'new-title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      vi.mocked(api.post).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.post>>)

      const store = useContentStore()

      const createData: CreateContentRequest = {
        title: 'New Title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        userId: 1,
      }

      const result = await store.create(createData)

      expect(api.post).toHaveBeenCalledWith('/api/v1/content_items', createData)
      expect(result).toEqual(mockContent)
      expect(store.content).toEqual(mockContent)
    })
  })

  describe('getByUser', () => {
    it('should fetch user contents successfully', async () => {
      const mockContents: Content[] = [
        {
          id: 1,
          userId: 1,
          title: 'Content 1',
          slug: 'content-1',
          content: '{"type":"doc"}',
          tags: [],
          status: 'draft',
          postType: 'post',
          language: 'en',
          createdAt: '2026-04-08T00:00:00Z',
          updatedAt: '2026-04-08T00:00:00Z',
        },
        {
          id: 2,
          userId: 1,
          title: 'Content 2',
          slug: 'content-2',
          content: '{"type":"doc"}',
          tags: [],
          status: 'published',
          postType: 'post',
          language: 'en',
          createdAt: '2026-04-08T00:00:00Z',
          updatedAt: '2026-04-08T00:00:00Z',
        },
      ]

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: mockContents,
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 2, limit: 50, offset: 0, hasMore: false },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      const result = await store.getByUser()

      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items', { params: { limit: 50, offset: 0 } })
      expect(result).toEqual(mockContents)
      expect(store.contents).toEqual(mockContents)
      expect(store.total).toBe(2)
      expect(store.hasMore).toBe(false)
    })
  })

  describe('fetchContents', () => {
    function buildContent(id: number): Content {
      return {
        id,
        userId: 1,
        title: `Content ${id}`,
        slug: `content-${id}`,
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }
    }

    it('should reset and fetch first page with pagination meta', async () => {
      const pageOne: Content[] = [buildContent(1), buildContent(2)]

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: pageOne,
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 3, limit: 50, offset: 0, hasMore: true },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      const result = await store.fetchContents({ postType: 'post' })

      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items', { params: { limit: 50, offset: 0, post_type: 'post' } })
      expect(result).toEqual(pageOne)
      expect(store.contents).toEqual(pageOne)
      expect(store.total).toBe(3)
      expect(store.hasMore).toBe(true)
      expect(store.nextOffset).toBe(2)
      expect(store.isLoading).toBe(false)
      expect(store.isLoadingMore).toBe(false)
    })

    it('should include search and language params', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: [],
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 0, limit: 50, offset: 0, hasMore: false },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      await store.fetchContents({ search: 'motor', language: 'id' })

      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items', {
        params: { limit: 50, offset: 0, search: 'motor', language: 'id' },
      })
    })

    it('should include status param and keep it for loadMore', async () => {
      vi.mocked(api.get).mockResolvedValueOnce({
        data: {
          data: [buildContent(1)],
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 3, limit: 50, offset: 0, hasMore: true },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)
      vi.mocked(api.get).mockResolvedValueOnce({
        data: {
          data: [buildContent(2)],
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 3, limit: 50, offset: 1, hasMore: false },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      await store.fetchContents({ status: 'draft' })
      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items', {
        params: { limit: 50, offset: 0, status: 'draft' },
      })

      await store.loadMore()
      expect(api.get).toHaveBeenLastCalledWith('/api/v1/content_items', {
        params: { limit: 50, offset: 1, status: 'draft' },
      })
      expect(store.total).toBe(3)
    })

    it('should propagate fetch errors', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Failed to fetch contents'))

      const store = useContentStore()

      await expect(store.fetchContents()).rejects.toThrow('Failed to fetch contents')
      expect(store.error).toBeInstanceOf(Error)
      expect(store.isLoading).toBe(false)
    })
  })

  describe('loadMore', () => {
    function buildContent(id: number): Content {
      return {
        id,
        userId: 1,
        title: `Content ${id}`,
        slug: `content-${id}`,
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }
    }

    it('should append next page items and advance offset', async () => {
      const pageOne: Content[] = [buildContent(1), buildContent(2)]
      const pageTwo: Content[] = [buildContent(3)]

      vi.mocked(api.get).mockResolvedValueOnce({
        data: {
          data: pageOne,
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 3, limit: 50, offset: 0, hasMore: true },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)
      vi.mocked(api.get).mockResolvedValueOnce({
        data: {
          data: pageTwo,
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 3, limit: 50, offset: 2, hasMore: false },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      await store.fetchContents()
      await store.loadMore()

      expect(api.get).toHaveBeenLastCalledWith('/api/v1/content_items', { params: { limit: 50, offset: 2 } })
      expect(store.contents).toEqual([...pageOne, ...pageTwo])
      expect(store.total).toBe(3)
      expect(store.hasMore).toBe(false)
      expect(store.nextOffset).toBe(3)
      expect(store.isLoadingMore).toBe(false)
    })

    it('should no-op when there are no more pages', async () => {
      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: [],
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 0, limit: 50, offset: 0, hasMore: false },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      await store.fetchContents()
      await store.loadMore()

      expect(api.get).toHaveBeenCalledTimes(1)
      expect(store.contents).toEqual([])
    })

    it('should propagate loadMore errors', async () => {
      const pageOne: Content[] = [buildContent(1)]

      vi.mocked(api.get).mockResolvedValueOnce({
        data: {
          data: pageOne,
          error: null,
          meta: {
            timestamp: '2026-04-08T00:00:00Z',
            pagination: { total: 2, limit: 50, offset: 0, hasMore: true },
          },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)
      vi.mocked(api.get).mockRejectedValueOnce(new Error('Failed to load more'))

      const store = useContentStore()

      await store.fetchContents()
      await expect(store.loadMore()).rejects.toThrow('Failed to load more')
      expect(store.error).toBeInstanceOf(Error)
      expect(store.contents).toEqual(pageOne)
      expect(store.isLoadingMore).toBe(false)
    })
  })

  describe('SEO Field Handling', () => {
    it('should include SEO fields in update request when provided', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Updated Title',
        slug: 'updated-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: 'Custom meta description',
        ogTitle: 'Custom OG Title',
        ogDescription: 'Custom OG description',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Original Title',
        slug: 'original-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Updated Title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: 'Custom meta description',
        ogTitle: 'Custom OG Title',
        ogDescription: 'Custom OG description',
      }

      const result = await store.update(1, updateData)

      expect(api.put).toHaveBeenCalledWith('/api/v1/content_items/1', updateData)
      expect(result.metaDescription).toBe('Custom meta description')
      expect(result.ogTitle).toBe('Custom OG Title')
      expect(result.ogDescription).toBe('Custom OG description')
    })

    it('should include SEO fields in create request when provided', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'New Title',
        slug: 'new-title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        metaDescription: 'Initial meta description',
        ogTitle: 'Initial OG Title',
        ogDescription: 'Initial OG description',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      vi.mocked(api.post).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.post>>)

      const store = useContentStore()

      const createData: CreateContentRequest = {
        title: 'New Title',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        userId: 1,
        metaDescription: 'Initial meta description',
        ogTitle: 'Initial OG Title',
        ogDescription: 'Initial OG description',
      }

      const result = await store.create(createData)

      expect(api.post).toHaveBeenCalledWith('/api/v1/content_items', createData)
      expect(result.metaDescription).toBe('Initial meta description')
      expect(result.ogTitle).toBe('Initial OG Title')
      expect(result.ogDescription).toBe('Initial OG description')
    })

    it('should handle update with empty SEO fields', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Updated Title',
        slug: 'updated-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: '',
        ogTitle: '',
        ogDescription: '',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Original Title',
        slug: 'original-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Updated Title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: '',
        ogTitle: '',
        ogDescription: '',
      }

      const result = await store.update(1, updateData)

      expect(result.metaDescription).toBe('')
      expect(result.ogTitle).toBe('')
      expect(result.ogDescription).toBe('')
    })

    it('should handle partial SEO field updates', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Updated Title',
        slug: 'updated-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: 'Only meta description provided',
        ogTitle: '',
        ogDescription: '',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T12:00:00Z',
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Original Title',
        slug: 'original-title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
      }

      const updateData: UpdateContentRequest = {
        title: 'Updated Title',
        content: '{"type":"doc"}',
        tags: ['tag1'],
        status: 'published',
        postType: 'post',
        language: 'en',
        metaDescription: 'Only meta description provided',
      }

      const result = await store.update(1, updateData)

      expect(result.metaDescription).toBe('Only meta description provided')
      expect(result.ogTitle).toBe('')
      expect(result.ogDescription).toBe('')
    })
  })

  describe('fetchSEO', () => {
    it('should fetch SEO metadata successfully', async () => {
      const mockSEOMetadata = {
        metaDescription: 'Auto-generated meta description',
        ogTitle: 'Article Title',
        ogDescription: 'Auto-generated OG description',
        ogImage: '/uploads/media/featured.webp',
        ogUrl: '/posts/article-slug',
        ogType: 'article',
        ogSiteName: 'Lesstruct',
        twitterCard: 'summary_large_image',
        twitterTitle: 'Article Title',
        twitterDescription: 'Auto-generated OG description',
        twitterImage: '/uploads/media/featured.webp',
        jsonLd: {
          '@context': 'https://schema.org',
          '@type': 'Article',
          headline: 'Article Title',
          description: 'Auto-generated meta description',
        },
      }

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: { seo: mockSEOMetadata },
          error: null,
          meta: { timestamp: '2026-04-08T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      const result = await store.fetchSEO(1)

      expect(api.get).toHaveBeenCalledWith('/api/v1/content_items/1/seo')
      expect(result).toEqual(mockSEOMetadata)
    })

    it('should handle fetchSEO errors', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Failed to fetch SEO'))

      const store = useContentStore()

      await expect(store.fetchSEO(1)).rejects.toThrow('Failed to fetch SEO')
      expect(store.error).toBeInstanceOf(Error)
      expect(store.isLoading).toBe(false)
    })

    it('should return auto-generated SEO metadata for published content', async () => {
      const mockSEOMetadata = {
        metaDescription: 'This is the first 160 characters of the article content extracted from TipTap JSON...',
        ogTitle: 'Article Title from Content',
        ogDescription: 'This is the first 160 characters of the article content...',
        ogImage: '/uploads/media/abc123.webp',
        ogUrl: '/posts/article-slug',
        ogType: 'article',
        ogSiteName: 'Lesstruct',
        twitterCard: 'summary_large_image',
        twitterTitle: 'Article Title from Content',
        twitterDescription: 'This is the first 160 characters...',
        twitterImage: '/uploads/media/abc123.webp',
        jsonLd: {
          '@context': 'https://schema.org',
          '@type': 'Article',
          headline: 'Article Title from Content',
          description: 'This is the first 160 characters...',
          datePublished: '2026-04-10T00:00:00Z',
          dateModified: '2026-04-10T12:00:00Z',
          author: {
            '@type': 'Person',
            name: 'Author',
          },
        },
      }

      vi.mocked(api.get).mockResolvedValue({
        data: {
          data: { seo: mockSEOMetadata },
          error: null,
          meta: { timestamp: '2026-04-10T00:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.get>>)

      const store = useContentStore()

      const result = await store.fetchSEO(1)

      expect(result.jsonLd.headline).toBe('Article Title from Content')
      expect(result.jsonLd['@type']).toBe('Article')
      expect(result.ogType).toBe('article')
      expect(result.twitterCard).toBe('summary_large_image')
    })
  })

  describe('deleteContent', () => {
    it('should delete content and remove from contents array', async () => {
      const store = useContentStore()
      store.total = 2
      store.contents = [
        { id: 1, userId: 1, title: 'Post 1', slug: 'post-1', content: '{}', tags: [], status: 'draft', postType: 'post', language: 'en', createdAt: '2026-04-08T00:00:00Z', updatedAt: '2026-04-08T00:00:00Z' },
        { id: 2, userId: 1, title: 'Post 2', slug: 'post-2', content: '{}', tags: [], status: 'draft', postType: 'post', language: 'en', createdAt: '2026-04-08T00:00:00Z', updatedAt: '2026-04-08T00:00:00Z' },
      ]

      vi.mocked(api.delete).mockResolvedValue({} as unknown as Awaited<ReturnType<typeof api.delete>>)

      await store.deleteContent(1)

      expect(store.contents.length).toBe(1)
      expect(store.contents[0]!.id).toBe(2)
      expect(store.total).toBe(1)
      expect(api.delete).toHaveBeenCalledWith('/api/v1/content_items/1')
    })

    it('should also clear current content if it matches deleted id', async () => {
      const store = useContentStore()
      store.content = { id: 1, userId: 1, title: 'Post 1', slug: 'post-1', content: '{}', tags: [], status: 'draft', postType: 'post', language: 'en', createdAt: '2026-04-08T00:00:00Z', updatedAt: '2026-04-08T00:00:00Z' }

      vi.mocked(api.delete).mockResolvedValue({} as unknown as Awaited<ReturnType<typeof api.delete>>)

      await store.deleteContent(1)

      expect(store.content).toBeNull()
    })

    it('should handle delete error', async () => {
      const store = useContentStore()
      store.contents = [
        { id: 1, userId: 1, title: 'Post 1', slug: 'post-1', content: '{}', tags: [], status: 'draft', postType: 'post', language: 'en', createdAt: '2026-04-08T00:00:00Z', updatedAt: '2026-04-08T00:00:00Z' },
      ]

      vi.mocked(api.delete).mockRejectedValue(new Error('Not found'))

      await expect(store.deleteContent(1)).rejects.toThrow('Not found')
      expect(store.error).not.toBeNull()
      expect(store.contents.length).toBe(1)
    })
  })

  describe('updateSystemFields', () => {
    it('should call PUT /api/admin/content/:id/system-fields with systemFields', async () => {
      const mockContent: Content = {
        id: 1,
        userId: 1,
        title: 'Test Post',
        slug: 'test-post',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
        customFields: { price: 10, internal_sku: 'SKU-001' },
      }

      vi.mocked(api.put).mockResolvedValue({
        data: {
          data: { content: mockContent },
          error: null,
          meta: { timestamp: '2026-04-08T12:00:00Z' },
        },
      } as unknown as Awaited<ReturnType<typeof api.put>>)

      const store = useContentStore()
      store.content = {
        id: 1,
        userId: 1,
        title: 'Test Post',
        slug: 'test-post',
        content: '{"type":"doc"}',
        tags: [],
        status: 'draft',
        postType: 'post',
        language: 'en',
        createdAt: '2026-04-08T00:00:00Z',
        updatedAt: '2026-04-08T00:00:00Z',
        customFields: { price: 10 },
      }

      const result = await store.updateSystemFields(1, { internal_sku: 'SKU-001' })

      expect(api.put).toHaveBeenCalledWith(
        '/api/admin/content/1/system-fields',
        { systemFields: { internal_sku: 'SKU-001' } },
      )
      expect(result?.customFields).toEqual({ price: 10, internal_sku: 'SKU-001' })
    })

    it('should propagate errors from system fields endpoint', async () => {
      vi.mocked(api.put).mockRejectedValue(new Error('Forbidden'))

      const store = useContentStore()

      await expect(store.updateSystemFields(1, { internal_sku: 'SKU-001' })).rejects.toThrow('Forbidden')
      expect(store.error).not.toBeNull()
    })
  })
})
