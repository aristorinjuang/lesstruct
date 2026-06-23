import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/request'
import type { Comment, CommentStatus, UpdateCommentStatusRequest, CommentsResponse, CommentResponse } from '@/types/comment'

export const useCommentsStore = defineStore('comments', () => {
  const comments = ref<Comment[]>([])
  const isLoading = ref(false)
  const error = ref<Error | null>(null)

  async function fetchForContent(contentId: number): Promise<Comment[]> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<CommentsResponse>(`/api/v1/content_items/${contentId}/comments`)
      comments.value = response.data.data
      return comments.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function updateStatus(commentId: number, status: CommentStatus): Promise<Comment> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.put<CommentResponse>(
        `/api/v1/comments/${commentId}/status`,
        { status } as UpdateCommentStatusRequest
      )
      const updated = response.data.data

      // Update local state
      const index = comments.value.findIndex(c => c.id === commentId)
      if (index !== -1) {
        comments.value[index] = updated
      }

      return updated
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function approve(commentId: number): Promise<Comment> {
    return updateStatus(commentId, 'approved')
  }

  async function reject(commentId: number): Promise<Comment> {
    return updateStatus(commentId, 'rejected')
  }

  async function markAsSpam(commentId: number): Promise<Comment> {
    return updateStatus(commentId, 'spam')
  }

  async function deleteComment(commentId: number): Promise<void> {
    isLoading.value = true
    error.value = null

    try {
      await api.delete(`/api/v1/comments/${commentId}`)

      // Remove from local state
      comments.value = comments.value.filter(c => c.id !== commentId)
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function fetchPending(): Promise<Comment[]> {
    isLoading.value = true
    error.value = null

    try {
      const response = await api.get<CommentsResponse>('/api/v1/comments/pending')
      comments.value = response.data.data
      return comments.value
    } catch (err) {
      error.value = err as Error
      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    comments,
    isLoading,
    error,
    fetchForContent,
    fetchPending,
    updateStatus,
    approve,
    reject,
    markAsSpam,
    deleteComment
  }
})
