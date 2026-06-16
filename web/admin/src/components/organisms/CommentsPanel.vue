<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useCommentsStore } from '@/stores/domain/comments'
import { useConfirmationDialog } from '@/composables/useConfirmationDialog'
import ConfirmationDialog from '@/components/organisms/ConfirmationDialog.vue'
import Button from '@/components/atoms/Button.vue'
import type { Comment, CommentStatus } from '@/types/comment'

interface Props {
  contentId: number
  contentSlug: string
}

const props = defineProps<Props>()
const router = useRouter()
const commentsStore = useCommentsStore()

const isLoading = ref(false)
const error = ref('')
const successMessage = ref('')

const { showConfirmationDialog } = useConfirmationDialog()

const comments = computed(() => commentsStore.comments)
const commentsCount = computed(() => comments.value.length)
const pendingCount = computed(() => comments.value.filter(c => c.status === 'pending').length)

onMounted(async () => {
  await loadComments()
})

async function loadComments() {
  isLoading.value = true
  error.value = ''

  try {
    await commentsStore.fetchForContent(props.contentId)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load comments'
  } finally {
    isLoading.value = false
  }
}

async function approve(comment: Comment) {
  try {
    await commentsStore.approve(comment.id)
    successMessage.value = 'Comment approved'
    setTimeout(() => { successMessage.value = '' }, 3000)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to approve comment'
    setTimeout(() => { error.value = '' }, 5000)
  }
}

async function reject(comment: Comment) {
  try {
    await commentsStore.reject(comment.id)
    successMessage.value = 'Comment rejected'
    setTimeout(() => { successMessage.value = '' }, 3000)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to reject comment'
    setTimeout(() => { error.value = '' }, 5000)
  }
}

async function markAsSpam(comment: Comment) {
  try {
    await commentsStore.markAsSpam(comment.id)
    successMessage.value = 'Comment marked as spam'
    setTimeout(() => { successMessage.value = '' }, 3000)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to mark comment as spam'
    setTimeout(() => { error.value = '' }, 5000)
  }
}

async function confirmDelete(comment: Comment) {
  const confirmed = await showConfirmationDialog({
    title: 'Delete Comment',
    message: `Are you sure you want to delete this comment by ${comment.author}? This action cannot be undone.`,
    confirmButtonText: 'Delete',
    cancelButtonText: 'Cancel',
  })

  if (confirmed) {
    try {
      await commentsStore.deleteComment(comment.id)
      successMessage.value = 'Comment deleted'
      setTimeout(() => { successMessage.value = '' }, 3000)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to delete comment'
      setTimeout(() => { error.value = '' }, 5000)
    }
  }
}

function getStatusBadgeClass(status: CommentStatus): string {
  switch (status) {
    case 'pending':
      return 'status-badge status-badge--pending'
    case 'approved':
      return 'status-badge status-badge--approved'
    case 'rejected':
      return 'status-badge status-badge--rejected'
    case 'spam':
      return 'status-badge status-badge--spam'
    default:
      return 'status-badge'
  }
}

