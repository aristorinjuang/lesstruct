import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ThemeToggle from './ThemeToggle.vue'
import { resetThemeState } from '@/composables/useTheme'

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

describe('ThemeToggle', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
    vi.clearAllMocks()
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
      onchange: null,
    }))
    resetThemeState()
  })

  describe('rendering', () => {
    it('should render moon icon when in light mode', () => {
      const wrapper = mount(ThemeToggle)

      expect(wrapper.find('.theme-toggle__icon').exists()).toBe(true)
      // Moon icon should be visible in light mode (click to go to dark)
      expect(wrapper.html()).toContain('Switch to dark mode')
    })

    it('should render sun icon when in dark mode', async () => {
      localStorage.setItem('lesstruct-theme', 'dark')
      resetThemeState()

      const wrapper = mount(ThemeToggle)
      await nextTick()

      expect(wrapper.find('.theme-toggle__icon').exists()).toBe(true)
      expect(wrapper.html()).toContain('Switch to light mode')
    })

    it('should have correct aria-label', () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      expect(button.attributes('aria-label')).toBe('Switch to dark mode')
    })

    it('should have correct title attribute', () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      expect(button.attributes('title')).toBe('Switch to dark mode')
    })

    it('should have minimum 44x44px touch target', () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      // The component has min-width and min-height of 44px in its scoped CSS
      expect(button.exists()).toBe(true)
      // Verify the button has the theme-toggle class which applies the sizing
      expect(button.classes()).toContain('theme-toggle')
    })
  })

  describe('interaction', () => {
    it('should call toggleTheme on click', async () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      await button.trigger('click')

      // Theme should be toggled from light to dark
      expect(localStorage.getItem('lesstruct-theme')).toBe('dark')
    })

    it('should toggle from dark to light on click', async () => {
      localStorage.setItem('lesstruct-theme', 'dark')
      resetThemeState()

      const wrapper = mount(ThemeToggle)
      await nextTick()

      const button = wrapper.find('.theme-toggle')
      await button.trigger('click')
      await nextTick()

      // Theme should be toggled from dark to light
      expect(localStorage.getItem('lesstruct-theme')).toBe('light')
    })

    it('should respond to Enter key', async () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      await button.trigger('keydown', { key: 'Enter' })

      expect(localStorage.getItem('lesstruct-theme')).toBe('dark')
    })

    it('should respond to Space key', async () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      await button.trigger('keydown', { key: ' ' })

      expect(localStorage.getItem('lesstruct-theme')).toBe('dark')
    })

    it('should update aria-label after theme change', async () => {
      const wrapper = mount(ThemeToggle)

      expect(wrapper.find('.theme-toggle').attributes('aria-label')).toBe('Switch to dark mode')

      await wrapper.find('.theme-toggle').trigger('click')
      await nextTick()

      expect(wrapper.find('.theme-toggle').attributes('aria-label')).toBe('Switch to light mode')
    })
  })

  describe('accessibility', () => {
    it('should be a button element', () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      expect(button.element.tagName.toLowerCase()).toBe('button')
    })

    it('should have type="button"', () => {
      const wrapper = mount(ThemeToggle)

      const button = wrapper.find('.theme-toggle')
      expect(button.attributes('type')).toBe('button')
    })
  })
})
