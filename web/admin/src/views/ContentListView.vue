<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import ContentEditor from '@/components/organisms/ContentEditor.vue'
import DeleteConfirmDialog from '@/components/molecules/DeleteConfirmDialog.vue'
import Toast from '@/components/molecules/Toast.vue'
import { useContentStore } from '@/stores/domain/content'
import { useAuth } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
import type { Content } from '@/types/content'
import type { PostType } from '@/types/posttype'
import { formatRelativeTime } from '@/utils/date'

const router = useRouter()
const route = useRoute()
const contentStore = useContentStore()
const { userId, role } = useAuth()
const { languages, fetchConfig, primaryLanguage } = useConfig()

const isAdmin = computed(() => role.value === 'Admin')

const contents = ref<Content[]>([])
const postTypes = ref<PostType[]>([])
const isLoading = ref(false)
const editingContentId = ref<number | null>(null)
const showEditor = ref(false)
const selectedPostType = ref<string>('all')
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

// Watch route query type to sync with selectedPostType
watch(() => route.fullPath, (newPath) => {
  const type = route.query.type as string | undefined
  if (type) {
    selectedPostType.value = type
  } else {
    selectedPostType.value = 'all'
  }
}, { immediate: true })

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

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

async function loadContents() {
  if (!userId.value) {
    router.push('/login')
    return
  }

  isLoading.value = true
  try {
    const searchOptions = searchQuery.value.length >= 2 ? { search: searchQuery.value } : undefined
    const postTypeParam = selectedPostType.value !== 'all' ? selectedPostType.value : undefined
    const lang = languages.value.length > 1 ? primaryLanguage() : undefined
    const result = isAdmin.value
      ? await contentStore.getAll(undefined, undefined, { ...searchOptions, postType: postTypeParam, language: lang })
      : await contentStore.getByUser({ ...searchOptions, postType: postTypeParam })
    contents.value = result ?? []
  } catch (err) {
    console.error('Failed to load contents:', err)
    contents.value = []
  } finally {
    isLoading.value = false
  }
}

async function loadPostTypes() {
  try {
    postTypes.value = await contentStore.fetchPostTypes()
  } catch (err) {
    console.error('Failed to load post types:', err)
  }
}

watch(() => route.path, (newPath) => {
  if (newPath === '/content/create') {
    showEditor.value = true
    editingContentId.value = null
  } else if (newPath.startsWith('/content/') && newPath.endsWith('/edit')) {
    const id = parseInt(route.params.id as string)
    if (!isNaN(id)) {
      editingContentId.value = id
      showEditor.value = true
    }
  } else {
    showEditor.value = false
    editingContentId.value = null
  }
}, { immediate: true })

function createNew() {
  router.push('/content/create')
}

function editContent(content: Content) {
  router.push(`/content/${content.id}/edit`)
}

async function handleSaved(content: Content, redirectTo?: string) {
  await loadContents()
  if (redirectTo) {
    router.push(redirectTo)
  }
}

async function handleDeleted() {
  displayToast('Content deleted successfully')
  router.push('/content')
}

function handleCancel() {
  router.push('/content')
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
    const deletedId = deletingContent.value.id
    await contentStore.deleteContent(deletedId)
    contents.value = contents.value.filter(c => c.id !== deletedId)
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

const filteredContents = computed(() => {
  if (selectedPostType.value === 'all') {
    return contents.value
  }
  return contents.value.filter(c => c.postType === selectedPostType.value)
})

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

const editingContent = computed(() => {
  if (editingContentId.value === null) return undefined
  return contents.value.find(c => c.id === editingContentId.value)
})
</script>

<template>
  <div class="content-list">
    <div class="page-header">
      <h1 class="page-title">Content</h1>
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

    <div v-if="isLoading" class="state-loading">
      Loading...
    </div>

    <div v-else-if="filteredContents.length === 0" class="state-loading">
      <p>No content yet. Create your first post!</p>
    </div>

    <div v-else class="content-list__items">
      <div
        v-for="item in filteredContents"
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

    <Teleport to="body">
      <div v-if="showEditor && userId" class="content-list__editor-modal">
        <div class="content-list__editor-backdrop" @click="handleCancel"></div>
        <div class="content-list__editor-container">
          <ContentEditor
            :user-id="Number(userId)"
            :content-id="editingContentId || undefined"
            :initial-content="editingContent"
            @saved="handleSaved"
            @deleted="handleDeleted"
            @cancel="handleCancel"
          />
        </div>
      </div>
    </Teleport>
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

.content-list__editor-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 2rem;
  overflow-y: auto;
}

.content-list__editor-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
}

.content-list__editor-container {
  position: relative;
  background-color: var(--color-background);
  border-radius: 0.5rem;
  max-width: 900px;
  width: 100%;
  max-height: calc(100vh - 4rem);
  overflow-y: auto;
}

@media (max-width: 640px) {
  .content-list__create-btn {
    width: 100%;
  }
}
</style>
