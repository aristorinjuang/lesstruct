import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { Content, CreateContentRequest, UpdateContentRequest, SEOMetadata } from '@/types/content'
import type { PostType, PostTypesResponse } from '@/types/posttype'

export interface ContentFiltersOptions {
  search?: string
  postType?: string
  language?: string
  status?: string
}

interface ContentListResponse {
  data: Content[]
  error: null
  meta?: {
    pagination?: {
      total?: number
      limit?: number
      offset?: number
      hasMore: boolean
    }
  }
}

const CONTENT_PAGE_SIZE = 50

export const useContentStore = defineStore('content', () => {
  const content = ref<Content | null>(null)
  const contents = ref<Content[]>([])
  const postTypes = ref<PostType[]>([])
  const total = ref(0)
  const isLoading = ref(false)
  const isLoadingMore = ref(false)
  const hasMore = ref(false)
  const nextOffset = ref(0)
  const error = ref<Error | null>(null)
  const activeSearch = ref('')
  const activePostType = ref('')
  const activeLanguage = ref('')
  const activeStatus = ref('')

  let requestToken = 0

  async function create(data: CreateContentRequest): Promise<Content> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.post<{ data: { content: Content }; error: null; meta: { timestamp: string } }>(
        '/api/v1/content_items',
        data as unknown as Record<string, unknown>,
      )
      content.value = response.data.data.content
      if (!content.value) {
        throw new Error('Failed to create content: No data returned')
      }
      return content.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function update(id: number, data: UpdateContentRequest): Promise<Content> {
    isLoading.value = true
    error.value = null

    const previousContent = content.value ? { ...content.value } : null

    try {
      const response = await api.put<{ data: { content: Content }; error: null; meta: { timestamp: string } }>(
        `/api/v1/content_items/${id}`,
        data as unknown as Record<string, unknown>,
      )
      content.value = response.data.data.content
      if (!content.value) {
        throw new Error('Failed to update content: No data returned')
      }
      return content.value
    } catch (err) {
      error.value = err as Error
      if (previousContent) {
        content.value = previousContent
      }
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function generateSlug(title: string): Promise<{ slug: string }> {
    const response = await api.post<{ data: { slug: string }; error: null; meta: { timestamp: string } }>('/api/v1/content/slug', { title })
    return response.data.data
  }

  async function getBySlug(slug: string): Promise<Content | null> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<{ data: Content; error: null; meta: { timestamp: string } }>(`/api/v1/content_items/slug/${slug}`)
      content.value = response.data.data
      return content.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchContents(options?: ContentFiltersOptions): Promise<Content[]> {
    const token = ++requestToken
    isLoading.value = true
    isLoadingMore.value = false
    error.value = null
    activeSearch.value = options?.search ?? ''
    activePostType.value = options?.postType ?? ''
    activeLanguage.value = options?.language ?? ''
    activeStatus.value = options?.status ?? ''

    try {
      const response = await api.get<ContentListResponse>('/api/v1/content_items', {
        params: buildPageParams(activeSearch.value, activePostType.value, activeLanguage.value, activeStatus.value, CONTENT_PAGE_SIZE, 0),
      })
      if (token !== requestToken) {
        return contents.value
      }
      const items = response.data.data ?? []
      const pagination = response.data.meta?.pagination
      contents.value = items
      total.value = pagination?.total ?? items.length
      hasMore.value = pagination?.hasMore === true
      nextOffset.value = items.length
      return contents.value
    } catch (err) {
      if (token !== requestToken) {
        return contents.value
      }
      error.value = err as Error
      throw err
    } finally {
      if (token === requestToken) {
        isLoading.value = false
      }
    }
  }

  async function loadMore(): Promise<Content[]> {
    if (isLoading.value || isLoadingMore.value || !hasMore.value) {
      return contents.value
    }

    const token = ++requestToken
    isLoadingMore.value = true
    error.value = null

    try {
      const response = await api.get<ContentListResponse>('/api/v1/content_items', {
        params: buildPageParams(activeSearch.value, activePostType.value, activeLanguage.value, activeStatus.value, CONTENT_PAGE_SIZE, nextOffset.value),
      })
      if (token !== requestToken) {
        return contents.value
      }
      const items = response.data.data ?? []
      const pagination = response.data.meta?.pagination
      contents.value = [...contents.value, ...items]
      total.value = pagination?.total ?? total.value
      hasMore.value = pagination?.hasMore === true
      nextOffset.value += items.length
      return contents.value
    } catch (err) {
      if (token !== requestToken) {
        return contents.value
      }
      error.value = err as Error
      throw err
    } finally {
      if (token === requestToken) {
        isLoadingMore.value = false
      }
    }
  }

  function buildPageParams(search: string, postType: string, language: string, status: string, limit: number, offset: number): Record<string, string | number> {
    const params: Record<string, string | number> = { limit, offset }
    if (search) params.search = search
    if (postType) params.post_type = postType
    if (language) params.language = language
    if (status) params.status = status
    return params
  }

  async function getByUser(options?: ContentFiltersOptions): Promise<Content[]> {
    return fetchContents(options)
  }

  async function getAll(limit?: number, offset?: number, options?: ContentFiltersOptions): Promise<Content[]> {
    return fetchContents(options)
  }

  async function getById(id: number): Promise<Content> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<{ data: Content; error: null; meta: { timestamp: string } }>(`/api/v1/content_items/${id}`)
      content.value = response.data.data
      if (!content.value) {
        throw new Error(`Failed to fetch content with id ${id}: No data returned`)
      }
      return content.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function deleteContent(id: number): Promise<void> {
    isLoading.value = true
    error.value = null

    try {
      await api.delete(`/api/v1/content_items/${id}`)
      contents.value = contents.value.filter(c => c.id !== id)
      total.value = Math.max(0, total.value - 1)
      if (content.value?.id === id) {
        content.value = null
      }
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchSEO(id: number): Promise<SEOMetadata> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<{ data: { seo: SEOMetadata }; error: null; meta: { timestamp: string } }>(`/api/v1/content_items/${id}/seo`)
      return response.data.data.seo
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function updateSystemFields(id: number, systemFields: Record<string, unknown>): Promise<Content> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.put<{ data: { content: Content }; error: null; meta: { timestamp: string } }>(
        `/api/admin/content/${id}/system-fields`,
        { systemFields },
      )
      const updated = response.data.data.content
      if (!updated) {
        throw new Error('Failed to update system fields: No data returned')
      }
      if (content.value) {
        content.value = { ...content.value, customFields: { ...content.value.customFields, ...updated.customFields } }
      }
      return content.value!
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchPostTypes(): Promise<PostType[]> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<PostTypesResponse>('/api/v1/post_types')
      postTypes.value = response.data.data
      return postTypes.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function getTranslations(groupId: number): Promise<Content[]> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<{ data: { translations: Content[] }; error: null; meta: { timestamp: string } }>(
        `/api/v1/content_items/translations/${groupId}`,
      )
      return response.data.data.translations
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function enhanceContent(
    content: string,
    format: 'tiptap' | 'html' = 'tiptap',
    existingHtml: string = '',
  ): Promise<string> {
    isLoading.value = true
    error.value = null

    try {
      const body: Record<string, string> = { content, format }
      if (format === 'html' && existingHtml) {
        body.existingHtml = existingHtml
      }
      const response = await api.postWithTimeout<{ data: { content: string } }>('/api/v1/text/enhance', body, 130_000)
      const data = response.data.data
      if (!data || !data.content) {
        throw new Error('Failed to enhance content: No data returned')
      }
      return data.content
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function translateContent(
    content: string,
    sourceLang: string,
    targetLang: string,
    format: 'tiptap' | 'html' = 'tiptap',
  ): Promise<string> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.postWithTimeout<{ data: { content: string } }>(
        '/api/v1/text/translate',
        { content, sourceLang, targetLang, format },
        130_000,
      )
      const data = response.data.data
      if (!data || !data.content) {
        throw new Error('Failed to translate content: No data returned')
      }
      return data.content
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    content,
    contents,
    postTypes,
    total,
    isLoading,
    isLoadingMore,
    hasMore,
    nextOffset,
    error,
    create,
    update,
    generateSlug,
    getBySlug,
    getByUser,
    getAll,
    getById,
    fetchContents,
    loadMore,
    deleteContent,
    fetchSEO,
    fetchPostTypes,
    updateSystemFields,
    getTranslations,
    enhanceContent,
    translateContent,
  }
})
