import { ref } from 'vue'
import api from '@/utils/request'

interface ConfigData {
  languages: string[]
  commentsEnabled?: boolean
  headless?: boolean
}

const languages = ref<string[]>(['en'])
const commentsEnabled = ref<boolean>(true)
const headless = ref<boolean>(false)
const isLoaded = ref(false)

export function useConfig() {
  async function fetchConfig(): Promise<string[]> {
    if (isLoaded.value) return languages.value

    try {
      const response = await api.get<{ data: ConfigData }>('/api/v1/config')
      languages.value = response.data.data.languages
      commentsEnabled.value = response.data.data.commentsEnabled ?? true
      headless.value = response.data.data.headless ?? false
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
    isLoaded,
    fetchConfig,
    primaryLanguage,
  }
}
