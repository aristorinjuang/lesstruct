// TipTap JSON content types
export interface TipTapContent {
  type: 'doc'
  content: TipTapNode[]
}

export interface TipTapNode {
  type: string
  attrs?: Record<string, unknown>
  content?: TipTapNode[]
  marks?: TipTapMark[]
  text?: string
}

export interface TipTapMark {
  type: string
  attrs?: Record<string, unknown>
}

export interface Content {
  id: number
  userId: number
  title: string
  slug: string
  content: string // TipTap JSON string
  tags: string[]
  status: 'draft' | 'published'
  postType: string
  metaDescription?: string
  ogTitle?: string
  ogDescription?: string
  author?: string
  username?: string
  allowComments?: boolean
  customFields?: Record<string, any>
  updatedBy?: number
  updatedByUsername?: string
  createdAt: string
  updatedAt: string
  language: string
  translationGroupId?: number
  translations?: Content[]
}

export interface CreateContentRequest {
  title: string
  content: string // TipTap JSON string
  tags: string[]
  status: 'draft' | 'published'
  postType: string
  userId: number
  metaDescription?: string
  ogTitle?: string
  ogDescription?: string
  allowComments?: boolean
  customFields?: Record<string, any>
  language?: string
  translationGroupId?: number
}

export interface UpdateContentRequest {
  title: string
  content: string // TipTap JSON string
  tags: string[]
  status: 'draft' | 'published'
  postType: string
  metaDescription?: string
  ogTitle?: string
  ogDescription?: string
  allowComments?: boolean
  customFields?: Record<string, any>
  language?: string
  translationGroupId?: number
}

export interface GenerateSlugRequest {
  title: string
}

export interface GenerateSlugResponse {
  slug: string
}

export interface ApiResponse<T> {
  data: T
  error: null | {
    code: string
    message: string
    details: unknown
  }
  meta: {
    timestamp: string
  }
}

export interface SEOMetadata {
  metaDescription: string
  ogTitle: string
  ogDescription: string
  ogImage: string
  ogUrl: string
  ogType: string
  ogSiteName: string
  twitterCard: string
  twitterTitle: string
  twitterDescription: string
  twitterImage: string
  jsonLd: Record<string, unknown>
}

export interface SEOResponse {
  seo: SEOMetadata
}
