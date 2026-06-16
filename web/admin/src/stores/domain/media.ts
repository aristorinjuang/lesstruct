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
  media: Media[]
}

export const useMediaStore = defineStore('media', () => {
  const media = ref<Media[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function upload(file: File, altText: string, options?: { force?: boolean }): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      const formData = new FormData()
      formData.append('image', file)
      formData.append('alt_text', altText)

      const url = options?.force
        ? '/api/v1/media/upload?force=true'
        : '/api/v1/media/upload'

      const response = await api.post<Media>(url, formData)

      const data = response.data.data as any
      if (data?.duplicate) {
        const duplicateError = new Error('Duplicate media detected') as any
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
    isLoading.value = true
    error.value = null

    try {
      const params: Record<string, string> = {}
      if (options?.search) {
        params.search = options.search
      }
      if (options?.dateFilter) {
        params.date_filter = options.dateFilter
      }
      const response = await api.get<MediaListResponse>('/api/v1/media', { params })
      const mediaList = response.data.data.media || []
      media.value = mediaList
      return media.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function generateImage(prompt: string): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      // at least two minutes for waiting the image generation
      const response = await api.postWithTimeout<Media>('/api/v1/media/generate', { prompt }, 130_000)
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

  async function searchMedia(query: string, dateFilter?: string): Promise<Media[]> {
    return fetchMedia({ search: query, dateFilter })
  }

  async function filterByDate(dateFilter: string): Promise<Media[]> {
    return fetchMedia({ dateFilter })
  }

  async function getById(id: number): Promise<Media> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<Media>(`/api/v1/media/${id}`)
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
    error,
    upload,
    generateImage,
    fetchMedia,
    searchMedia,
    filterByDate,
    getById,
    deleteMedia,
    clearError
  }
})
