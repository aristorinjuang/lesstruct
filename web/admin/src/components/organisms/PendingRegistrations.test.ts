import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PendingRegistrations from './PendingRegistrations.vue'
import UserActions from '@/components/molecules/UserActions.vue'
import type { User } from '@/types/user'

const mockPendingUsers: User[] = [
  {
    id: '1',
    username: 'johndoe',
    email: 'john@example.com',
    role: 'Contributor',
    status: 'Pending',
    createdAt: '2026-03-26T14:30:00Z',
  },
  {
    id: '2',
    username: 'janedoe',
    email: 'jane@example.com',
    role: 'Commentator',
    status: 'Pending',
    createdAt: '2026-03-27T10:00:00Z',
  },
]

describe('PendingRegistrations', () => {
  it('should not render when there are no pending users', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: [],
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    expect(wrapper.find('.pending-registrations').exists()).toBe(false)
  })

  it('should render when there are pending users', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    expect(wrapper.find('.pending-registrations').exists()).toBe(true)
    expect(wrapper.text()).toContain('Pending Registrations')
  })

  it('should display loading state when isLoading is true', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        isLoading: true,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
    })

    expect(wrapper.text()).toContain('Loading pending registrations...')
  })

  it('should render all pending users', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    const cards = wrapper.findAll('.pending-registrations__card')
    expect(cards).toHaveLength(2)
  })

  it('should display user information correctly', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    expect(wrapper.text()).toContain('johndoe')
    expect(wrapper.text()).toContain('john@example.com')
    expect(wrapper.text()).toContain('janedoe')
    expect(wrapper.text()).toContain('jane@example.com')
  })

  it('should render UserActions component for each user', () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    const actionButtons = wrapper.findAllComponents(UserActions)
    expect(actionButtons).toHaveLength(2)
  })

  it('should emit approve event when approve action is triggered', async () => {
    const onApprove = vi.fn()
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove,
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    const userActionsComponent = wrapper.findAllComponents(UserActions)[0]
    await userActionsComponent.vm.$emit('approve', '1')

    expect(wrapper.emitted('approve')).toBeTruthy()
    expect(wrapper.emitted('approve')?.[0]).toEqual(['1'])
  })

  it('should show confirmation dialog when reject action is triggered', async () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    const userActionsComponent = wrapper.findAllComponents(UserActions)[0]
    await userActionsComponent.vm.$emit('reject', '1')

    // After emit, confirmation dialog should be set (we can check for dialog title)
    expect(wrapper.vm.confirmationDialog.title).toBe('Reject User')
  })

  it('should show confirmation dialog when mark as spam action is triggered', async () => {
    const wrapper = mount(PendingRegistrations, {
      props: {
        pendingUsers: mockPendingUsers,
        onApprove: vi.fn(),
        onReject: vi.fn(),
        onMarkAsSpam: vi.fn(),
      },
      global: {
        components: {
          UserActions,
        },
      },
    })

    const userActionsComponent = wrapper.findAllComponents(UserActions)[0]
    await userActionsComponent.vm.$emit('markAsSpam', '1')

    // After emit, confirmation dialog should be set
    expect(wrapper.vm.confirmationDialog.title).toBe('Mark as Spam')
  })
})
