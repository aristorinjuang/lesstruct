import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { ref } from 'vue'

// Mock useAuth composable BEFORE importing other modules
vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    role: ref('Admin'),
  })),
}))

import { useNavigation } from './useNavigation'
import { useNavigationStore } from '@/stores/ui/navigation'

// Helper component to test composable in proper Vue context
const TestComponent = defineComponent({
  setup() {
    const navigation = useNavigation()
    return { navigation }
  },
  render() {
    return h('div', ['test'])
  },
})

describe('useNavigation', () => {
  let router: Router
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()
    localStorage.clear()

    // Create a new router for each test
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
        { path: '/content', component: { template: '<div>Content</div>' } },
        { path: '/content/create', component: { template: '<div>Create Content</div>' } },
        { path: '/media', component: { template: '<div>Media</div>' } },
        { path: '/profile', component: { template: '<div>Profile</div>' } },
        { path: '/users', component: { template: '<div>Users</div>' } },
      ],
    })
  })

  const mountTestComponent = () => {
    return mount(TestComponent, {
      global: {
        plugins: [router, pinia],
      },
    })
  }

  describe('initial state', () => {
    it('should return navigation state from store', async () => {
      const wrapper = mountTestComponent()
      const store = useNavigationStore()
      store.setActiveItem('/dashboard')

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.activeItem.value).toBe('/dashboard')
    })

    it('should return sidebar collapsed state from store', async () => {
      const wrapper = mountTestComponent()
      const store = useNavigationStore()
      store.toggleSidebar()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.sidebarCollapsed.value).toBe(true)
    })

    it('should return mobile menu open state from store', async () => {
      const wrapper = mountTestComponent()
      const store = useNavigationStore()
      store.toggleMobileMenu()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isMobileMenuOpen.value).toBe(true)
    })

    it('should return 7 navigation items from store', async () => {
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.navigationItems.value).toHaveLength(7)
      expect(wrapper.vm.navigation.navigationItems.value[0]).toEqual({
        path: '/dashboard',
        label: 'Dashboard',
        icon: 'chart-bars',
        permission: 'admin',
      })
    })
  })

  describe('isItemActive', () => {
    it('should return true when path matches exactly', async () => {
      await router.push('/dashboard')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isItemActive('/dashboard')).toBe(true)
    })

    it('should return true when path starts with item path plus slash', async () => {
      await router.push('/content/create')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isItemActive('/content')).toBe(true)
    })

    it('should return true when path and query parameters match', async () => {
      await router.push('/content?type=page')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isItemActive('/content?type=page')).toBe(true)
    })

    it('should return false when path matches but query parameters differ', async () => {
      await router.push('/content?type=post')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isItemActive('/content?type=page')).toBe(false)
    })

    it('should return false when path does not match', async () => {
      await router.push('/dashboard')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isItemActive('/content')).toBe(false)
    })
  })

  describe('currentActiveItem', () => {
    it('should return dashboard item when on dashboard route', async () => {
      await router.push('/dashboard')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.currentActiveItem.value).toEqual({
        path: '/dashboard',
        label: 'Dashboard',
        icon: 'chart-bars',
        permission: 'admin',
      })
    })

    it('should return pages item when on content with type=page query', async () => {
      await router.push('/content?type=page')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.currentActiveItem.value).toEqual({
        path: '/content?type=page',
        label: 'Pages',
        icon: 'document',
        permission: 'content_creator',
      })
    })

    it('should return undefined when on unknown route', async () => {
      await router.push('/')
      const wrapper = mountTestComponent()

      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.currentActiveItem.value).toBeUndefined()
    })
  })

  describe('updateActiveItem', () => {
    it('should update active item based on current route', async () => {
      await router.push('/dashboard')
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.updateActiveItem()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.activeItem.value).toBe('/dashboard')
    })

    it('should not update active item if route does not match any navigation item', async () => {
      await router.push('/')
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.updateActiveItem()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.activeItem.value).toBe('')
    })
  })

  describe('setActiveItem', () => {
    it('should set active item in store', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.setActiveItem('/media')
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.activeItem.value).toBe('/media')
    })
  })

  describe('toggleSidebar', () => {
    it('should toggle sidebar collapsed state', async () => {
      const wrapper = mountTestComponent()

      expect(wrapper.vm.navigation.sidebarCollapsed.value).toBe(false)

      wrapper.vm.navigation.toggleSidebar()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.sidebarCollapsed.value).toBe(true)
    })
  })

  describe('setSidebarCollapsed', () => {
    it('should set sidebar collapsed to true', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.setSidebarCollapsed(true)
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.sidebarCollapsed.value).toBe(true)
    })

    it('should set sidebar collapsed to false', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.setSidebarCollapsed(true)
      await wrapper.vm.$nextTick()

      wrapper.vm.navigation.setSidebarCollapsed(false)
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.sidebarCollapsed.value).toBe(false)
    })
  })

  describe('toggleMobileMenu', () => {
    it('should toggle mobile menu open state', async () => {
      const wrapper = mountTestComponent()

      expect(wrapper.vm.navigation.isMobileMenuOpen.value).toBe(false)

      wrapper.vm.navigation.toggleMobileMenu()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isMobileMenuOpen.value).toBe(true)
    })
  })

  describe('closeMobileMenu', () => {
    it('should close mobile menu', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.navigation.toggleMobileMenu()
      await wrapper.vm.$nextTick()

      wrapper.vm.navigation.closeMobileMenu()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.navigation.isMobileMenuOpen.value).toBe(false)
    })
  })
})
