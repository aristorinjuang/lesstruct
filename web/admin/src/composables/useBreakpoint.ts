import { computed, onMounted, onUnmounted, ref } from 'vue'

/**
 * Breakpoint values matching TailwindCSS defaults
 */
export const BREAKPOINTS = {
  mobile: 640,   // sm
  tablet: 768,   // md
  desktop: 1024, // lg
} as const

export type Breakpoint = keyof typeof BREAKPOINTS

/**
 * Composable for responsive breakpoint detection
 *
 * Provides reactive breakpoint information for responsive design.
 * Uses mobile-first approach: styles written for mobile, then enhanced
 * for larger screens using md: and lg: prefixes in CSS.
 *
 * @example
 * const { isMobile, isTablet, isDesktop, currentBreakpoint } = useBreakpoint()
 *
 * if (isMobile.value) {
 *   // Mobile-specific code
 * }
 */
export function useBreakpoint() {
  const windowWidth = ref(0)

  // Update window width on resize
  function updateWidth() {
    if (typeof window !== 'undefined') {
      windowWidth.value = window.innerWidth
    }
  }

  // Computed properties for each breakpoint
  const isMobile = computed(() => windowWidth.value < BREAKPOINTS.tablet)
  const isTablet = computed(
    () =>
      windowWidth.value >= BREAKPOINTS.tablet &&
      windowWidth.value < BREAKPOINTS.desktop
  )
  const isDesktop = computed(() => windowWidth.value >= BREAKPOINTS.desktop)

  const isSmallMobile = computed(() => windowWidth.value < BREAKPOINTS.mobile)

  // Current breakpoint name
  const currentBreakpoint = computed<Breakpoint>(() => {
    if (windowWidth.value >= BREAKPOINTS.desktop) return 'desktop'
    if (windowWidth.value >= BREAKPOINTS.tablet) return 'tablet'
    return 'mobile'
  })

  onMounted(() => {
    updateWidth()
    window.addEventListener('resize', updateWidth)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', updateWidth)
  })

  return {
    windowWidth: computed(() => windowWidth.value),
    isMobile,
    isTablet,
    isDesktop,
    isSmallMobile,
    currentBreakpoint,
    BREAKPOINTS,
  }
}
