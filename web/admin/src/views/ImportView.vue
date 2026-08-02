<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import wordpressLogo from '@/assets/wordpress-logo.webp'
import wordpressLogoDark from '@/assets/wordpress-logo-dark.webp'

interface Platform {
  id: string
  name: string
  description: string
  lightLogo?: string
  darkLogo?: string
  target: string
}

const router = useRouter()
const { resolvedTheme } = useTheme()

const isDark = computed(() => resolvedTheme.value === 'dark')

// Supported import platforms. Add more (e.g. Blogger, Medium) by extending
// this list. Platforms without a logo get a monogram badge instead.
const platforms: Platform[] = [
  {
    id: 'wordpress',
    name: 'WordPress',
    description: 'Import posts and pages from a WXR export file.',
    lightLogo: wordpressLogo,
    darkLogo: wordpressLogoDark,
    target: '/import/wordpress',
  },
  {
    id: 'hugo',
    name: 'Hugo',
    description: 'Import posts from a Hugo site archive (.tar.gz).',
    target: '/import/hugo',
  },
]

function platformLogo(platform: Platform): string | undefined {
  return isDark.value ? platform.darkLogo : platform.lightLogo
}

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
          v-if="platformLogo(platform)"
          class="platform-card__logo"
          :src="platformLogo(platform)"
          :alt="`${platform.name} logo`"
        />
        <span v-else class="platform-card__badge" aria-hidden="true">
          {{ platform.name.charAt(0) }}
        </span>
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

.platform-card__badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 0.5rem;
  background: var(--brand-primary);
  color: #fff;
  font-size: 1.5rem;
  font-weight: 700;
  text-transform: uppercase;
}
</style>
