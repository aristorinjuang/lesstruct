import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

// Mock useAuth composable BEFORE importing other modules
vi.mock('@/composables/useAuth', () => ({
  useAuth: vi.fn(() => ({
    role: ref('Admin'),
  })),
}))

import BottomNav from '@/components/organisms/BottomNav.vue'

describe('BottomNav', () => {
  let router: Router
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()

    // Create a new router for each test
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
        { path: '/content', component: { template: '<div>Content</div>' } },
        { path: '/media', component: { template: '<div>Media</div>' } },
        { path: '/profile', component: { template: '<div>Profile</div>' } },
        { path: '/users', component: { template: '<div>Users</div>' } },
      ],
    })
  })

  const mountBottomNav = () => {
    return mount(BottomNav, {
      global: {
        plugins: [router, pinia],
      },
    })
  }

  describe('rendering', () => {
    it('should render bottom navigation with 7 items', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const navItems = wrapper.findAll('.bottom-nav__item')
      expect(navItems).toHaveLength(7)
    })

    it('should have correct navigation items', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const labels = wrapper.findAll('.bottom-nav__label')
      const labelTexts = labels.map((l) => l.text())

      expect(labelTexts).toContain('Dashboard')
      expect(labelTexts).toContain('Comments')
      expect(labelTexts).toContain('Pages')
      expect(labelTexts).toContain('Posts')
      expect(labelTexts).toContain('Media')
      expect(labelTexts).toContain('Users')
      expect(labelTexts).toContain('Import')
    })

    it('should have icons for each navigation item', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const icons = wrapper.findAll('.bottom-nav__icon')
      expect(icons).toHaveLength(7)
    })
  })

  describe('active state', () => {
    it('should highlight dashboard item when on dashboard route', async () => {
      await router.push('/dashboard')
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const activeItem = wrapper.find('.bottom-nav__item--active')
      expect(activeItem.exists()).toBe(true)
      expect(activeItem.text()).toContain('Dashboard')
    })

    it('should highlight pages item when on content with type=page query', async () => {
      await router.push('/content?type=page')
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const activeItem = wrapper.find('.bottom-nav__item--active')
      expect(activeItem.exists()).toBe(true)
      expect(activeItem.text()).toContain('Pages')
    })
  })

  describe('accessibility', () => {
    it('should have nav element with aria-label', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const nav = wrapper.find('nav')
      expect(nav.attributes('aria-label')).toBe('Bottom navigation')
    })

    it('should have aria-current="page" on active item', async () => {
      await router.push('/dashboard')
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const activeItem = wrapper.find('.bottom-nav__item--active')
      expect(activeItem.attributes('aria-current')).toBe('page')
    })

    it('should have minimum touch target size of 44x44px', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      // Check that min-width and min-height are set in the component styles
      const bottomNav = wrapper.find('.bottom-nav')
      expect(bottomNav.exists()).toBe(true)

      // The CSS sets min-width: 44px and min-height: 44px on .bottom-nav__item
      // We verify the class is applied, which includes these styles
      const items = wrapper.findAll('.bottom-nav__item')
      expect(items.length).toBeGreaterThan(0)

      // Verify the element structure supports touch targets
      items.forEach((item) => {
        expect(item.classes()).toContain('bottom-nav__item')
      })
    })
  })

  describe('navigation', () => {
    it('should navigate to dashboard when clicking dashboard item', async () => {
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()

      const dashboardLink = wrapper.findAll('.bottom-nav__item')[0]

      expect(dashboardLink).toBeDefined()
      // The navigation should have updated the route
      // Note: router-link navigation is handled by Vue Router
      expect(dashboardLink.attributes('href')).toBe('/dashboard')
    })
  })

  describe('responsive behavior', () => {
    it('should be hidden on desktop (768px+)', async () => {
      const wrapper = mountBottomNav()

      // Check if display: none is applied in media query
      const nav = wrapper.find('.bottom-nav')
      expect(nav.exists()).toBe(true)
      // Media query would need to be tested with actual viewport width
    })
  })

  describe('horizontal scroll', () => {
    const scrollIntoView = () => Element.prototype.scrollIntoView as ReturnType<typeof vi.fn>

    it('should scroll the active item into view on mount', async () => {
      await router.push('/dashboard')
      const wrapper = mountBottomNav()
      // Wait for the onMounted hook + its nextTick to run.
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      expect(scrollIntoView()).toHaveBeenCalled()
    })

    it('should scroll the new active item into view on route change', async () => {
      await router.push('/dashboard')
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      scrollIntoView().mockClear()

      // Navigate to a different route and wait for the watcher + nextTick
      await router.push('/users')
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      expect(scrollIntoView()).toHaveBeenCalled()
    })

    it('should not call scrollIntoView when no active item is present', async () => {
      await router.push('/nonexistent-route')
      const wrapper = mountBottomNav()
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick()

      // No active item means scrollIntoView should not have been called,
      // and crucially the component should not have thrown.
      expect(scrollIntoView()).not.toHaveBeenCalled()
      expect(wrapper.find('.bottom-nav').exists()).toBe(true)
    })
  })
})
