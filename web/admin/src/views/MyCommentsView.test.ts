import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import type { Pinia } from 'pinia'
import MyCommentsView from './MyCommentsView.vue'
import { useCommentsStore } from '@/stores/domain/comments'
import api from '@/utils/request'

vi.mock('@/utils/request', () => ({
  default: {
    get: vi.fn(),
  },
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({ role: 'Admin' }),
}))

const mockMyComments = [
  { id: 1, comment: 'Nice post!', status: 'approved', createdAt: '2026-04-28T10:30:00Z' },
  { id: 2, comment: 'Thanks!', status: 'approved', createdAt: '2026-04-27T08:00:00Z' },
]

describe('MyCommentsView', () => {
  let pinia: Pinia
  let commentsStore: ReturnType<typeof useCommentsStore>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.clearAllMocks()

    commentsStore = useCommentsStore()
    vi.spyOn(commentsStore, 'fetchPending').mockResolvedValue([])
    vi.mocked(api.get).mockResolvedValue({
      data: {
        data: mockMyComments,
        meta: {
          pagination: { total: 42, limit: 50, offset: 0, hasMore: false },
        },
      },
    })
  })

  const createWrapper = async () => {
    const wrapper = mount(MyCommentsView, {
      global: {
        plugins: [pinia],
        stubs: {
          Button: { template: '<button><slot /></button>' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()
    return wrapper
  }

  it('renders the total badge with the pagination total', async () => {
    const wrapper = await createWrapper()
    expect(wrapper.find('h1').text()).toContain('Comments')
    expect(wrapper.find('.my-comments__total-badge').text()).toBe('42')
  })

  it('hides the total badge when total is zero', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { data: [], meta: { pagination: { total: 0, limit: 50, offset: 0, hasMore: false } } },
    })
    const wrapper = await createWrapper()
    expect(wrapper.find('.my-comments__total-badge').exists()).toBe(false)
  })

  it('falls back to the list length when pagination total is missing', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { data: mockMyComments, meta: {} },
    })
    const wrapper = await createWrapper()
    expect(wrapper.find('.my-comments__total-badge').text()).toBe('2')
  })
})
