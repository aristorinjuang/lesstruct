import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'

export interface MediaVariant {
  filePath: string
  url: string
  width: number
  height: number
}

export interface Media {
  id: number
  userId: number
  filename: string
  originalFilename: string
  mimeType: string
  fileSize: number
  width: number
  height: number
  altText: string
  isWebp: boolean
  filePath: string
  url: string
  hash: string
  variants?: Record<string, MediaVariant>
  uploadedBy: string
  createdAt: string
  updatedAt: string
}

interface MediaListResponse {
  data: Media[]
  meta?: {
    pagination?: {
      nextCursor?: string
      hasMore: boolean
      total?: number
    }
  }
}

export const useMediaStore = defineStore('media', () => {
  const media = ref<Media[]>([])
  const isLoading = ref(false)
  const isLoadingMore = ref(false)
  const hasMore = ref(false)
  const total = ref(0)
  const nextCursor = ref('')
  const error = ref<Error | null>(null)

  // Filters active on the last (re)load. loadMore() must re-send them: the cursor only
  // encodes the id boundary, the server still needs the same WHERE clause on every page.
  const activeSearch = ref('')
  const activeDateFilter = ref('')

  // Monotonic token discarding responses that were superseded by a newer request (e.g. a
  // loadMore still in flight when the user changes the search filter). Only the request
  // holding the latest token may mutate state.
  let requestToken = 0

  async function upload(
    file: File,
    altText: string,
    options?: { force?: boolean },
  ): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      const formData = new FormData()
      formData.append('image', file)
      formData.append('alt_text', altText)

      const url = options?.force ? '/api/v1/media/upload?force=true' : '/api/v1/media/upload'

      const response = await api.post<{ data: Media }>(url, formData)

      const data = response.data.data as Media & { duplicate?: boolean; existingMedia?: Media }
      if (data?.duplicate) {
        const duplicateError = new Error('Duplicate media detected') as Error & { duplicate: true; existingMedia: Media }
        duplicateError.duplicate = true
        duplicateError.existingMedia = data.existingMedia as Media
        throw duplicateError
      }

      if (!data) {
        throw new Error('Failed to upload media: No data returned')
      }
      media.value.unshift(data as Media)
      return data as Media
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchMedia(options?: { search?: string; dateFilter?: string }): Promise<Media[]> {
    const token = ++requestToken
    isLoading.value = true
    isLoadingMore.value = false
    error.value = null
    activeSearch.value = options?.search || ''
    activeDateFilter.value = options?.dateFilter || ''

    try {
      const params: Record<string, string> = {}
      if (activeSearch.value) {
        params.search = activeSearch.value
      }
      if (activeDateFilter.value) {
        params.date_filter = activeDateFilter.value
      }
      const response = await api.get<MediaListResponse>('/api/v1/media', { params })
      if (token !== requestToken) {
        return media.value
      }
      const mediaList = response.data.data || []
      const pagination = response.data.meta?.pagination
      media.value = mediaList
      hasMore.value = pagination?.hasMore === true
      total.value = pagination?.total ?? mediaList.length
      nextCursor.value = pagination?.nextCursor || ''
      return media.value
    } catch (err) {
      if (token !== requestToken) {
        return media.value
      }
      error.value = err as Error
      throw err
    } finally {
      if (token === requestToken) {
        isLoading.value = false
      }
    }
  }

  async function loadMore(): Promise<Media[]> {
    if (isLoading.value || isLoadingMore.value || !hasMore.value || !nextCursor.value) {
      return media.value
    }

    const token = ++requestToken
    isLoadingMore.value = true
    error.value = null

    try {
      const params: Record<string, string> = { cursor: nextCursor.value }
      if (activeSearch.value) {
        params.search = activeSearch.value
      }
      if (activeDateFilter.value) {
        params.date_filter = activeDateFilter.value
      }
      const response = await api.get<MediaListResponse>('/api/v1/media', {
        params,
      })
      if (token !== requestToken) {
        return media.value
      }
      const mediaList = response.data.data || []
      const pagination = response.data.meta?.pagination
      media.value = [...media.value, ...mediaList]
      hasMore.value = pagination?.hasMore === true
      total.value = pagination?.total ?? total.value
      nextCursor.value = pagination?.nextCursor || ''
      return media.value
    } catch (err) {
      if (token !== requestToken) {
        return media.value
      }
      error.value = err as Error
      throw err
    } finally {
      if (token === requestToken) {
        isLoadingMore.value = false
      }
    }
  }

  async function generateImage(prompt: string): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      // at least two minutes for waiting the image generation
      const response = await api.postWithTimeout<{ data: Media }>(
        '/api/v1/media/generate',
        { prompt },
        130_000,
      )
      const data = response.data.data
      if (!data) {
        throw new Error('Failed to generate image: No data returned')
      }
      media.value.unshift(data as Media)
      return data as Media
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function getById(id: number): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<{ data: Media }>(`/api/v1/media/${id}`)
      const mediaItem = response.data.data
      if (!mediaItem) {
        throw new Error('Failed to get media: No data returned')
      }
      return mediaItem
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function deleteMedia(id: number) {
    isLoading.value = true
    error.value = null

    try {
      await api.delete(`/api/v1/media/${id}`)
      media.value = media.value.filter((m) => m.id !== id)
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    media,
    isLoading,
    isLoadingMore,
    hasMore,
    total,
    nextCursor,
    error,
    upload,
    generateImage,
    fetchMedia,
    loadMore,
    getById,
    deleteMedia,
    clearError,
  }
})
