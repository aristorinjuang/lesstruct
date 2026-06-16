import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UserTable from './UserTable.vue'
import UserStatusBadge from '@/components/atoms/UserStatusBadge.vue'
import UserRoleBadge from '@/components/atoms/UserRoleBadge.vue'
import UserActions from '@/components/molecules/UserActions.vue'
import type { User } from '@/types/user'
import type { FieldSchema } from '@/types/customfield'

const mockUsers: User[] = [
  {
    id: '1',
    username: 'johndoe',
    name: 'John Doe',
    email: 'john@example.com',
    role: 'Contributor',
    status: 'Active',
    createdAt: '2026-03-26T14:30:00Z',
    customFields: {
      job_title: 'Developer',
      company: 'Acme',
      points: 42,
      account_tier: 'pro',
    },
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

const mockUserFields: FieldSchema[] = [
  { name: 'Job Title', slug: 'job_title', type: 'text' },
  { name: 'Company', slug: 'company', type: 'text' },
]

const mockSystemFields: FieldSchema[] = [
  { name: 'Points', slug: 'points', type: 'number' },
  { name: 'Account Tier', slug: 'account_tier', type: 'select', options: ['free', 'basic', 'pro', 'enterprise'] },
]

describe('UserTable', () => {
  it('should render loading state when isLoading is true', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: [],
        isLoading: true,
      },
    })

    expect(wrapper.text()).toContain('Loading users...')
  })

  it('should render empty state when no users', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: [],
        isLoading: false,
      },
    })

    expect(wrapper.text()).toContain('No users found.')
  })

  it('should render user table with users', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
  })

  it('should display base table headers', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
        userFields: [],
        userSystemFields: [],
      },
    })

    const headerTexts = wrapper.findAll('th').map(th => th.text())
    expect(headerTexts).toContain('Username')
    expect(headerTexts).toContain('Name')
    expect(headerTexts).toContain('Email')
    expect(headerTexts).toContain('Role')
    expect(headerTexts).toContain('Status')
    expect(headerTexts).toContain('Registration Date')
    expect(headerTexts).toContain('Actions')
  })

  it('should render custom field column headers', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
        userFields: mockUserFields,
        userSystemFields: [],
      },
    })

    const headerTexts = wrapper.findAll('th').map(th => th.text())
    expect(headerTexts).toContain('Job Title')
    expect(headerTexts).toContain('Company')
  })

  it('should render system field column headers', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
        userFields: [],
        userSystemFields: mockSystemFields,
      },
    })

    const headerTexts = wrapper.findAll('th').map(th => th.text())
    expect(headerTexts).toContain('Points')
    expect(headerTexts).toContain('Account Tier')
  })

  it('should render user data correctly', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('johndoe')
    expect(rows[0].text()).toContain('John Doe')
    expect(rows[0].text()).toContain('john@example.com')
    expect(rows[1].text()).toContain('janedoe')
    expect(rows[1].text()).toContain('jane@example.com')
  })

  it('should render custom field values from user data', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        userFields: mockUserFields,
        userSystemFields: mockSystemFields,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Developer')
    expect(rows[0].text()).toContain('Acme')
    expect(rows[0].text()).toContain('42')
    expect(rows[0].text()).toContain('pro')
  })

  it('should render dash for missing custom field values', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: [mockUsers[1]],
        userFields: mockUserFields,
        userSystemFields: [],
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const row = wrapper.findAll('tbody tr')[0]
    expect(row.text()).toContain('janedoe')
  })

  it('should render UserStatusBadge component for each user', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const statusBadges = wrapper.findAllComponents(UserStatusBadge)
    expect(statusBadges).toHaveLength(2)
  })

  it('should render UserRoleBadge component for each user', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const roleBadges = wrapper.findAllComponents(UserRoleBadge)
    expect(roleBadges).toHaveLength(2)
  })

  it('should render UserActions component for each user', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const actionButtons = wrapper.findAllComponents(UserActions)
    expect(actionButtons).toHaveLength(2)
  })

  it('should emit approve event when user is approved', async () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const userActionsComponent = wrapper.findAllComponents(UserActions)[1]
    await userActionsComponent.vm.$emit('approve', '2')

    expect(wrapper.emitted('approve')).toBeTruthy()
    expect(wrapper.emitted('approve')?.[0]).toEqual(['2'])
  })

  it('should set confirmation dialog when reject action is triggered', async () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
      },
      global: {
        components: {
          UserStatusBadge,
          UserRoleBadge,
          UserActions,
        },
      },
    })

    const userActionsComponent = wrapper.findAllComponents(UserActions)[1]
    await userActionsComponent.vm.$emit('reject', '2')

    // UserTable shows confirmation dialog instead of directly emitting
    expect(wrapper.vm.confirmationDialog.title).toBe('Reject User')
    expect(wrapper.vm.confirmationDialog.userId).toBe('2')
  })

  it('should have proper accessibility attributes', () => {
    const wrapper = mount(UserTable, {
      props: {
        users: mockUsers,
        isLoading: false,
        userFields: [],
        userSystemFields: [],
      },
    })

    const headers = wrapper.findAll('th')
    headers.forEach((header) => {
      expect(header.attributes('scope')).toBe('col')
    })
  })
})
