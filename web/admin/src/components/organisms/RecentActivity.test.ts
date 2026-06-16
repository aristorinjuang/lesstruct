import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import RecentActivity from './RecentActivity.vue'
import type { ActivityItem } from '@/types/dashboard'

describe('RecentActivity', () => {
  const mockActivities: ActivityItem[] = [
    {
      type: 'post_published',
      title: 'Getting Started with Lesstruct',
      actor: 'ari',
      date: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(), // 2 hours ago
    },
    {
      type: 'user_registered',
      title: 'new_user_123',
      actor: 'new_user_123',
      date: new Date(Date.now() - 30 * 60 * 1000).toISOString(), // 30 minutes ago
    },
    {
      type: 'comment_added',
      title: 'Great post!',
      actor: 'john_doe',
      date: new Date(Date.now() - 5 * 60 * 1000).toISOString(), // 5 minutes ago
    },
  ]

  it('renders the section title', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    expect(wrapper.text()).toContain('Recent Activity')
  })

  it('renders all activity items', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    const items = wrapper.findAll('.recent-activity__item')
    expect(items).toHaveLength(3)
  })

  it('displays activity title', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    expect(wrapper.text()).toContain('Getting Started with Lesstruct')
    expect(wrapper.text()).toContain('new_user_123')
    expect(wrapper.text()).toContain('Great post!')
  })

  it('displays actor information', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    expect(wrapper.text()).toContain('ari')
    expect(wrapper.text()).toContain('new_user_123')
    expect(wrapper.text()).toContain('john_doe')
  })

  it('formats dates in relative time', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    // The component should format dates relative to now
    // Just check that some time text is displayed
    expect(wrapper.text()).toMatch(/ago|hours|minutes|days|Just now|Yesterday/i)
  })

  it('renders icons for each activity type', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    const icons = wrapper.findAll('.recent-activity__icon')
    expect(icons).toHaveLength(3)
  })

  it('displays empty state when no activities', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: [] },
    })

    expect(wrapper.text()).toContain('No recent activity')
  })

  it('applies proper CSS classes', () => {
    const wrapper = mount(RecentActivity, {
      props: { activities: mockActivities },
    })

    expect(wrapper.find('.recent-activity').exists()).toBe(true)
    expect(wrapper.find('.recent-activity__title').exists()).toBe(true)
    expect(wrapper.find('.recent-activity__list').exists()).toBe(true)
  })

  it('handles post_published activity type', () => {
    const wrapper = mount(RecentActivity, {
      props: {
        activities: [
          {
            type: 'post_published',
            title: 'Test Post',
            actor: 'admin',
            date: '2026-04-12T10:00:00Z',
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('Test Post')
  })

  it('handles user_registered activity type', () => {
    const wrapper = mount(RecentActivity, {
      props: {
        activities: [
          {
            type: 'user_registered',
            title: 'newuser',
            actor: 'newuser',
            date: '2026-04-12T10:00:00Z',
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('newuser')
  })

  it('handles comment_added activity type', () => {
    const wrapper = mount(RecentActivity, {
      props: {
        activities: [
          {
            type: 'comment_added',
            title: 'Great article!',
            actor: 'reader',
            date: '2026-04-12T10:00:00Z',
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('Great article!')
  })
})
