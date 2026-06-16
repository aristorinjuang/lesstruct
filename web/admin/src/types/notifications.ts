/**
 * Notification types and interfaces for the notification badge system
 */

/**
 * Notification type identifiers
 */
export type NotificationType = 'pendingRegistrations' | 'pendingComments' | 'pendingUpdates'

/**
 * Notification counts for all badge types
 */
export interface NotificationCounts {
  pendingRegistrations: number
  pendingComments: number
  pendingUpdates: number
}

/**
 * Props for the NotificationBadge atom component
 */
export interface NotificationBadgeProps {
  /** The count to display */
  count: number
  /** Maximum count before showing "99+" (default: 99) */
  maxCount?: number
}

/**
 * Props for the NotificationBadgeWithTooltip molecule component
 */
export interface NotificationBadgeWithTooltipProps extends NotificationBadgeProps {
  /** Text to display in the tooltip */
  tooltipText: string
}
