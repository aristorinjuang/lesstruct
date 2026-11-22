import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { SNAPSHOT_VIEWPORT_WIDTH, useSnapshotScale } from './useSnapshotScale'

class ResizeObserverMock {
  static instances: ResizeObserverMock[] = []

  callback: ResizeObserverCallback
  observed: Element[] = []

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback
    ResizeObserverMock.instances.push(this)
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

  trigger(width: number) {
    const entry = { contentRect: { width } } as ResizeObserverEntry
    this.callback([entry], this as unknown as ResizeObserver)
  }
}

function lastInstance(): ResizeObserverMock {
  return ResizeObserverMock.instances[0]!
}

describe('useSnapshotScale', () => {
  let originalResizeObserver: typeof ResizeObserver

  beforeEach(() => {
    originalResizeObserver = globalThis.ResizeObserver
    ResizeObserverMock.instances = []
    globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver
  })

  afterEach(() => {
    globalThis.ResizeObserver = originalResizeObserver
  })

  it('scales the frame content to the virtual viewport width', async () => {
    const TestComponent = defineComponent({
      setup() {
        const frame = ref<HTMLElement | null>(null)
        const { scale } = useSnapshotScale(frame)
        return { frame, scale }
      },
      render() {
        return h('div', { ref: 'frame' }, String(this.scale))
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    expect(ResizeObserverMock.instances).toHaveLength(1)
    expect(lastInstance().observed).toHaveLength(1)

    lastInstance().trigger(SNAPSHOT_VIEWPORT_WIDTH / 2)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toBe('0.5')
  })

  it('keeps the last scale when the frame collapses to zero width', async () => {
    const TestComponent = defineComponent({
      setup() {
        const frame = ref<HTMLElement | null>(null)
        const { scale } = useSnapshotScale(frame)
        return { frame, scale }
      },
      render() {
        return h('div', { ref: 'frame' }, String(this.scale))
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    lastInstance().trigger(SNAPSHOT_VIEWPORT_WIDTH / 4)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toBe('0.25')

    lastInstance().trigger(0)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toBe('0.25')
  })

  it('re-arms the observer when the frame is recreated', async () => {
    const TestComponent = defineComponent({
      setup() {
        const showFrame = ref(false)
        const frame = ref<HTMLElement | null>(null)
        const { scale } = useSnapshotScale(frame)
        return { frame, scale, showFrame }
      },
      render() {
        return h('div', [
          this.showFrame
            ? h('div', { ref: 'frame' }, String(this.scale))
            : h('div', ['placeholder']),
        ])
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    expect(ResizeObserverMock.instances).toHaveLength(0)

    wrapper.vm.showFrame = true
    await wrapper.vm.$nextTick()

    expect(ResizeObserverMock.instances).toHaveLength(1)
    expect(lastInstance().observed).toHaveLength(1)
  })

  it('disconnects the observer on unmount', async () => {
    const TestComponent = defineComponent({
      setup() {
        const frame = ref<HTMLElement | null>(null)
        const { scale } = useSnapshotScale(frame)
        return { frame, scale }
      },
      render() {
        return h('div', { ref: 'frame' }, String(this.scale))
      },
    })

    const wrapper = mount(TestComponent)
    await wrapper.vm.$nextTick()

    wrapper.unmount()

    expect(lastInstance().observed).toHaveLength(0)
  })
})
