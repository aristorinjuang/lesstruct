<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import ContentEditor from '@/components/organisms/ContentEditor.vue'
import { useAuth } from '@/composables/useAuth'
import { useContentStore } from '@/stores/domain/content'
import type { Content } from '@/types/content'

const router = useRouter()
const route = useRoute()
const { userId } = useAuth()
const contentStore = useContentStore()

const content = ref<Content | null>(null)
const isLoading = ref(true)
const loadError = ref('')

if (!userId.value) {
  router.push('/login')
}

function contentListPath(): string {
  const type = route.query.type as string | undefined
  return type && type !== 'all' ? `/content?type=${type}` : '/content'
}

onMounted(async () => {
  const id = Number(route.params.id)
  if (isNaN(id)) {
    loadError.value = 'Invalid content ID'
    isLoading.value = false
    return
  }

  try {
    content.value = await contentStore.getById(id)
  } catch {
    loadError.value = 'Failed to load content'
  } finally {
    isLoading.value = false
  }
})

function onSaved(_content: Content, redirectTo?: string) {
  if (redirectTo) {
    router.push(contentListPath())
  }
}

function onDeleted() {
  router.push(contentListPath())
}

function onCancel() {
  router.push(contentListPath())
}
</script>

<template>
  <div class="content-edit">
    <div v-if="isLoading" class="content-edit__state">
      Loading…
    </div>
    <div v-else-if="loadError" class="content-edit__state">
      {{ loadError }}
    </div>
    <ContentEditor
      v-else-if="userId && content"
      :user-id="Number(userId)"
      :content-id="content.id"
      :initial-content="content"
      @saved="onSaved"
      @deleted="onDeleted"
      @cancel="onCancel"
    />
  </div>
</template>

<style scoped>
.content-edit {
  min-height: 100vh;
  background-color: var(--brand-light-1);
}

.content-edit__state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  color: var(--brand-dark-2);
  font-size: 1rem;
}
</style>
