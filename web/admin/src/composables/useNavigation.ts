import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useNavigationStore } from '@/stores/ui/navigation'
import type { NavigationItem } from '@/types/navigation'

export function useNavigation() {
  const route = useRoute()
  const navigationStore = useNavigationStore()

  // Computed properties from store
  const activeItem = computed(() => navigationStore.activeItem)
  const sidebarCollapsed = computed(() => navigationStore.sidebarCollapsed)
  const isMobileMenuOpen = computed(() => navigationStore.isMobileMenuOpen)
  const navigationItems = computed<NavigationItem[]>(() => navigationStore.navigationItems)
  const activeNavigationItem = computed(() => navigationStore.activeNavigationItem)

  // Check if a navigation item is active
  const isItemActive = (path: string): boolean => {
    // Check if the path contains query parameters
    const pathQueryIndex = path.indexOf('?')
    if (pathQueryIndex !== -1) {
      // For paths with query parameters, match both path and query
      const basePath = path.substring(0, pathQueryIndex)
      const queryParams = new URLSearchParams(path.substring(pathQueryIndex))
      // Null check for route.query before creating URLSearchParams
      const routeQueryParams = route.query ? new URLSearchParams(route.query as Record<string, string>) : new URLSearchParams()

      // Compare paths and check if all query parameters match (order-insensitive)
      if (route.path !== basePath) return false

      // Check each parameter in the expected query string exists in route query with same value
      for (const [key, value] of queryParams.entries()) {
        if (routeQueryParams.get(key) !== value) return false
      }
      // Ensure no extra parameters in route query that aren't in expected query
      for (const key of routeQueryParams.keys()) {
        if (!queryParams.has(key)) return false
      }
      return true
    }
    // For paths without query parameters, use exact match or prefix match
    return route.path === path || route.path.startsWith(path + '/')
  }

  // Get the current active item based on route
  const currentActiveItem = computed(() => {
    return navigationItems.value.find((item) => isItemActive(item.path))
  })

  // Update active item based on current route
  function updateActiveItem() {
    const active = currentActiveItem.value
    if (active) {
      navigationStore.setActiveItem(active.path)
    }
  }

  // Actions
  function setActiveItem(path: string) {
    navigationStore.setActiveItem(path)
  }

  function toggleSidebar() {
    navigationStore.toggleSidebar()
  }

  function setSidebarCollapsed(collapsed: boolean) {
    navigationStore.setSidebarCollapsed(collapsed)
  }

  function toggleMobileMenu() {
    navigationStore.toggleMobileMenu()
  }

  function closeMobileMenu() {
    navigationStore.closeMobileMenu()
  }

  return {
    // State
    activeItem,
    sidebarCollapsed,
    isMobileMenuOpen,
    navigationItems,
    activeNavigationItem,
    currentActiveItem,

    // Methods
    isItemActive,
    updateActiveItem,
    setActiveItem,
    toggleSidebar,
    setSidebarCollapsed,
    toggleMobileMenu,
    closeMobileMenu,
  }
}
