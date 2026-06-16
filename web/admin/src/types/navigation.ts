/**
 * Navigation item types for the admin panel
 */

export type NavigationItemIcon =
  | 'dashboard'
  | 'chart-bars'
  | 'document'
  | 'document-text'
  | 'image'
  | 'photo'
  | 'menu'
  | 'x-mark'
  | 'chevron-left'
  | 'chevron-right'
  | 'user'
  | 'users'
  | 'chat-bubble'
  | 'arrow-down-tray'

export interface NavigationItem {
  path: string
  label: string
  icon: NavigationItemIcon
  permission?: string
  badge?: number | string
}

export interface NavigationState {
  activeItem: string
  sidebarCollapsed: boolean
  isMobileMenuOpen: boolean
}

export interface NavigationActions {
  setActiveItem: (path: string) => void
  toggleSidebar: () => void
  setSidebarCollapsed: (collapsed: boolean) => void
  toggleMobileMenu: () => void
  closeMobileMenu: () => void
}
