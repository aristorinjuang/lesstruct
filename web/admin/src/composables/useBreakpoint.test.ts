import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { useBreakpoint, BREAKPOINTS } from './useBreakpoint'

// Helper component to test composable in proper Vue context
const TestComponent = defineComponent({
  setup() {
    const breakpoint = useBreakpoint()
    return { breakpoint }
  },
  render() {
    return h('div', ['test'])
  },
})

describe('useBreakpoint', () => {
  let originalInnerWidth: number
  let originalAddEventListener: typeof window.addEventListener
  let originalRemoveEventListener: typeof window.removeEventListener

  beforeEach(() => {
    // Store original values
    originalInnerWidth = window.innerWidth
    originalAddEventListener = window.addEventListener
    originalRemoveEventListener = window.removeEventListener

    // Mock window.innerWidth with a getter/setter
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1024,
    })

    // Mock addEventListener and removeEventListener
    window.addEventListener = vi.fn()
    window.removeEventListener = vi.fn()
  })

  afterEach(() => {
    // Restore original values
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: originalInnerWidth,
    })
    window.addEventListener = originalAddEventListener
    window.removeEventListener = originalRemoveEventListener
  })

  const mountTestComponent = () => {
    return mount(TestComponent)
  }

  it('should initialize with desktop breakpoint for 1024px', async () => {
    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    const { isDesktop, isTablet, isMobile, currentBreakpoint } = wrapper.vm.breakpoint

    expect(isDesktop.value).toBe(true)
    expect(isTablet.value).toBe(false)
    expect(isMobile.value).toBe(false)
    expect(currentBreakpoint.value).toBe('desktop')
  })

  it('should detect tablet breakpoint (768px - 1023px)', async () => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 800,
    })

    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    const { isDesktop, isTablet, isMobile, currentBreakpoint } = wrapper.vm.breakpoint

    expect(isDesktop.value).toBe(false)
    expect(isTablet.value).toBe(true)
    expect(isMobile.value).toBe(false)
    expect(currentBreakpoint.value).toBe('tablet')
  })

  it('should detect mobile breakpoint (< 768px)', async () => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 600,
    })

    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    const { isDesktop, isTablet, isMobile, currentBreakpoint } = wrapper.vm.breakpoint

    expect(isDesktop.value).toBe(false)
    expect(isTablet.value).toBe(false)
    expect(isMobile.value).toBe(true)
    expect(currentBreakpoint.value).toBe('mobile')
  })

  it('should detect small mobile breakpoint (< 640px)', async () => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 500,
    })

    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    const { isSmallMobile, isMobile } = wrapper.vm.breakpoint

    expect(isSmallMobile.value).toBe(true)
    expect(isMobile.value).toBe(true)
  })

  it('should provide correct BREAKPOINTS constants', () => {
    expect(BREAKPOINTS.mobile).toBe(640)
    expect(BREAKPOINTS.tablet).toBe(768)
    expect(BREAKPOINTS.desktop).toBe(1024)
  })

  it('should provide windowWidth as computed ref', async () => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1200,
    })

    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    expect(wrapper.vm.breakpoint.windowWidth.value).toBe(1200)
  })

  it('should handle exact breakpoint boundaries correctly', async () => {
    // Test tablet boundary (768px)
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 768,
    })

    const wrapper = mountTestComponent()
    await wrapper.vm.$nextTick()

    const { isTablet: tablet1, isMobile: mobile1 } = wrapper.vm.breakpoint
    expect(tablet1.value).toBe(true)
    expect(mobile1.value).toBe(false)

    // Test desktop boundary (1024px)
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1024,
    })

    const wrapper2 = mountTestComponent()
    await wrapper2.vm.$nextTick()

    const { isDesktop: desktop1, isTablet: tablet2 } = wrapper2.vm.breakpoint
    expect(desktop1.value).toBe(true)
    expect(tablet2.value).toBe(false)
  })
})
