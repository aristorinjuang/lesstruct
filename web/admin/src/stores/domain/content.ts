import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { Content, CreateContentRequest, UpdateContentRequest, SEOMetadata } from '@/types/content'
import type { PostType, PostTypesResponse } from '@/types/posttype'

export const useContentStore = defineStore('content', () => {
  const content = ref<Content | null>(null)
  const contents = ref<Content[]>([])
  const postTypes = ref<PostType[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function create(data: CreateContentRequest): Promise<Content> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.post<{ data: { content: Content }; error: null; meta: { timestamp: string } }>('/api/v1/content_items', data)
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
      const response = await api.put<{ data: { content: Content }; error: null; meta: { timestamp: string } }>(`/api/v1/content_items/${id}`, data)
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

  async function getByUser(options?: { search?: string; postType?: string; language?: string }): Promise<Content[]> {
    isLoading.value = true
    error.value = null

    try {
      const params = new URLSearchParams()
      if (options?.search) params.set('search', options.search)
      if (options?.postType) params.set('post_type', options.postType)
      if (options?.language) params.set('language', options.language)
      const query = params.toString()
      const url = `/api/v1/content_items${query ? '?' + query : ''}`
      const response = await api.get<{ data: Content[]; error: null; meta: { timestamp: string } }>(url)
      contents.value = response.data.data
      return contents.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function getAll(limit?: number, offset?: number, options?: { search?: string; postType?: string; language?: string }): Promise<Content[]> {
    isLoading.value = true
    error.value = null

    try {
      const params = new URLSearchParams()
      if (limit !== undefined) params.set('limit', String(limit))
      if (offset !== undefined) params.set('offset', String(offset))
      if (options?.search) params.set('search', options.search)
      if (options?.postType) params.set('post_type', options.postType)
      if (options?.language) params.set('language', options.language)
      const query = params.toString()
      const url = `/api/v1/content_items${query ? '?' + query : ''}`
      const response = await api.get<{ data: Content[]; error: null; meta: { timestamp: string } }>(url)
      contents.value = response.data.data
      return contents.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
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

  async function updateSystemFields(id: number, systemFields: Record<string, any>): Promise<Content> {
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

  async function enhanceContent(content: string): Promise<string> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.postWithTimeout<{ content: string }>('/api/v1/text/enhance', { content }, 130_000)
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

  async function translateContent(content: string, sourceLang: string, targetLang: string): Promise<string> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.postWithTimeout<{ content: string }>(
        '/api/v1/text/translate',
        { content, sourceLang, targetLang },
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
    isLoading,
    error,
    create,
    update,
    generateSlug,
    getBySlug,
    getByUser,
    getAll,
    getById,
    deleteContent,
    fetchSEO,
    fetchPostTypes,
    updateSystemFields,
    getTranslations,
    enhanceContent,
    translateContent,
  }
})
