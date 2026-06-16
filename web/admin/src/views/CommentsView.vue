<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import CommentsPanel from '@/components/organisms/CommentsPanel.vue'

const route = useRoute()

const contentId = computed(() => {
  const id = route.params.id
  return typeof id === 'string' ? parseInt(id, 10) : Array.isArray(id) ? parseInt(id[0], 10) : 0
})

const contentSlug = computed(() => {
  const slug = route.query.slug
  return typeof slug === 'string' ? slug : ''
})
</script>

<template>
  <div class="comments-view">
    <CommentsPanel
      v-if="contentId && contentSlug"
      :content-id="contentId"
      :content-slug="contentSlug"
    />
    <div v-else class="alert alert-error">
      <p>Invalid content parameters.</p>
    </div>
  </div>
</template>
