import { ref } from 'vue'
import api from '@/utils/request'

interface ConfigData {
  languages: string[]
}

const languages = ref<string[]>(['en'])
const isLoaded = ref(false)

export function useConfig() {
  async function fetchConfig(): Promise<string[]> {
    if (isLoaded.value) return languages.value

    try {
      const response = await api.get<{ data: ConfigData }>('/api/v1/config')
      languages.value = response.data.data.languages
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
    isLoaded,
    fetchConfig,
    primaryLanguage,
  }
}
