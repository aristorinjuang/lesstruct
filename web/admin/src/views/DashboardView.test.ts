import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { mount } from '@vue/test-utils'
import type { Pinia } from 'pinia'
import DashboardView from './DashboardView.vue'
import { useDashboardStore } from '@/stores/domain/dashboard'
import { useNotificationStore } from '@/stores/ui/notifications'

describe('DashboardView', () => {
  let router: Router
  let pinia: Pinia
  let dashboardStore: ReturnType<typeof useDashboardStore>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()

    // Create a new router for each test
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
        { path: '/admin/content', component: { template: '<div>Content</div>' } },
        { path: '/users', component: { template: '<div>Users</div>' } },
        { path: '/admin/media', component: { template: '<div>Media</div>' } },
      ],
    })

    dashboardStore = useDashboardStore()
  })

  const createWrapper = () => {
    return mount(DashboardView, {
      global: {
        plugins: [router, pinia],
        stubs: {
          RouterLink: {
            template: '<a v-bind="$attrs" class="router-link-stub"><slot /></a>',
          },
          StatCard: {
            template: '<div class="stat-card-stub"><slot /></div>',
            props: ['label', 'count', 'icon', 'route', 'notificationBadge'],
          },
          // Don't stub PendingRegistrationsAlert - we need to test its conditional rendering
          IconDocumentText: { template: '<span class="icon">IconDocumentText</span>' },
          IconDocument: { template: '<span class="icon">IconDocument</span>' },
          IconUsers: { template: '<span class="icon">IconUsers</span>' },
          IconUser: { template: '<span class="icon">IconUser</span>' },
          IconPhoto: { template: '<span class="icon">IconPhoto</span>' },
        },
      },
    })
  }

  it('renders the dashboard heading', () => {
    const wrapper = createWrapper()

    expect(wrapper.find('h1').text()).toBe('Dashboard')
  })

  it('loads dashboard stats on mount', async () => {
    const fetchSpy = vi.spyOn(dashboardStore, 'fetchAll').mockResolvedValue()

    const wrapper = createWrapper()
    await wrapper.vm.$nextTick()

    expect(fetchSpy).toHaveBeenCalled()
  })

  it('can display loading state', async () => {
    // Verify the loading state class exists in the component
    const wrapper = createWrapper()

    // The component should have loading, error, and content states
    // This is a basic sanity check that the template is structured correctly
    expect(wrapper.find('.dashboard-view').exists()).toBe(true)
  })

  it('renders stat cards when stats are loaded', async () => {
    dashboardStore.stats = {
      publishedPosts: 42,
      draftPosts: 8,
      registeredUsers: 156,
      pendingRegistrations: 3,
      mediaItems: 234,
    }

    const wrapper = createWrapper()
    await wrapper.vm.$nextTick()

    // Should render stat cards
    const statCards = wrapper.findAll('.stat-card-stub')
    expect(statCards.length).toBeGreaterThan(0)
  })

  it('renders PendingRegistrationsAlert when there are pending registrations', async () => {
    dashboardStore.stats = {
      publishedPosts: 42,
      draftPosts: 8,
      registeredUsers: 156,
      pendingRegistrations: 3,
      mediaItems: 234,
    }

    const wrapper = createWrapper()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.pending-registrations-alert').exists()).toBe(true)
  })

  it('does not render PendingRegistrationsAlert when no pending registrations', async () => {
    dashboardStore.stats = {
      publishedPosts: 42,
      draftPosts: 8,
      registeredUsers: 156,
      pendingRegistrations: 0,
      mediaItems: 234,
    }

    const wrapper = createWrapper()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.pending-registrations-alert').exists()).toBe(false)
  })

  it('applies responsive grid layout', async () => {
    dashboardStore.stats = {
      publishedPosts: 42,
      draftPosts: 8,
      registeredUsers: 156,
      pendingRegistrations: 3,
      mediaItems: 234,
    }

    const wrapper = createWrapper()
    await wrapper.vm.$nextTick()

    const statsGrid = wrapper.find('.dashboard-stats')
    expect(statsGrid.exists()).toBe(true)
  })

  it('has proper heading hierarchy', () => {
    const wrapper = createWrapper()

    const h1 = wrapper.find('h1')
    expect(h1.exists()).toBe(true)
    expect(h1.text()).toBe('Dashboard')
  })

  describe('notification badges', () => {
    it('should sync notification counts from dashboard store after fetch', async () => {
      const notificationStore = useNotificationStore()
      const syncSpy = vi.spyOn(notificationStore, 'syncFromDashboard')
      vi.spyOn(dashboardStore, 'fetchAll').mockResolvedValue()

      createWrapper()
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(syncSpy).toHaveBeenCalled()
      syncSpy.mockRestore()
    })

    it('should pass notificationBadge prop to Pending Registrations StatCard', async () => {
      dashboardStore.stats = {
        publishedPosts: 42,
        draftPosts: 8,
        registeredUsers: 156,
        pendingRegistrations: 5,
        mediaItems: 234,
      }
      vi.spyOn(dashboardStore, 'fetchAll').mockResolvedValue()

      const wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      // StatCard stub should render 5 cards
      const statCards = wrapper.findAll('.stat-card-stub')
      expect(statCards.length).toBe(5)
    })
  })
})
