import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { useTheme, resetThemeState } from './useTheme'

// Helper component to test composable in proper Vue context
const TestComponent = defineComponent({
  setup() {
    const theme = useTheme()
    return { theme }
  },
  render() {
    return h('div', ['test'])
  },
})

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
    onchange: null,
  })),
})

describe('useTheme', () => {
  beforeEach(() => {
    // Clear localStorage before each test
    localStorage.clear()
    // Reset document theme
    document.documentElement.removeAttribute('data-theme')
    // Clear all mocks
    vi.clearAllMocks()
    // Reset matchMedia to default (light mode)
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
      onchange: null,
    }))
    // Reset theme state for each test
    resetThemeState()
  })

  const mountTestComponent = () => {
    return mount(TestComponent)
  }

  describe('initial state', () => {
    it('should default to system preference when no saved preference', () => {
      // Mock system preference to light mode (default)
      const wrapper = mountTestComponent()

      expect(wrapper.vm.theme.theme.value).toBe('system')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('light')
    })

    it('should resolve system preference to dark when OS is in dark mode', () => {
      // Mock system preference to dark mode
      window.matchMedia = vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
        onchange: null,
      }))

      const wrapper = mountTestComponent()

      expect(wrapper.vm.theme.theme.value).toBe('system')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('dark')
    })

    it('should load saved preference from localStorage', () => {
      localStorage.setItem('lesstruct-theme', 'dark')

      const wrapper = mountTestComponent()

      expect(wrapper.vm.theme.theme.value).toBe('dark')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('dark')
    })

    it('should apply data-theme attribute to document on init', () => {
      localStorage.setItem('lesstruct-theme', 'dark')

      mountTestComponent()

      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })
  })

  describe('setTheme', () => {
    it('should set theme to light', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.theme.setTheme('light')
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('light')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('light')
      expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    })

    it('should set theme to dark', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.theme.setTheme('dark')
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('dark')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('dark')
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })

    it('should set theme to system', async () => {
      // Mock system preference to dark mode
      window.matchMedia = vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
        onchange: null,
      }))

      const wrapper = mountTestComponent()

      wrapper.vm.theme.setTheme('system')
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('system')
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('dark')
    })

    it('should persist theme preference to localStorage', async () => {
      const wrapper = mountTestComponent()

      wrapper.vm.theme.setTheme('dark')
      await nextTick()

      expect(localStorage.getItem('lesstruct-theme')).toBe('dark')
    })
  })

  describe('toggleTheme', () => {
    it('should toggle from light to dark', async () => {
      const wrapper = mountTestComponent()
      wrapper.vm.theme.setTheme('light')
      await nextTick()

      wrapper.vm.theme.toggleTheme()
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('dark')
    })

    it('should toggle from dark to light', async () => {
      const wrapper = mountTestComponent()
      wrapper.vm.theme.setTheme('dark')
      await nextTick()

      wrapper.vm.theme.toggleTheme()
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('light')
    })

    it('should toggle from system to opposite of system preference', async () => {
      // Mock system preference to light mode (default)
      const wrapper = mountTestComponent()
      wrapper.vm.theme.setTheme('system')
      await nextTick()

      wrapper.vm.theme.toggleTheme()
      await nextTick()

      expect(wrapper.vm.theme.theme.value).toBe('dark')
    })
  })

  describe('system preference change listener', () => {
    it('should update resolvedTheme when system preference changes', async () => {
      let mediaQueryCallback: ((this: MediaQueryList, ev: any) => any) | null = null

      const mockMediaQueryList = {
        matches: false,
        media: '(prefers-color-scheme: dark)',
        addEventListener: vi.fn((event, callback) => {
          if (event === 'change') {
            mediaQueryCallback = callback as any
          }
        }),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
        onchange: null,
      } as any

      window.matchMedia = vi.fn().mockReturnValue(mockMediaQueryList)

      const wrapper = mountTestComponent()
      wrapper.vm.theme.setTheme('system')
      await nextTick()

      expect(wrapper.vm.theme.resolvedTheme.value).toBe('light')

      // Simulate system preference change to dark mode
      if (mediaQueryCallback) {
        const event = { matches: true, media: '(prefers-color-scheme: dark)' }
        mediaQueryCallback.call(mockMediaQueryList, event)
      }

      await nextTick()

      // The DOM attribute should be updated to dark mode immediately
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })

    it('should not update resolvedTheme when theme is not system', async () => {
      const mockMediaQueryList = {
        matches: false,
        media: '(prefers-color-scheme: dark)',
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
        onchange: null,
      } as any

      window.matchMedia = vi.fn().mockReturnValue(mockMediaQueryList)

      const wrapper = mountTestComponent()
      wrapper.vm.theme.setTheme('light')
      await nextTick()

      expect(wrapper.vm.theme.resolvedTheme.value).toBe('light')

      // Simulate system preference change
      const event = { matches: true, media: '(prefers-color-scheme: dark)' }
      mockMediaQueryList.addEventListener.mock.calls[0]?.[1]?.(event)

      await nextTick()

      // Should still be light since we explicitly set light mode
      expect(wrapper.vm.theme.resolvedTheme.value).toBe('light')
    })
  })
})
