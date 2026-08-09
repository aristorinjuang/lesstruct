import { ref, watch, computed } from 'vue'
import { defineStore } from 'pinia'
import type { NavigationItem } from '@/types/navigation'
import { useAuth } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
import { useContentStore } from '@/stores/domain/content'

// Post types that have dedicated sidebar entries — exclude from dynamic nav
const DEDICATED_POST_TYPES = ['post', 'page', 'media', 'comment']

export const useNavigationStore = defineStore('navigation', () => {
  // State
  const activeItem = ref<string>('')
  const sidebarCollapsed = ref<boolean>(false)
  const isMobileMenuOpen = ref<boolean>(false)

  // Initialize from localStorage
  function initializeFromStorage() {
    if (typeof window === 'undefined') return

    // On tablet, always start collapsed regardless of localStorage
    const width = window.innerWidth
    if (width >= 768 && width <= 1023) {
      sidebarCollapsed.value = true
      return
    }

    const storedCollapsed = localStorage.getItem('sidebarCollapsed')
    if (storedCollapsed !== null) {
      sidebarCollapsed.value = storedCollapsed === 'true'
    }
  }

  // Watch sidebarCollapsed and persist to localStorage
  watch(
    sidebarCollapsed,
    (newValue) => {
      if (typeof window !== 'undefined') {
        try {
          localStorage.setItem('sidebarCollapsed', String(newValue))
        } catch {
          // Silently fail if localStorage is unavailable (quota exceeded, private browsing)
        }
      }
    },
    { immediate: false }
  )

  // Static navigation items (non-post-type). The Comments entry is hidden when
  // the comment system is disabled.
  const staticNavigationItems = computed<NavigationItem[]>(() => {
    const { commentsEnabled } = useConfig()
    const items: NavigationItem[] = [
      { path: '/dashboard', label: 'Dashboard', icon: 'chart-bars', permission: 'admin' },
    ]
    if (commentsEnabled.value) {
      items.push({ path: '/comment', label: 'Comments', icon: 'chat-bubble', permission: 'commentator' })
    }
    return items
  })

  const postFooterNavigationItems: NavigationItem[] = [
    { path: '/media', label: 'Media', icon: 'photo', permission: 'content_creator' },
    { path: '/users', label: 'Users', icon: 'users', permission: 'admin' },
    { path: '/import', label: 'Import', icon: 'arrow-down-tray', permission: 'admin' },
    { path: '/export', label: 'Export', icon: 'arrow-up-tray', permission: 'admin' },
  ]

  // Dynamic post type navigation items (Posts, Pages, + custom types)
  const postTypeNavigationItems = computed<NavigationItem[]>(() => {
    const contentStore = useContentStore()
    const allTypes = contentStore.postTypes ?? []
    const hiddenSlugs = new Set(
      allTypes.filter(pt => pt.hidden).map(pt => pt.slug)
    )

    const customTypes = allTypes
      .filter(pt => !DEDICATED_POST_TYPES.includes(pt.slug))
      .filter(pt => !pt.hidden)
      .sort((a, b) => a.name.localeCompare(b.name))

    // Built-in types with dedicated entries. A hidden post type (e.g.
    // [[post_type]] slug = "post" hidden = true) is absent from the visible
    // list; its sidebar entry is skipped. Before post types load (empty
    // array) everything shows by default, so there is no flash on normal
    // instances.
    const items: NavigationItem[] = []
    if (!hiddenSlugs.has('post')) {
      items.push({ path: '/content?type=post', label: 'Posts', icon: 'document-text', permission: 'content_creator' })
    }
    if (!hiddenSlugs.has('page')) {
      items.push({ path: '/content?type=page', label: 'Pages', icon: 'document', permission: 'content_creator' })
    }

    for (const pt of customTypes) {
      items.push({
        path: `/content?type=${pt.slug}`,
        label: pt.name,
        icon: 'document-text',
        permission: 'content_creator',
      })
    }

    return items
  })

  // All navigation items (static + dynamic post types + footer)
  const allNavigationItems = computed<NavigationItem[]>(() => [
    ...staticNavigationItems.value,
    ...postTypeNavigationItems.value,
    ...postFooterNavigationItems,
  ])

  // Check if user has permission for a navigation item
  function hasPermission(itemPermission: string | undefined): boolean {
    if (!itemPermission) {
      return true
    }

    const { role } = useAuth()
    const userRole = role.value?.toLowerCase()

    if (!userRole) {
      return false
    }

    // Map backend roles to navigation permissions
    // Backend: 'Admin', 'Contributor', 'Commentator'
    // Navigation: 'admin', 'content_creator', 'commentator'
    if (itemPermission === 'admin') {
      return userRole === 'admin'
    }

    if (itemPermission === 'content_creator') {
      return ['admin', 'contributor'].includes(userRole)
    }

    if (itemPermission === 'commentator') {
      return ['admin', 'contributor', 'commentator'].includes(userRole)
    }

    return false
  }

  // Filtered navigation items based on user's role
  const navigationItems = computed<NavigationItem[]>(() => {
    return allNavigationItems.value.filter((item) => hasPermission(item.permission))
  })

  // Actions
  function setActiveItem(path: string) {
    activeItem.value = path
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setSidebarCollapsed(collapsed: boolean) {
    sidebarCollapsed.value = collapsed
  }

  function toggleMobileMenu() {
    isMobileMenuOpen.value = !isMobileMenuOpen.value
  }

  function closeMobileMenu() {
    isMobileMenuOpen.value = false
  }

  // Getters
  const activeNavigationItem = computed(() => {
    return navigationItems.value.find((item) => item.path === activeItem.value)
  })

  // Initialize on store creation
  initializeFromStorage()

  return {
    // State
    activeItem,
    sidebarCollapsed,
    isMobileMenuOpen,
    navigationItems,

    // Actions
    setActiveItem,
    toggleSidebar,
    setSidebarCollapsed,
    toggleMobileMenu,
    closeMobileMenu,

    // Getters
    activeNavigationItem,
  }
})
