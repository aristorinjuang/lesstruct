import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ThemeSnapshot from '@/components/molecules/ThemeSnapshot.vue'
import { SNAPSHOT_VIEWPORT_HEIGHT, SNAPSHOT_VIEWPORT_WIDTH } from '@/composables/useSnapshotScale'

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

describe('ThemeSnapshot', () => {
  let originalResizeObserver: typeof ResizeObserver

  beforeEach(() => {
    originalResizeObserver = globalThis.ResizeObserver
    ResizeObserverMock.instances = []
    globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver
  })

  afterEach(() => {
    globalThis.ResizeObserver = originalResizeObserver
  })

  it('frames the theme snapshot at the fixed virtual viewport', () => {
    const wrapper = mount(ThemeSnapshot, { props: { name: 'midnight-serif' } })

    const frame = wrapper.find('.theme-snapshot')
    expect(frame.attributes('style')).toContain(
      `aspect-ratio: ${SNAPSHOT_VIEWPORT_WIDTH} / ${SNAPSHOT_VIEWPORT_HEIGHT}`,
    )

    const iframe = wrapper.find('.theme-snapshot iframe')
    expect(iframe.attributes('src')).toBe('/api/v1/themegen/themes/midnight-serif/snapshot')
    expect(iframe.attributes('title')).toBe('Snapshot of midnight-serif')
    expect(iframe.attributes('sandbox')).toBe('allow-same-origin')
    expect(iframe.attributes('loading')).toBe('lazy')
    expect(iframe.attributes('style')).toContain(`width: ${SNAPSHOT_VIEWPORT_WIDTH}px`)
    expect(iframe.attributes('style')).toContain(`height: ${SNAPSHOT_VIEWPORT_HEIGHT}px`)
  })

  it('scales the iframe to the measured frame width', async () => {
    const wrapper = mount(ThemeSnapshot, { props: { name: 'midnight-serif' } })

    lastInstance().trigger(SNAPSHOT_VIEWPORT_WIDTH / 4)
    await wrapper.vm.$nextTick()

    expect(wrapper.find('iframe').attributes('style')).toContain('transform: scale(0.25)')
  })

  it('disconnects the observer on unmount', () => {
    const wrapper = mount(ThemeSnapshot, { props: { name: 'midnight-serif' } })

    wrapper.unmount()

    expect(lastInstance().observed).toHaveLength(0)
  })
})
