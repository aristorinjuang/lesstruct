<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import wordpressLogo from '@/assets/wordpress-logo.webp'
import wordpressLogoDark from '@/assets/wordpress-logo-dark.webp'

const router = useRouter()
const { resolvedTheme } = useTheme()

const isDark = computed(() => resolvedTheme.value === 'dark')

// Supported import platforms. Only WordPress for now; the structure is ready
// to add more (e.g. Blogger, Medium) by extending this list.
const platforms = [
  {
    id: 'wordpress',
    name: 'WordPress',
    description: 'Import posts and pages from a WXR export file.',
    lightLogo: wordpressLogo,
    darkLogo: wordpressLogoDark,
    target: '/import/wordpress',
  },
]

function selectPlatform(target: string) {
  router.push(target)
}
</script>

<template>
  <div class="import-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Import</h1>
        <p class="page-subtitle">Choose a platform to import content from.</p>
      </div>
    </header>

    <div class="platform-grid">
      <button
        v-for="platform in platforms"
        :key="platform.id"
        type="button"
        class="platform-card"
        @click="selectPlatform(platform.target)"
      >
        <img
          class="platform-card__logo"
          :src="isDark ? platform.darkLogo : platform.lightLogo"
          :alt="`${platform.name} logo`"
        />
        <h2 class="platform-card__title">{{ platform.name }}</h2>
        <p class="platform-card__description">{{ platform.description }}</p>
      </button>
    </div>
  </div>
</template>

<style scoped>
.import-view {
  max-width: 100%;
}
</style>
