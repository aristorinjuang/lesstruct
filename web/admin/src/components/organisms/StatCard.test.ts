import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import StatCard from './StatCard.vue'

describe('StatCard', () => {
  const createRouterWithRoute = () => {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/content', component: { template: '<div>Content</div>' } },
        { path: '/users', component: { template: '<div>Users</div>' } },
        { path: '/admin/media', component: { template: '<div>Media</div>' } },
      ],
    })
  }

  const createWrapper = (props: { label: string; count: number; icon: string; route?: string; notificationBadge?: number }) => {
    const router = createRouterWithRoute()

    return mount(StatCard, {
      props,
      global: {
        plugins: [router],
        stubs: {
          RouterLink: {
            template: '<a href="#" class="router-link-stub" v-bind="$attrs"><slot /></a>',
          },
          IconDocumentText: { template: '<span class="icon-stub">DocumentTextIcon</span>' },
          IconDocument: { template: '<span class="icon-stub">DocumentIcon</span>' },
          IconUsers: { template: '<span class="icon-stub">UsersIcon</span>' },
          IconUser: { template: '<span class="icon-stub">UserIcon</span>' },
          IconPhoto: { template: '<span class="icon-stub">PhotoIcon</span>' },
        },
      },
    })
  }

  it('renders label and count', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
    })

    expect(wrapper.text()).toContain('Published Posts')
    expect(wrapper.text()).toContain('42')
  })

  it('renders the correct icon', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
    })

    const iconElement = wrapper.find('.stat-card__icon')
    expect(iconElement.exists()).toBe(true)
    // The icon component is rendered inside the icon container
    expect(iconElement.find('svg, span').exists()).toBe(true)
  })

  it('renders as a plain div when no route is provided', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
    })

    const cardElement = wrapper.find('.stat-card')
    expect(cardElement.element.tagName).toBe('DIV')
    expect(cardElement.classes()).not.toContain('stat-card--clickable')
  })

  it('renders as a router link when route is provided', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
      route: '/admin/content?type=post&status=published',
    })

    const cardElement = wrapper.find('.stat-card')
    expect(cardElement.classes()).toContain('stat-card--clickable')
  })

  it('applies proper CSS classes', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
    })

    expect(wrapper.find('.stat-card').exists()).toBe(true)
    expect(wrapper.find('.stat-card__icon').exists()).toBe(true)
    expect(wrapper.find('.stat-card__content').exists()).toBe(true)
    expect(wrapper.find('.stat-card__count').exists()).toBe(true)
    expect(wrapper.find('.stat-card__label').exists()).toBe(true)
  })

  it('displays count with proper formatting for large numbers', () => {
    const wrapper = createWrapper({
      label: 'Registered Users',
      count: 1250,
      icon: 'users',
    })

    expect(wrapper.text()).toContain('1250')
  })

  it('handles zero count', () => {
    const wrapper = createWrapper({
      label: 'Draft Posts',
      count: 0,
      icon: 'document',
    })

    expect(wrapper.text()).toContain('0')
  })

  it('includes route as to prop when provided', () => {
    const route = '/admin/content?type=post&status=published'
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
      route,
    })

    const linkElement = wrapper.find('.router-link-stub')
    expect(linkElement.attributes('to')).toBe(route)
  })

  it('has proper accessibility attributes', () => {
    const wrapper = createWrapper({
      label: 'Published Posts',
      count: 42,
      icon: 'document-text',
    })

    const cardElement = wrapper.find('.stat-card')
    // No role attribute — links and divs don't need role="button"
    expect(cardElement.attributes('role')).toBeUndefined()
    expect(cardElement.attributes('aria-label')).toBe('Published Posts: 42')
  })

  describe('notification badges', () => {
    it('should not render notification badge when notificationBadge prop is not provided', () => {
      const wrapper = createWrapper({
        label: 'Published Posts',
        count: 42,
        icon: 'document-text',
      })

      expect(wrapper.find('.notification-badge').exists()).toBe(false)
    })

    it('should not render notification badge when notificationBadge is 0', () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 0,
      })

      expect(wrapper.find('.notification-badge').exists()).toBe(false)
    })

    it('should render notification badge when notificationBadge prop is greater than 0', () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 3,
      })

      expect(wrapper.find('.notification-badge').exists()).toBe(true)
      expect(wrapper.find('.notification-badge').text()).toBe('3')
    })

    it('should position notification badge in the icon container', () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 7,
      })

      const iconContainer = wrapper.find('.stat-card__icon')
      expect(iconContainer.find('.notification-badge').exists()).toBe(true)
    })

    it('should display correct count in notification badge', () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 12,
      })

      expect(wrapper.find('.notification-badge').text()).toBe('12')
    })

    it('should react to changes in notificationBadge prop', async () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 3,
      })

      expect(wrapper.find('.notification-badge').text()).toBe('3')

      await wrapper.setProps({ notificationBadge: 8 })

      expect(wrapper.find('.notification-badge').text()).toBe('8')
    })

    it('should hide notification badge when count changes to 0', async () => {
      const wrapper = createWrapper({
        label: 'Pending Registrations',
        count: 5,
        icon: 'user',
        notificationBadge: 3,
      })

      expect(wrapper.find('.notification-badge').exists()).toBe(true)

      await wrapper.setProps({ notificationBadge: 0 })

      expect(wrapper.find('.notification-badge').exists()).toBe(false)
    })
  })
})
