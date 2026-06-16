<script setup lang="ts">
import { useRouter } from 'vue-router'
import ContentEditor from '@/components/organisms/ContentEditor.vue'
import { useAuth } from '@/composables/useAuth'
import type { Content } from '@/types/content'

const router = useRouter()
const { userId } = useAuth()

if (!userId.value) {
  router.push('/login')
}

function onSaved(content: Content, redirectTo?: string) {
  if (redirectTo) {
    router.push(redirectTo)
  }
}

function onCancel() {
  router.push('/content')
}
</script>

<template>
  <div class="content-create">
    <ContentEditor
      v-if="userId"
      :user-id="Number(userId)"
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
