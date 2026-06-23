export type CommentStatus = 'pending' | 'approved' | 'rejected' | 'spam'

export interface Comment {
  id: number
  contentId: number
  userId: number
  comment: string
  status: CommentStatus
  author: string
  username: string | null
  contentTitle?: string
  contentSlug?: string
  createdAt: string
  updatedAt: string
}

export interface UpdateCommentStatusRequest {
  status: CommentStatus
}

export interface CommentsResponse {
  data: Comment[]
  error: null | {
    code: string
    message: string
  }
  meta: {
    timestamp: string
  }
}

export interface CommentResponse {
  data: Comment
  error: null | {
    code: string
    message: string
  }
  meta: {
    timestamp: string
  }
}
