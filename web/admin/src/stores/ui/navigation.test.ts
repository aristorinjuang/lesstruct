import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

// Mock useAuth composable BEFORE importing the store
vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    role: ref('Admin'),
  })),
}))

import { useNavigationStore } from './navigation'

describe('NavigationStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    // Clear localStorage
    localStorage.clear()
  })

  afterEach(() => {
    // Reset window.innerWidth to desktop default after tests that modify it
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1024,
    })
  })

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const store = useNavigationStore()

      expect(store.activeItem).toBe('')
      expect(store.sidebarCollapsed).toBe(false)
      expect(store.isMobileMenuOpen).toBe(false)
    })

    it('should load sidebar collapsed state from localStorage', () => {
      localStorage.setItem('sidebarCollapsed', 'true')
      const store = useNavigationStore()

      expect(store.sidebarCollapsed).toBe(true)
    })

    it('should load sidebar expanded state from localStorage', () => {
      localStorage.setItem('sidebarCollapsed', 'false')
      const store = useNavigationStore()

      expect(store.sidebarCollapsed).toBe(false)
    })

    it('should start collapsed on tablet regardless of localStorage', () => {
      // Simulate tablet viewport (768px - 1023px)
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 800,
      })

      // Set localStorage to expanded
      localStorage.setItem('sidebarCollapsed', 'false')

      const store = useNavigationStore()

      // Should be collapsed on tablet despite localStorage saying expanded
      expect(store.sidebarCollapsed).toBe(true)
    })

    it('should use localStorage on desktop', () => {
      // Simulate desktop viewport (1024px+)
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 1200,
      })

      localStorage.setItem('sidebarCollapsed', 'true')

      const store = useNavigationStore()

      // Should respect localStorage on desktop
      expect(store.sidebarCollapsed).toBe(true)
    })
  })

  describe('navigation items', () => {
    it('should have all required navigation items', () => {
      const store = useNavigationStore()

      expect(store.navigationItems).toHaveLength(7)
      expect(store.navigationItems).toEqual([
        { path: '/dashboard', label: 'Dashboard', icon: 'chart-bars', permission: 'admin' },
        { path: '/comment', label: 'Comments', icon: 'chat-bubble', permission: 'commentator' },
        { path: '/content?type=post', label: 'Posts', icon: 'document-text', permission: 'content_creator' },
        { path: '/content?type=page', label: 'Pages', icon: 'document', permission: 'content_creator' },
        { path: '/media', label: 'Media', icon: 'photo', permission: 'content_creator' },
        { path: '/users', label: 'Users', icon: 'users', permission: 'admin' },
        { path: '/import', label: 'Import', icon: 'arrow-down-tray', permission: 'admin' },
      ])
    })
  })

  describe('setActiveItem', () => {
    it('should set active item', () => {
      const store = useNavigationStore()

      store.setActiveItem('/dashboard')

      expect(store.activeItem).toBe('/dashboard')
    })

    it('should update active navigation item getter', () => {
      const store = useNavigationStore()

      store.setActiveItem('/dashboard')

      expect(store.activeNavigationItem).toEqual({
        path: '/dashboard',
        label: 'Dashboard',
        icon: 'chart-bars',
        permission: 'admin',
      })
    })

    it('should return undefined for non-existent active item', () => {
      const store = useNavigationStore()

      store.setActiveItem('/non-existent')

      expect(store.activeNavigationItem).toBeUndefined()
    })
  })

  describe('toggleSidebar', () => {
    it('should toggle sidebar collapsed state', () => {
      const store = useNavigationStore()

      expect(store.sidebarCollapsed).toBe(false)

      store.toggleSidebar()

      expect(store.sidebarCollapsed).toBe(true)

      store.toggleSidebar()

      expect(store.sidebarCollapsed).toBe(false)
    })

    it('should persist sidebar state to localStorage', async () => {
      const store = useNavigationStore()
      const setItemSpy = vi.spyOn(Storage.prototype, 'setItem')

      store.toggleSidebar()

      // Wait for watch to trigger
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(setItemSpy).toHaveBeenCalledWith('sidebarCollapsed', 'true')
    })
  })

  describe('setSidebarCollapsed', () => {
    it('should set sidebar collapsed to true', () => {
      const store = useNavigationStore()

      store.setSidebarCollapsed(true)

      expect(store.sidebarCollapsed).toBe(true)
    })

    it('should set sidebar collapsed to false', () => {
      const store = useNavigationStore()
      store.sidebarCollapsed = true

      store.setSidebarCollapsed(false)

      expect(store.sidebarCollapsed).toBe(false)
    })

    it('should persist sidebar state to localStorage', async () => {
      const store = useNavigationStore()
      const setItemSpy = vi.spyOn(Storage.prototype, 'setItem')

      store.setSidebarCollapsed(true)

      // Wait for watch to trigger
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(setItemSpy).toHaveBeenCalledWith('sidebarCollapsed', 'true')
    })
  })

  describe('mobile menu', () => {
    it('should toggle mobile menu open state', () => {
      const store = useNavigationStore()

      expect(store.isMobileMenuOpen).toBe(false)

      store.toggleMobileMenu()

      expect(store.isMobileMenuOpen).toBe(true)

      store.toggleMobileMenu()

      expect(store.isMobileMenuOpen).toBe(false)
    })

    it('should close mobile menu', () => {
      const store = useNavigationStore()
      store.isMobileMenuOpen = true

      store.closeMobileMenu()

      expect(store.isMobileMenuOpen).toBe(false)
    })

    it('should not persist mobile menu state to localStorage', () => {
      const store = useNavigationStore()
      const setItemSpy = vi.spyOn(Storage.prototype, 'setItem')

      store.toggleMobileMenu()

      // Should only have sidebarCollapsed set, not mobile menu
      expect(setItemSpy).not.toHaveBeenCalledWith('isMobileMenuOpen', expect.any(String))
    })
  })
})
