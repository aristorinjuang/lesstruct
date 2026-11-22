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

interface HealthData {
  features?: {
    themeGeneration?: boolean
    pluginGeneration?: boolean
    moduleGeneration?: boolean
    moduleApply?: boolean
  }
}

const languages = ref<string[]>(['en'])
const commentsEnabled = ref<boolean>(true)
const headless = ref<boolean>(false)
const catalog = ref<{ postType: string; currency: string; displayCurrencies?: Array<{ code: string; rate: number }> } | null>(null)
const orders = ref<{ currency: string } | null>(null)
const payments = ref<{ methods: string[]; gateway: boolean } | null>(null)
const themeGeneration = ref<boolean>(true)
const pluginGeneration = ref<boolean>(true)
const moduleGeneration = ref<boolean>(true)
const moduleApply = ref<boolean>(false)
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

    // Generator availability lives on /api/health (the same probe the
    // generator views use). Keep the entries visible when the probe fails so
    // a transient error never hides a working surface; only an explicit
    // `false` from the server removes the sidebar entry.
    try {
      const response = await api.get<HealthData>('/api/health')
      themeGeneration.value = response.data.features?.themeGeneration !== false
      pluginGeneration.value = response.data.features?.pluginGeneration !== false
      moduleGeneration.value = response.data.features?.moduleGeneration !== false
      // Apply stays hidden unless the server explicitly reports it: staging
      // a module into the checkout is the one generator action with
      // source-tree side effects.
      moduleApply.value = response.data.features?.moduleApply === true
    } catch {
      // Keep the defaults (visible); the generator views gate themselves.
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
    themeGeneration,
    pluginGeneration,
    moduleGeneration,
    moduleApply,
    isLoaded,
    fetchConfig,
    primaryLanguage,
  }
}
