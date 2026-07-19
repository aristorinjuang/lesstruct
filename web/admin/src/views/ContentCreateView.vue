<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import ContentEditor from '@/components/organisms/ContentEditor.vue'
import { useAuth } from '@/composables/useAuth'
import type { Content } from '@/types/content'

const router = useRouter()
const route = useRoute()
const { userId } = useAuth()

if (!userId.value) {
  router.push('/login')
}

function contentListPath(): string {
  const type = route.query.type as string | undefined
  return type && type !== 'all' ? `/content?type=${type}` : '/content'
}

function onSaved(content: Content, redirectTo?: string) {
  if (redirectTo) {
    router.push(contentListPath())
  }
}

function onCancel() {
  router.push(contentListPath())
}
</script>

<template>
  <div class="content-create">
    <ContentEditor
      v-if="userId"
      :user-id="Number(userId)"
      :initial-post-type="(route.query.type as string) || ''"
      @saved="onSaved"
      @cancel="onCancel"
    />
  </div>
</template>

<style scoped>
.content-create {
  min-height: 100vh;
  background-color: var(--brand-light-1);
}
</style>
