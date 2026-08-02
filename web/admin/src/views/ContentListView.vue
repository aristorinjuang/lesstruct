<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import DeleteConfirmDialog from '@/components/molecules/DeleteConfirmDialog.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useContentStore } from '@/stores/domain/content'
import { useAuth } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import type { Content } from '@/types/content'
import type { PostType } from '@/types/posttype'
import { formatRelativeTime } from '@/utils/date'

const router = useRouter()
const route = useRoute()
const contentStore = useContentStore()
const { userId, role } = useAuth()
const { languages, fetchConfig, primaryLanguage } = useConfig()

const isAdmin = computed(() => role.value === 'Admin')

const postTypes = ref<PostType[]>([])
const selectedPostType = ref<string>((route.query.type as string) || 'all')
const selectedStatus = ref<string>((route.query.status as string) || '')
const searchQuery = ref('')

const deletingContent = ref<Content | null>(null)
const isDeleting = ref(false)
const deleteError = ref('')

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

// Watch route query to sync with selectedPostType and selectedStatus
watch(() => route.fullPath, () => {
  const type = route.query.type as string | undefined
  if (type) {
    selectedPostType.value = type
  } else {
    selectedPostType.value = 'all'
  }
  selectedStatus.value = (route.query.status as string) || ''
})

onMounted(async () => {
  try {
    await fetchConfig()
  } catch (err) {
    console.error('Failed to load config:', err)
  }
  await loadContents()
  await loadPostTypes()
})

let searchTimer: ReturnType<typeof setTimeout> | null = null

watch(searchQuery, (val) => {
  if (searchTimer) clearTimeout(searchTimer)
  if (val.length < 2) {
    loadContents()
    return
  }
  searchTimer = setTimeout(() => {
    loadContents()
  }, 300)
})

watch(selectedPostType, () => {
  loadContents()
})

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

async function loadContents() {
  if (!userId.value) {
    router.push('/login')
    return
  }

  try {
    const searchOptions = searchQuery.value.length >= 2 ? { search: searchQuery.value } : undefined
    const postTypeParam = selectedPostType.value !== 'all' ? selectedPostType.value : undefined
    const statusParam = selectedStatus.value !== '' ? selectedStatus.value : undefined
    const lang = isAdmin.value && languages.value.length > 1 ? primaryLanguage() : undefined
    await contentStore.fetchContents({ ...searchOptions, postType: postTypeParam, status: statusParam, language: lang })
  } catch (err) {
    console.error('Failed to load contents:', err)
  }
}

async function loadPostTypes() {
  try {
    postTypes.value = await contentStore.fetchPostTypes()
  } catch (err) {
    console.error('Failed to load post types:', err)
  }
}

async function loadMore() {
  try {
    await contentStore.loadMore()
  } catch (err) {
    console.error('Failed to load more contents:', err)
  }
}

const { sentinel } = useInfiniteScroll(loadMore, {
  disabled: computed(() => contentStore.isLoading || contentStore.isLoadingMore || !contentStore.hasMore),
})

function buildContentPath(base: string): string {
  const type = selectedPostType.value
  return type && type !== 'all' ? `${base}?type=${type}` : base
}

function createNew() {
  router.push(buildContentPath('/content/create'))
}

function editContent(content: Content) {
  router.push(buildContentPath(`/content/${content.id}/edit`))
}

function selectPostType(postTypeSlug: string) {
  selectedPostType.value = postTypeSlug
  // Update URL query parameter
  if (postTypeSlug === 'all') {
    router.push('/content')
  } else {
    router.push(`/content?type=${postTypeSlug}`)
  }
}

function requestDelete(item: Content, event: Event) {
  event.stopPropagation()
  deletingContent.value = item
  deleteError.value = ''
}

function cancelDelete() {
  deletingContent.value = null
  deleteError.value = ''
}

async function confirmDelete() {
  if (!deletingContent.value) return
  isDeleting.value = true
  deleteError.value = ''

  try {
    await contentStore.deleteContent(deletingContent.value.id)
    deletingContent.value = null
    displayToast('Content deleted successfully')
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : 'Failed to delete content'
  } finally {
    isDeleting.value = false
  }
}

function getStatusBadgeClass(status: string) {
  return `content-list__status--${status}`
}

const postTypeTabs = computed(() => {
  const postTypeOrder: Record<string, number> = {
    'post': 1,
    'page': 2,
  }

  // Exclude types with dedicated panels
  const excludedTypes = ['media', 'comment']

  const sortedPostTypes = [...postTypes.value]
    .filter(pt => !excludedTypes.includes(pt.slug))
    .sort((a, b) => {
      const orderA = postTypeOrder[a.slug] ?? 3
      const orderB = postTypeOrder[b.slug] ?? 3
      if (orderA !== orderB) {
        return orderA - orderB
      }
      return a.name.localeCompare(b.name)
    })

  return [
    { slug: 'all', name: 'All' },
    ...sortedPostTypes
  ]
})

</script>

