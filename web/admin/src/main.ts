import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { useConfig } from '@/composables/useConfig'

async function bootstrap() {
  const app = createApp(App)

  app.use(createPinia())
  app.use(router)

  // Load feature flags (languages, commentsEnabled, headless) before the first
  // render so the sidebar and comment surfaces start in the correct state.
  await useConfig().fetchConfig()

  app.mount('#app')
}

bootstrap()
