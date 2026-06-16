<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '@/utils/request'

interface MyComment {
  id: number
  comment: string
  status: string
  createdAt: string
}

const comments = ref<MyComment[]>([])
const isLoading = ref(false)
const error = ref('')
const successMessage = ref('')
const deletingId = ref<number | null>(null)

onMounted(async () => {
  await loadComments()
})

async function loadComments() {
  isLoading.value = true
  error.value = ''

  try {
    const response = await api.get<{ data: MyComment[] }>('/api/v1/my-comments')
    comments.value = response.data.data
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load comments'
  } finally {
    isLoading.value = false
  }
}

function confirmDelete(comment: MyComment) {
  if (!window.confirm('Delete this comment? This cannot be undone.')) return
  deleteComment(comment)
}

async function deleteComment(comment: MyComment) {
  deletingId.value = comment.id
  try {
    await api.delete('/api/v1/my-comments/' + comment.id)
    comments.value = comments.value.filter(c => c.id !== comment.id)
    successMessage.value = 'Comment deleted.'
    setTimeout(() => { successMessage.value = '' }, 3000)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to delete comment'
    setTimeout(() => { error.value = '' }, 3000)
  } finally {
    deletingId.value = null
  }
}

function getStatusBadgeClass(status: string): string {
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

function getStatusLabel(status: string): string {
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
</script>

<template>
  <div class="my-comments">
    <h1 class="page-title">Comments</h1>

    <div v-if="error" class="alert alert-error">
      {{ error }}
    </div>

    <div v-if="successMessage" class="alert alert-success">
      {{ successMessage }}
    </div>

    <div v-if="isLoading" class="state-loading">
      Loading comments...
    </div>

    <div v-else-if="comments.length === 0" class="my-comments__empty">
      <p>You haven't submitted any comments yet.</p>
    </div>

    <div v-else class="my-comments__list">
      <article
        v-for="comment in comments"
        :key="comment.id"
        class="comment-item"
      >
        <div class="comment-item__header">
          <time :datetime="comment.createdAt">{{ formatDate(comment.createdAt) }}</time>
          <div class="comment-item__actions">
            <span :class="getStatusBadgeClass(comment.status)">
              {{ getStatusLabel(comment.status) }}
            </span>
            <button
              class="comment-item__delete-btn"
              :disabled="deletingId === comment.id"
              @click="confirmDelete(comment)"
            >
              Delete
            </button>
          </div>
        </div>
        <p class="comment-item__text">{{ comment.comment }}</p>
      </article>
    </div>
  </div>
</template>

<style scoped>
.my-comments h1 {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 2px solid var(--brand-light-2);
}

.alert {
  margin-bottom: 1rem;
}

.my-comments__empty {
  padding: 3rem 1rem;
  text-align: center;
  color: var(--brand-dark-2);
}

.my-comments__list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.comment-item {
  padding: 1.25rem;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
}

.comment-item__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}

.comment-item__header time {
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
  line-height: 1.6;
  color: var(--brand-dark-1);
  white-space: pre-wrap;
}

.comment-item__actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.comment-item__delete-btn {
  padding: 0.25rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--color-error-dark);
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  cursor: pointer;
  transition: background-color 0.15s;
}

.comment-item__delete-btn:hover:not(:disabled) {
  background-color: var(--color-error-border);
}

.comment-item__delete-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
