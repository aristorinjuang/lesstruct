import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useInfiniteScroll } from './useInfiniteScroll'

class IntersectionObserverMock {
  static instances: IntersectionObserverMock[] = []

  callback: IntersectionObserverCallback
  observed: Element[] = []

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    IntersectionObserverMock.instances.push(this)
  }

  observe(element: Element) {
    this.observed.push(element)
  }

  unobserve() {}

  disconnect() {
    this.observed = []
  }

  takeRecords() {
    return []
  }

  trigger(intersecting: boolean) {
    const entry = { isIntersecting: intersecting } as IntersectionObserverEntry
    this.callback([entry], this as unknown as IntersectionObserver)
  }
}

function lastInstance(): IntersectionObserverMock {
  return IntersectionObserverMock.instances[0]!
}

describe('useInfiniteScroll', () => {
  let originalIntersectionObserver: typeof IntersectionObserver

  beforeEach(() => {
    originalIntersectionObserver = globalThis.IntersectionObserver
    IntersectionObserverMock.instances = []
    globalThis.IntersectionObserver =
      IntersectionObserverMock as unknown as typeof IntersectionObserver
  })

  afterEach(() => {
    globalThis.IntersectionObserver = originalIntersectionObserver
  })

  it('should call onLoadMore when the sentinel intersects', async () => {
    const onLoadMore = vi.fn()

    const TestComponent = defineComponent({
      setup() {
        const showSentinel = ref(true)
        const { sentinel } = useInfiniteScroll(onLoadMore)
        return { sentinel, showSentinel }
      },
      render() {
        return h('div', [this.showSentinel ? h('div', { ref: 'sentinel' }) : null])
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    expect(IntersectionObserverMock.instances).toHaveLength(1)
    expect(lastInstance().observed).toHaveLength(1)

    lastInstance().trigger(true)

    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })

  it('should not call onLoadMore when the sentinel does not intersect', async () => {
    const onLoadMore = vi.fn()

    const TestComponent = defineComponent({
      setup() {
        const { sentinel } = useInfiniteScroll(onLoadMore)
        return { sentinel }
      },
      render() {
        return h('div', [h('div', { ref: 'sentinel' })])
      },
    })

    mount(TestComponent)
    await Promise.resolve()

    lastInstance().trigger(false)

    expect(onLoadMore).not.toHaveBeenCalled()
  })

  it('should not call onLoadMore while disabled', async () => {
    const onLoadMore = vi.fn()
    const disabled = ref(false)

    const TestComponent = defineComponent({
      setup() {
        const { sentinel } = useInfiniteScroll(onLoadMore, { disabled })
        return { sentinel }
      },
      render() {
        return h('div', [h('div', { ref: 'sentinel' })])
      },
    })

    mount(TestComponent)
    await Promise.resolve()

    disabled.value = true
    lastInstance().trigger(true)

    expect(onLoadMore).not.toHaveBeenCalled()
  })

  it('should re-arm the observer when the sentinel is recreated', async () => {
    const onLoadMore = vi.fn()

    const TestComponent = defineComponent({
      setup() {
        const showSentinel = ref(false)
        const { sentinel } = useInfiniteScroll(onLoadMore)
        return { sentinel, showSentinel }
      },
      render() {
        return h('div', [
          this.showSentinel ? h('div', { ref: 'sentinel' }) : h('div', ['placeholder']),
        ])
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    expect(IntersectionObserverMock.instances).toHaveLength(0)

    wrapper.vm.showSentinel = true
    await wrapper.vm.$nextTick()

    expect(IntersectionObserverMock.instances).toHaveLength(1)
    expect(lastInstance().observed).toHaveLength(1)
  })
})
