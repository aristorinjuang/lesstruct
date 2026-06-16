import type { FieldSchema } from './customfield'

/**
 * User management types and interfaces
 */

/**
 * User role types in the system
 */
export type UserRole = 'Admin' | 'Contributor' | 'Commentator'

/**
 * User status types
 */
export type UserStatus = 'Pending' | 'Active' | 'Suspended' | 'SoftDeleted' | 'pending' | 'verified' | 'suspended' | 'soft_deleted'

/**
 * Human-readable labels for user statuses
 */
export const UserStatusLabel: Record<string, string> = {
  Pending: 'Pending',
  Active: 'Active',
  Suspended: 'Suspended',
  'Soft Deleted': 'Soft Deleted',
  pending: 'Pending',
  verified: 'Active',
  suspended: 'Suspended',
  soft_deleted: 'Soft Deleted',
} as const

/**
 * User entity from the backend API
 */
export interface User {
  id: string | number
  username: string
  name?: string
  email: string
  role: UserRole | string
  status: UserStatus | string
  profilePicture?: string
  createdAt: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  customFields?: Record<string, any>
}

/**
 * API response wrapper for user endpoints
 */
export interface UserApiResponse<T> {
  data: T
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
 * Props for UserStatusBadge component
 */
export interface UserStatusBadgeProps {
  status: UserStatus
}

/**
 * Props for UserRoleBadge component
 */
export interface UserRoleBadgeProps {
  role: UserRole
}

/**
 * Props for UserActions component
 */
export interface UserActionsProps {
  userStatus: UserStatus
  disabled?: boolean
}

/**
 * Props for UserTable component
 */
export interface UserTableProps {
  users: User[]
  userFields: FieldSchema[]
  userSystemFields: FieldSchema[]
  isLoading?: boolean
}

/**
 * Props for PendingRegistrations component
 */
export interface PendingRegistrationsProps {
  pendingUsers: User[]
  isLoading?: boolean
}

/**
 * Props for ConfirmationDialog component
 */
export interface ConfirmationDialogProps {
  isOpen: boolean
  title: string
  message: string
  confirmButtonText?: string
  cancelButtonText?: string
}
