import { ref } from 'vue'
import api from '@/utils/request'

interface ConfigData {
  languages: string[]
  commentsEnabled?: boolean
  headless?: boolean
  catalog?: {
    postType: string
    currency: string
    displayCurrencies?: Array<{ code: string; rate: number }>
  }
  orders?: {
    currency: string
  }
  payments?: {
    methods: string[]
    gateway: boolean
  }
}

const languages = ref<string[]>(['en'])
const commentsEnabled = ref<boolean>(true)
const headless = ref<boolean>(false)
const catalog = ref<{ postType: string; currency: string; displayCurrencies?: Array<{ code: string; rate: number }> } | null>(null)
const orders = ref<{ currency: string } | null>(null)
const payments = ref<{ methods: string[]; gateway: boolean } | null>(null)
const isLoaded = ref(false)

export function useConfig() {
  async function fetchConfig(): Promise<string[]> {
    if (isLoaded.value) return languages.value

    try {
      const response = await api.get<{ data: ConfigData }>('/api/v1/config')
      languages.value = response.data.data.languages
      commentsEnabled.value = response.data.data.commentsEnabled ?? true
      headless.value = response.data.data.headless ?? false
      catalog.value = response.data.data.catalog ?? null
      orders.value = response.data.data.orders ?? null
      payments.value = response.data.data.payments ?? null
      isLoaded.value = true
    } catch {
      languages.value = ['en']
    }

    return languages.value
  }

  function primaryLanguage(): string {
    return languages.value[0] ?? 'en'
  }

  return {
    languages,
    commentsEnabled,
    headless,
    catalog,
    orders,
    payments,
    isLoaded,
    fetchConfig,
    primaryLanguage,
  }
}
