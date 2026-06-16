/**
 * Dashboard types and interfaces
 */

/**
 * Dashboard statistics from the backend API
 */
export interface DashboardStats {
  publishedPosts: number
  draftPosts: number
  registeredUsers: number
  pendingRegistrations: number
  mediaItems: number
}

/**
 * Recent activity item types
 */
export type ActivityType = 'post_published' | 'user_registered' | 'comment_added'

/**
 * Recent activity item
 */
export interface ActivityItem {
  type: ActivityType
  title: string
  actor: string
  date: string
}

/**
 * Dashboard statistics response from API
 */
export interface DashboardStatsResponse {
  data: DashboardStats
  error: null | {
    code: string
    message: string
    details: unknown
  }
  meta: {
    timestamp: string
    requestId: string
  }
}

/**
 * Recent activity response from API
 */
export interface RecentActivityResponse {
  data: {
    activities: ActivityItem[]
  }
  error: null | {
    code: string
    message: string
    details: unknown
  }
  meta: {
    timestamp: string
    requestId: string
  }
}

/**
 * Props for StatCard component
 */
export interface StatCardProps {
  label: string
  count: number
  icon: string
  route?: string
}

/**
 * Props for PendingRegistrationsAlert component
 */
export interface PendingRegistrationsAlertProps {
  pendingCount: number
}
