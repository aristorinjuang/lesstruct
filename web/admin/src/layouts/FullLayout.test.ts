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

import FullLayout from './FullLayout.vue'
import { useNavigationStore } from '@/stores/ui/navigation'

describe('FullLayout', () => {
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
        {
          path: '/',
          component: FullLayout,
          children: [
            {
              path: 'dashboard',
              component: { template: '<div class="dashboard-content">Dashboard Content</div>' },
            },
            {
              path: 'content',
              component: { template: '<div class="content-list">Content List</div>' },
            },
          ],
        },
      ],
    })
  })

  describe('rendering', () => {
    it('should render the layout structure', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      expect(wrapper.find('.full-layout').exists()).toBe(true)
      expect(wrapper.find('.full-layout__container').exists()).toBe(true)
      expect(wrapper.find('.full-layout__main').exists()).toBe(true)
    })

    it('should render Header component', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      expect(wrapper.findComponent({ name: 'Header' }).exists()).toBe(true)
    })

    it('should render Sidebar component', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      expect(wrapper.findComponent({ name: 'Sidebar' }).exists()).toBe(true)
    })

    it('should render child route content in main area via RouterView', async () => {
      await router.push('/dashboard')

      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      await wrapper.vm.$nextTick()

      expect(wrapper.find('.full-layout__main').html()).toContain('Dashboard Content')
    })
  })

  describe('navigation state', () => {
    it('should update active navigation item when route changes', async () => {
      await router.push('/dashboard')
      await router.isReady()

      mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const store = useNavigationStore()

      expect(store.activeItem).toBe('/dashboard')
    })

    it('should close mobile menu when route changes', async () => {
      const store = useNavigationStore()
      store.toggleMobileMenu()

      expect(store.isMobileMenuOpen).toBe(true)

      // Mount the component to activate the route watcher
      mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      await router.push('/content')
      await router.isReady()

      expect(store.isMobileMenuOpen).toBe(false)
    })
  })

  describe('keyboard interactions', () => {
    it('should close mobile menu on escape key', async () => {
      const store = useNavigationStore()
      store.toggleMobileMenu()

      expect(store.isMobileMenuOpen).toBe(true)

      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      // Trigger escape key event
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

      await wrapper.vm.$nextTick()

      expect(store.isMobileMenuOpen).toBe(false)
    })
  })

  describe('responsive design', () => {
    it('should have correct CSS classes for responsive layout', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const layout = wrapper.find('.full-layout')
      const main = wrapper.find('.full-layout__main')

      expect(layout.exists()).toBe(true)
      expect(main.exists()).toBe(true)
    })
  })

  describe('lifecycle', () => {
    it('should add escape key listener on mount', async () => {
      const addEventListenerSpy = vi.spyOn(document, 'addEventListener')

      mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      expect(addEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function))
    })

    it('should remove escape key listener on unmount', async () => {
      const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      wrapper.unmount()

      expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function))
    })
  })

  describe('accessibility', () => {
    it('should render skip to content link', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const skipLink = wrapper.find('.skip-to-content')
      expect(skipLink.exists()).toBe(true)
    })

    it('should have skip to content link with correct href', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const skipLink = wrapper.find('.skip-to-content')
      expect(skipLink.attributes('href')).toBe('#main-content')
    })

    it('should have skip to content link with correct text', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const skipLink = wrapper.find('.skip-to-content')
      expect(skipLink.text()).toBe('Skip to main content')
    })

    it('should have main element with id for skip link target', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const main = wrapper.find('#main-content')
      expect(main.exists()).toBe(true)
    })

    it('should have tabindex=-1 on main element for focus management', () => {
      const wrapper = mount(FullLayout, {
        global: {
          plugins: [router, pinia],
          stubs: {
            Sidebar: true,
            Header: true,
          },
        },
      })

      const main = wrapper.find('#main-content')
      expect(main.attributes('tabindex')).toBe('-1')
    })
  })
})
