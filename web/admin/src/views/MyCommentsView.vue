<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '@/utils/request'
import { useCommentsStore } from '@/stores/domain/comments'
import { useAuth } from '@/composables/useAuth'
import Button from '@/components/atoms/Button.vue'
import type { Comment } from '@/types/comment'

interface MyComment {
  id: number
  comment: string
  status: string
  createdAt: string
}

const commentsStore = useCommentsStore()
const { role } = useAuth()

const isAdmin = computed(() => role.value === 'Admin')

// My comments (all roles)
const myComments = ref<MyComment[]>([])
const isLoading = ref(false)
const error = ref('')
const successMessage = ref('')
const deletingId = ref<number | null>(null)

// Pending moderation (admin only)
const pendingComments = computed(() => commentsStore.comments.filter(c => c.status === 'pending'))
const pendingLoading = ref(false)
const pendingError = ref('')
const pendingSuccess = ref('')

onMounted(async () => {
  await loadComments()
  if (isAdmin.value) {
    await loadPending()
  }
})

async function loadComments() {
  isLoading.value = true
  error.value = ''

  try {
    const response = await api.get<{ data: MyComment[] }>('/api/v1/my-comments')
    myComments.value = response.data.data
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load comments'
  } finally {
    isLoading.value = false
  }
}

async function loadPending() {
  pendingLoading.value = true
  pendingError.value = ''

  try {
    await commentsStore.fetchPending()
  } catch (err) {
    pendingError.value = err instanceof Error ? err.message : 'Failed to load pending comments'
  } finally {
    pendingLoading.value = false
  }
}

function contentLink(comment: Comment) {
  return {
    path: `/content/${comment.contentId}/comments`,
    query: { slug: comment.contentSlug || '' },
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
    myComments.value = myComments.value.filter(c => c.id !== comment.id)
    successMessage.value = 'Comment deleted.'
    setTimeout(() => { successMessage.value = '' }, 3000)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to delete comment'
    setTimeout(() => { error.value = '' }, 3000)
  } finally {
    deletingId.value = null
  }
}

async function approve(comment: Comment) {
  try {
    await commentsStore.approve(comment.id)
    pendingSuccess.value = 'Comment approved'
    setTimeout(() => { pendingSuccess.value = '' }, 3000)
  } catch (err) {
    pendingError.value = err instanceof Error ? err.message : 'Failed to approve comment'
    setTimeout(() => { pendingError.value = '' }, 5000)
  }
}

async function reject(comment: Comment) {
  try {
    await commentsStore.reject(comment.id)
    pendingSuccess.value = 'Comment rejected'
    setTimeout(() => { pendingSuccess.value = '' }, 3000)
  } catch (err) {
    pendingError.value = err instanceof Error ? err.message : 'Failed to reject comment'
    setTimeout(() => { pendingError.value = '' }, 5000)
  }
}

async function markAsSpam(comment: Comment) {
  try {
    await commentsStore.markAsSpam(comment.id)
    pendingSuccess.value = 'Comment marked as spam'
    setTimeout(() => { pendingSuccess.value = '' }, 3000)
  } catch (err) {
    pendingError.value = err instanceof Error ? err.message : 'Failed to mark comment as spam'
    setTimeout(() => { pendingError.value = '' }, 5000)
  }
}

function confirmDeletePending(comment: Comment) {
  if (!window.confirm(`Delete this comment by ${comment.author || 'Anonymous'}? This cannot be undone.`)) {
    return
  }
  deletePending(comment)
}

async function deletePending(comment: Comment) {
  try {
    await commentsStore.deleteComment(comment.id)
    pendingSuccess.value = 'Comment deleted'
    setTimeout(() => { pendingSuccess.value = '' }, 3000)
  } catch (err) {
    pendingError.value = err instanceof Error ? err.message : 'Failed to delete comment'
    setTimeout(() => { pendingError.value = '' }, 5000)
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

    <section v-if="isAdmin" class="pending-section">
      <div class="pending-section__header">
        <h2 class="pending-section__title">
          Pending Comments
          <span v-if="pendingComments.length" class="pending-section__count">{{ pendingComments.length }}</span>
        </h2>
      </div>

      <div v-if="pendingError" class="alert alert-error">
        {{ pendingError }}
      </div>

      <div v-if="pendingSuccess" class="alert alert-success">
        {{ pendingSuccess }}
      </div>

      <div v-if="pendingLoading" class="state-loading">
        Loading pending comments...
      </div>

      <div v-else-if="pendingComments.length === 0" class="my-comments__empty">
        <p>No pending comments awaiting moderation.</p>
      </div>

      <div v-else class="pending-section__list">
        <article
          v-for="comment in pendingComments"
          :key="comment.id"
          class="comment-item comment-item--pending"
        >
          <div v-if="comment.contentTitle" class="comment-item__context">
            on
            <router-link :to="contentLink(comment)" class="comment-item__context-link">
              {{ comment.contentTitle }}
            </router-link>
          </div>

          <div class="comment-item__header">
            <div class="comment-item__author">
              <span class="comment-item__name">{{ comment.author || 'Anonymous' }}</span>
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
              type="button"
              variant="primary"
              size="small"
              @click="approve(comment)"
            >
              Approve
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="small"
              @click="reject(comment)"
            >
              Reject
            </Button>
            <Button
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
              @click="confirmDeletePending(comment)"
            >
              Delete
            </Button>
          </div>
        </article>
      </div>
    </section>

    <section class="my-comments__section">
      <h2 class="section-title">My Comments</h2>

      <div v-if="error" class="alert alert-error">
        {{ error }}
      </div>

      <div v-if="successMessage" class="alert alert-success">
        {{ successMessage }}
      </div>

      <div v-if="isLoading" class="state-loading">
        Loading comments...
      </div>

      <div v-else-if="myComments.length === 0" class="my-comments__empty">
        <p>You haven't submitted any comments yet.</p>
      </div>

      <div v-else class="my-comments__list">
        <article
          v-for="comment in myComments"
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
    </section>
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

.state-loading {
  padding: 2rem;
  text-align: center;
  color: var(--brand-dark-2);
}

.pending-section {
  margin-bottom: 2.5rem;
}

.pending-section__header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.pending-section__title {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--brand-dark-1);
}

.pending-section__count {
  background-color: var(--color-warning-bg);
  color: var(--color-warning-dark);
  border: 1px solid var(--color-warning-border);
  padding: 0.125rem 0.625rem;
  border-radius: 1rem;
  font-size: 0.875rem;
  font-weight: 600;
}

.pending-section__list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.my-comments__section {
  margin-top: 0.5rem;
}

.section-title {
  margin: 0 0 1rem 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--brand-dark-1);
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

.comment-item__context {
  margin-bottom: 0.5rem;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
}

.comment-item__context-link {
  font-weight: 600;
  color: var(--brand-primary);
  text-decoration: none;
}

.comment-item__context-link:hover {
  text-decoration: underline;
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

@media (max-width: 640px) {
  .comment-item__header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
