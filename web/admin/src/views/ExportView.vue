<script setup lang="ts">
import { computed, ref } from 'vue'
import api, { ApiError } from '@/utils/request'
import Button from '@/components/atoms/Button.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useTheme } from '@/composables/useTheme'
import hugoLogo from '@/assets/hugo-logo.webp'
import htmlLogo from '@/assets/html-logo.webp'

interface ExportOption {
  id: 'hugo' | 'ssg'
  title: string
  description: string
  badge: string
  lightLogo?: string
  darkLogo?: string
  endpoint: string
  filename: string
}

const exportOptions: ExportOption[] = [
  {
    id: 'hugo',
    title: 'Hugo source files',
    description:
      'Download all content as Hugo-compatible source files (YAML frontmatter + HTML body) with bundled media, ready for a Hugo project.',
    badge: 'H',
    lightLogo: hugoLogo,
    darkLogo: hugoLogo,
    endpoint: '/api/admin/export',
    filename: 'lesstruct-export.tar.gz',
  },
  {
    id: 'ssg',
    title: 'Static HTML site',
    description:
      'Generate a fully static HTML site with AMP variants, homepage and pagination, post type, author and tag pages, sitemap, robots.txt, theme CSS and media.',
    badge: 'S',
    lightLogo: htmlLogo,
    darkLogo: htmlLogo,
    endpoint: '/api/admin/ssg',
    filename: 'lesstruct-site.tar.gz',
  },
]

const downloading = ref<'hugo' | 'ssg' | null>(null)

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

const { resolvedTheme } = useTheme()

const isDark = computed(() => resolvedTheme.value === 'dark')

function optionLogo(option: ExportOption): string | undefined {
  return isDark.value ? option.darkLogo : option.lightLogo
}

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

async function handleDownload(option: ExportOption) {
  if (downloading.value !== null) {
    return
  }
  downloading.value = option.id
  try {
    const filename = await api.download(option.endpoint, option.filename)
    displayToast(`Downloaded ${filename}`)
  } catch (err) {
    if (err instanceof ApiError) {
      displayToast(err.message, 'error')
    } else {
      displayToast('Download failed. Please try again.', 'error')
    }
  } finally {
    downloading.value = null
  }
}
</script>

<template>
  <div class="export-view">
    <header class="page-header page-header--stacked">
      <div>
        <h1 class="page-title">Export</h1>
        <p class="page-subtitle">Download your content as Hugo source files or a fully static HTML site.</p>
      </div>
    </header>

    <div class="export-grid">
      <div v-for="option in exportOptions" :key="option.id" class="export-card">
        <div class="export-card__header">
          <img
            v-if="optionLogo(option)"
            class="export-card__logo"
            :src="optionLogo(option)"
            :alt="`${option.title} logo`"
          />
          <span v-else class="export-card__badge" aria-hidden="true">{{ option.badge }}</span>
          <h2 class="export-card__title">{{ option.title }}</h2>
        </div>
        <p class="export-card__description">{{ option.description }}</p>
        <div class="export-card__actions">
          <Button
            type="button"
            variant="primary"
            :is-loading="downloading === option.id"
            :disabled="downloading !== null && downloading !== option.id"
            @click="handleDownload(option)"
          >
            {{ downloading === option.id ? 'Generating...' : 'Download' }}
          </Button>
        </div>
      </div>
    </div>

    <Toast
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @update:visible="toastVisible = $event"
    />
  </div>
</template>

<style scoped>
.export-view {
  max-width: 100%;
}

.export-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
}

.export-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  background: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.75rem;
}

.export-card__header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.export-card__logo {
  width: 44px;
  height: 44px;
  object-fit: contain;
  flex-shrink: 0;
}

.export-card__badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 0.5rem;
  background: var(--brand-primary);
  color: #fff;
  font-size: 1.25rem;
  font-weight: 700;
  text-transform: uppercase;
  flex-shrink: 0;
}

.export-card__title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--brand-dark-1);
}

.export-card__description {
  margin: 0;
  font-size: 0.9rem;
  line-height: 1.5;
  color: var(--brand-dark-2);
}

.export-card__actions {
  margin-top: auto;
  padding-top: 0.5rem;
}
</style>
