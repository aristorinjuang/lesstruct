import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { CatalogProduct, CatalogUpsertPayload } from '@/types/catalog'

interface CatalogResponse {
  data: CatalogProduct
}

export const useCatalogStore = defineStore('catalog', () => {
  const product = ref<CatalogProduct | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)

  async function fetchProduct(contentId: number): Promise<CatalogProduct | null> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<CatalogResponse>(`/api/v1/catalog/products/${contentId}`)
      product.value = response.data.data
      return product.value
    } catch (err) {
      product.value = null
      error.value = (err as Error).message || 'Failed to load catalog data'
      return null
    } finally {
      isLoading.value = false
    }
  }

  async function upsertProduct(contentId: number, payload: CatalogUpsertPayload): Promise<CatalogProduct> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.put<CatalogResponse>(`/api/v1/catalog/products/${contentId}`, payload)
      product.value = response.data.data
      return product.value
    } catch (err) {
      error.value = (err as Error).message || 'Failed to save catalog data'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    product,
    isLoading,
    error,
    fetchProduct,
    upsertProduct,
  }
})
