import { ref, computed, onMounted, type Ref } from 'vue'

export type Theme = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

const THEME_KEY = 'lesstruct-theme'

const VALID_THEMES: Theme[] = ['light', 'dark', 'system']

// Shared reactive theme state — persists across all useTheme() calls
let theme: Ref<Theme> | null = null
let initialized = false

// Track system preference media query listener
let mediaQueryListener: ((this: MediaQueryList, ev: MediaQueryListEvent) => any) | null = null

// Reactive ref for system preference to trigger Vue reactivity on matchMedia changes
let systemPreference: Ref<ResolvedTheme> | null = null

/**
 * Check if a value is a valid Theme
 */
function isValidTheme(value: string | null): value is Theme {
  return value !== null && (VALID_THEMES as string[]).includes(value)
}

/**
 * Initialize theme state (lazy initialization)
 */
function initializeTheme() {
  if (initialized) return

  const savedTheme = typeof localStorage !== 'undefined'
    ? localStorage.getItem(THEME_KEY)
    : null

  theme = ref<Theme>(isValidTheme(savedTheme) ? savedTheme : 'system')
  systemPreference = ref<ResolvedTheme>(getSystemPreference())
  initialized = true
}

/**
 * Get system color scheme preference
 * @returns 'dark' if system prefers dark mode, 'light' otherwise
 */
function getSystemPreference(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/**
 * Apply theme to document element
 * @param resolvedTheme The resolved theme to apply
 */
function applyTheme(resolvedTheme: ResolvedTheme) {
  if (typeof document === 'undefined') return
  document.documentElement.setAttribute('data-theme', resolvedTheme)
}

/**
 * Persist theme preference to localStorage
 * @param newTheme The theme to persist
 */
function persistTheme(newTheme: Theme) {
  if (typeof localStorage === 'undefined') return
  if (newTheme === 'system') {
    localStorage.removeItem(THEME_KEY)
  } else {
    localStorage.setItem(THEME_KEY, newTheme)
  }
}

/**
 * Setup system preference change listener
 */
function setupSystemPreferenceListener() {
  if (typeof window === 'undefined') return

  // Remove existing listener if any
  if (mediaQueryListener) {
    window.matchMedia('(prefers-color-scheme: dark)').removeEventListener('change', mediaQueryListener)
    mediaQueryListener = null
  }

  // Add new listener
  mediaQueryListener = (event: MediaQueryListEvent) => {
    // Only update if current theme is 'system'
    if (theme!.value === 'system') {
      const newPreference: ResolvedTheme = event.matches ? 'dark' : 'light'
      systemPreference!.value = newPreference
      applyTheme(newPreference)
    }
  }

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', mediaQueryListener)
}

/**
 * Composable for theme management with localStorage persistence and system preference support
 * @returns Theme state and control methods
 */
export function useTheme() {
  // Initialize theme state on first call
  initializeTheme()

  // Computed property that resolves 'system' to actual theme
  const resolvedTheme = computed<ResolvedTheme>(() => {
    if (theme!.value === 'system') {
      return systemPreference!.value
    }
    return theme!.value
  })

  /**
   * Set theme to specific value
   * @param newTheme The theme to set ('light' | 'dark' | 'system')
   */
  function setTheme(newTheme: Theme) {
    theme!.value = newTheme
    persistTheme(newTheme)
    applyTheme(resolvedTheme.value)

    // Setup or update system preference listener
    if (newTheme === 'system') {
      setupSystemPreferenceListener()
    } else if (mediaQueryListener) {
      // Remove listener if not using system theme
      window.matchMedia('(prefers-color-scheme: dark)').removeEventListener('change', mediaQueryListener)
      mediaQueryListener = null
    }
  }

  /**
   * Toggle between light and dark themes
   * If current theme is 'system', toggles to opposite of current system preference
   */
  function toggleTheme() {
    if (theme!.value === 'light') {
      setTheme('dark')
    } else if (theme!.value === 'dark') {
      setTheme('light')
    } else {
      // If system, toggle to opposite of current system preference
      const systemPref = getSystemPreference()
      setTheme(systemPref === 'light' ? 'dark' : 'light')
    }
  }

  // Initialize theme on mount
  onMounted(() => {
    applyTheme(resolvedTheme.value)

    // Setup system preference listener if theme is 'system'
    if (theme!.value === 'system') {
      setupSystemPreferenceListener()
    }
  })

  return {
    // State
    theme: theme!,
    resolvedTheme,

    // Methods
    setTheme,
    toggleTheme,
  }
}

/**
 * Reset theme state (for testing purposes)
 */
export function resetThemeState() {
  initialized = false
  theme = null
  systemPreference = null
  if (mediaQueryListener) {
    if (typeof window !== 'undefined') {
      window.matchMedia('(prefers-color-scheme: dark)').removeEventListener('change', mediaQueryListener)
    }
    mediaQueryListener = null
  }
}