<template>
  <div class="content-list">
    <div class="page-header">
      <h1 class="page-title">
        Content
        <span v-if="contentStore.total > 0" class="content-list__total-badge">
          {{ contentStore.total.toLocaleString() }}
        </span>
      </h1>
      <button @click="createNew" class="content-list__create-btn">
        Create New Post
      </button>
    </div>

    <div class="content-list__tabs">
      <button
        v-for="postType in postTypeTabs"
        :key="postType.slug"
        @click="selectPostType(postType.slug)"
        :class="['content-list__tab', { 'content-list__tab--active': selectedPostType === postType.slug }]"
      >
        {{ postType.name }}
      </button>
    </div>

    <div class="search-wrapper">
      <svg class="search-wrapper__icon" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search posts..."
        class="search-wrapper__input"
      />
      <button
        v-if="searchQuery"
        @click="searchQuery = ''"
        class="search-wrapper__clear"
        title="Clear search"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
      </button>
    </div>

    <div v-if="contentStore.isLoading" class="state-loading">
      Loading...
    </div>

    <div v-else-if="contentStore.contents.length === 0" class="state-loading">
      <p>No content yet. Create your first post!</p>
    </div>

    <div v-else class="content-list__items">
      <div
        v-for="item in contentStore.contents"
        :key="item.id"
        class="content-list__item"
        @click="editContent(item)"
      >
        <div class="content-list__item-content">
          <h3>{{ item.title }}</h3>
          <p class="content-list__slug">{{ item.slug }}</p>
          <span class="content-list__status" :class="getStatusBadgeClass(item.status)">
            {{ item.status === 'published' ? 'Published' : 'Draft' }}
          </span>
          <span v-if="item.format === 'html'" class="content-list__format-badge">
            HTML
          </span>
          <span v-if="isAdmin && item.author" class="content-list__meta">Created by {{ item.author }}</span>
          <span v-if="item.updatedByUsername" class="content-list__meta">Updated by {{ item.updatedByUsername }}</span>
          <span class="content-list__date">{{ formatRelativeTime(item.updatedAt) }}</span>
        </div>
        <button
          class="content-list__delete-btn"
          @click="requestDelete(item, $event)"
          title="Delete"
          aria-label="Delete content"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
        </button>
      </div>
    </div>

    <div ref="sentinel" class="content-list__load-more" aria-hidden="true">
      <div v-if="contentStore.isLoadingMore" class="content-list__load-more-spinner"></div>
      <span v-if="contentStore.isLoadingMore" class="content-list__load-more-text"
        >Loading more...</span
      >
    </div>

    <DeleteConfirmDialog
      :title="'Delete Content'"
      :item-name="deletingContent?.title ?? ''"
      :is-open="deletingContent !== null"
      :is-loading="isDeleting"
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />

    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <div v-if="deleteError" class="alert alert-error">
      {{ deleteError }}
    </div>
  </div>
</template>

<style scoped>
.search-wrapper {
  margin-bottom: 1.5rem;
}

.content-list__tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 2rem;
  border-bottom: 1px solid var(--brand-light-2);
  overflow-x: auto;
}

.content-list__tab {
  padding: 0.5rem 1rem;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  white-space: nowrap;
  color: var(--brand-dark-2);
  transition: color 0.2s, border-color 0.2s;
}

.content-list__tab:hover {
  color: var(--brand-primary-hover);
}

.content-list__tab--active {
  color: var(--brand-primary-hover);
  border-bottom-color: var(--brand-primary-hover);
}

.content-list__create-btn {
  padding: 0.5rem 1rem;
  background-color: var(--color-info);
  color: var(--color-white);
  border: none;
  border-radius: 0.375rem;
  cursor: pointer;
}

.content-list__items {
  display: grid;
  gap: 1rem;
}

.content-list__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
  cursor: pointer;
  transition: background-color 0.2s, box-shadow 0.2s;
}

.content-list__item:hover {
  background-color: var(--brand-light-1);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.content-list__item-content {
  flex: 1;
  min-width: 0;
}

.content-list__delete-btn {
  flex-shrink: 0;
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  color: var(--color-error);
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  transition: color 0.2s, background-color 0.2s;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.content-list__delete-btn:hover {
  color: var(--color-error);
  background-color: var(--color-error-bg);
}

.content-list__item h3 {
  margin: 0 0 0.5rem 0;
  font-size: 1.125rem;
}

.content-list__slug {
  margin: 0;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
}

.content-list__status {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.75rem;
  font-weight: 500;
  margin-top: 0.5rem;
}

.content-list__meta {
  display: inline-block;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
  margin-top: 0.25rem;
  margin-left: 0.5rem;
}

.content-list__date {
  display: block;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
  margin-top: 0.25rem;
}

.content-list__status--draft {
  background-color: var(--color-bg-muted);
  color: var(--color-text-secondary);
}

.content-list__status--published {
  background-color: var(--color-success-bg);
  color: var(--color-success-dark);
}

.content-list__format-badge {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.75rem;
  font-weight: 500;
  margin-top: 0.5rem;
  margin-left: 0.5rem;
  background-color: var(--color-bg-muted);
  color: var(--color-text-secondary);
}

.content-list__total-badge {
  display: inline-block;
  margin-left: 0.5rem;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
  vertical-align: middle;
  background-color: var(--color-info-bg, var(--color-bg-muted));
  color: var(--color-info, var(--color-text-secondary));
}

.content-list__load-more {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 1rem;
  color: var(--brand-dark-2, #6b7280);
}

.content-list__load-more-spinner {
  width: 1rem;
  height: 1rem;
  border: 2px solid var(--brand-light-2, #e5e7eb);
  border-top-color: var(--color-info);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.content-list__load-more-text {
  font-size: 0.8125rem;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .content-list__create-btn {
    width: 100%;
  }
}
</style>
