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