function getStatusLabel(status: CommentStatus): string {
  switch (status) {
    case 'pending':
      return 'Pending'
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'spam':
      return 'Spam'
    default:
      return status
  }
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function viewPublicContent() {
  window.open(`/content/${props.contentSlug}`, '_blank')
}
</script>

<template>
  <div class="comments-panel">
    <div class="comments-panel__header">
      <div class="comments-panel__title">
        <h2>Comments</h2>
        <span class="comments-panel__count">{{ commentsCount }}</span>
      </div>
      <div class="comments-panel__actions">
        <Button
          type="button"
          variant="secondary"
          @click="viewPublicContent"
        >
          View Public Page
        </Button>
        <Button
          type="button"
          variant="secondary"
          @click="router.push('/content')"
        >
          Back to Content
        </Button>
      </div>
    </div>

    <div v-if="pendingCount > 0" class="comments-panel__pending-alert">
      {{ pendingCount }} pending comment{{ pendingCount > 1 ? 's' : '' }} awaiting moderation
    </div>

    <div v-if="error" class="comments-panel__error">
      {{ error }}
    </div>

    <div v-if="successMessage" class="comments-panel__success">
      {{ successMessage }}
    </div>

    <div v-if="isLoading" class="comments-panel__loading">
      Loading comments...
    </div>

    <div v-else-if="comments.length === 0" class="comments-panel__empty">
      <p>No comments yet for this content.</p>
      <Button
        type="button"
        variant="secondary"
        @click="router.push('/content')"
      >
        Back to Content
      </Button>
    </div>

    <div v-else class="comments-list">
      <article
        v-for="comment in comments"
        :key="comment.id"
        class="comment-item"
      >
        <div class="comment-item__header">
          <div class="comment-item__author">
            <span class="comment-item__name">{{ comment.author }}</span>
            <span v-if="comment.username" class="comment-item__username">@{{ comment.username }}</span>
          </div>
          <div class="comment-item__meta">
            <time :datetime="comment.createdAt">{{ formatDate(comment.createdAt) }}</time>
            <span :class="getStatusBadgeClass(comment.status)">
              {{ getStatusLabel(comment.status) }}
            </span>
          </div>
        </div>

        <p class="comment-item__text">{{ comment.comment }}</p>

        <div class="comment-item__actions">
          <Button
            v-if="comment.status === 'pending'"
            type="button"
            variant="primary"
            size="small"
            @click="approve(comment)"
          >
            Approve
          </Button>
          <Button
            v-if="comment.status !== 'rejected'"
            type="button"
            variant="secondary"
            size="small"
            @click="reject(comment)"
          >
            Reject
          </Button>
          <Button
            v-if="comment.status !== 'spam'"
            type="button"
            variant="secondary"
            size="small"
            @click="markAsSpam(comment)"
          >
            Spam
          </Button>
          <Button
            type="button"
            variant="danger"
            size="small"
            @click="confirmDelete(comment)"
          >
            Delete
          </Button>
        </div>
      </article>
    </div>

    <ConfirmationDialog />
  </div>
</template>

<style scoped>
.comments-panel {
  max-width: 900px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.comments-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid var(--brand-light-2);
}

.comments-panel__title {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.comments-panel__title h2 {
  margin: 0;
  font-size: 1.875rem;
  font-weight: 700;
  color: var(--brand-dark-1);
}

.comments-panel__count {
  background-color: var(--brand-primary);
  color: white;
  padding: 0.25rem 0.75rem;
  border-radius: 1rem;
  font-size: 0.875rem;
  font-weight: 600;
}

.comments-panel__actions {
  display: flex;
  gap: 0.75rem;
}

.comments-panel__pending-alert {
  padding: 1rem;
  margin-bottom: 1rem;
  background-color: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  border-radius: 0.375rem;
  color: var(--color-warning-dark);
  font-weight: 500;
}

.comments-panel__error {
  padding: 0.75rem;
  margin-bottom: 1rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
}

.comments-panel__success {
  padding: 0.75rem;
  margin-bottom: 1rem;
  background-color: var(--color-success-bg);
  border: 1px solid var(--color-success-border);
  border-radius: 0.375rem;
  color: var(--color-success-dark);
}

.comments-panel__loading {
  padding: 2rem;
  text-align: center;
  color: var(--brand-dark-2);
}

.comments-panel__empty {
  padding: 3rem 1rem;
  text-align: center;
  color: var(--brand-dark-2);
}

.comments-panel__empty p {
  margin-bottom: 1.5rem;
  font-size: 1.125rem;
}

.comments-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.comment-item {
  padding: 1.25rem;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  transition: box-shadow 0.2s;
}

.comment-item:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.comment-item__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}

.comment-item__author {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.comment-item__name {
  font-weight: 600;
  color: var(--brand-dark-1);
}

.comment-item__username {
  font-size: 0.875rem;
  color: var(--brand-dark-2);
}

.comment-item__meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.comment-item__meta time {
  font-size: 0.875rem;
  color: var(--brand-dark-2);
}

.status-badge {
  padding: 0.25rem 0.75rem;
  border-radius: 1rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.status-badge--pending {
  background-color: var(--color-warning-bg);
  color: var(--color-warning-dark);
}

.status-badge--approved {
  background-color: var(--color-success-bg);
  color: var(--color-success-dark);
}

.status-badge--rejected {
  background-color: var(--color-error-bg);
  color: var(--color-error-dark);
}

.status-badge--spam {
  background-color: var(--color-spam-bg);
  color: var(--color-spam);
}

.comment-item__text {
  margin: 0 0 1rem 0;
  line-height: 1.6;
  color: var(--brand-dark-1);
  white-space: pre-wrap;
}

.comment-item__actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .comments-panel__header {
    flex-direction: column;
    align-items: flex-start;
  }

  .comments-panel__actions {
    width: 100%;
  }

  .comments-panel__actions button {
    flex: 1;
  }

  .comment-item__header {
    flex-direction: column;
  }

  .comment-item__actions {
    flex-direction: column;
  }

  .comment-item__actions button {
    width: 100%;
  }
}
</style>
