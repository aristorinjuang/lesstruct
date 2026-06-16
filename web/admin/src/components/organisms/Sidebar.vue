<script setup lang="ts">

import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useNavigation } from '@/composables/useNavigation'
import { useAuth } from '@/composables/useAuth'
import { useDashboardStore } from '@/stores/domain/dashboard'
import { useNotificationStore } from '@/stores/ui/notifications'
import { getIcon } from '@/components/icons'
import IconXMark from '@/components/icons/IconXMark.vue'
import NotificationBadgeWithTooltip from '@/components/molecules/NotificationBadgeWithTooltip.vue'

const {
  navigationItems,
  sidebarCollapsed,
  isMobileMenuOpen,
  isItemActive,
  toggleSidebar,
  closeMobileMenu,
} = useNavigation()

// Auth store
const { role } = useAuth()

// Notification store
const notificationStore = useNotificationStore()
const dashboardStore = useDashboardStore()
const pendingRegistrations = computed(() => notificationStore.pendingRegistrations)

// For tablet hover expand behavior
const isHovered = ref(false)

// For focus trap in mobile menu
const sidebarRef = ref<HTMLElement | null>(null)
const firstFocusableElementRef = ref<HTMLElement | null>(null)
const lastFocusableElementRef = ref<HTMLElement | null>(null)
const previouslyFocusedElementRef = ref<HTMLElement | null>(null)

const sidebarClasses = computed(() => ({
  'sidebar': true,
  'sidebar--collapsed': sidebarCollapsed.value && !isHovered.value,
  'sidebar--mobile-open': isMobileMenuOpen.value,
  'sidebar--hovered': isHovered.value,
}))

function handleToggleSidebar() {
  toggleSidebar()
}

function handleMouseEnter() {
  // Only auto-expand on hover for tablet/desktop when collapsed
  if (sidebarCollapsed.value && typeof window !== 'undefined' && window.innerWidth >= 768) {
    isHovered.value = true
  }
}

function handleMouseLeave() {
  isHovered.value = false
}

function handleMobileMenuClose() {
  closeMobileMenu()
  // Restore focus to the element that opened the menu
  if (previouslyFocusedElementRef.value) {
    previouslyFocusedElementRef.value.focus()
  }
}

// Handle Escape key to close mobile menu
function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && isMobileMenuOpen.value) {
    handleMobileMenuClose()
  }
}

// Focus trap for mobile menu
function handleFocusTrap(event: KeyboardEvent) {
  if (event.key !== 'Tab' || !isMobileMenuOpen.value) return

  if (event.shiftKey) {
    // Shift + Tab
    if (document.activeElement === firstFocusableElementRef.value) {
      event.preventDefault()
      lastFocusableElementRef.value?.focus()
    }
  } else {
    // Tab
    if (document.activeElement === lastFocusableElementRef.value) {
      event.preventDefault()
      firstFocusableElementRef.value?.focus()
    }
  }
}

// Update focusable elements when mobile menu opens
async function updateFocusableElements() {
  if (!isMobileMenuOpen.value || !sidebarRef.value) {
    firstFocusableElementRef.value = null
    lastFocusableElementRef.value = null
    return
  }

  await nextTick()

  // Get all focusable elements within the sidebar
  const focusableElements = sidebarRef.value.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'
  )

  if (focusableElements.length > 0) {
    const firstElement = focusableElements[0]
    const lastElement = focusableElements[focusableElements.length - 1]

    if (firstElement && lastElement) {
      firstFocusableElementRef.value = firstElement
      lastFocusableElementRef.value = lastElement

      // Move focus to the first focusable element when menu opens
      firstElement.focus()
    }
  }
}

// Store previously focused element when menu opens
function storePreviouslyFocusedElement() {
  if (isMobileMenuOpen.value && document.activeElement instanceof HTMLElement) {
    previouslyFocusedElementRef.value = document.activeElement
  }
}

// Watch for mobile menu state changes
watch(isMobileMenuOpen, async (newValue, oldValue) => {
  if (newValue && !oldValue) {
    // Menu is opening
    storePreviouslyFocusedElement()
    await updateFocusableElements()
    // Add event listener for focus trap
    document.addEventListener('keydown', handleFocusTrap)
  } else if (!newValue && oldValue) {
    // Menu is closing
    document.removeEventListener('keydown', handleFocusTrap)
    firstFocusableElementRef.value = null
    lastFocusableElementRef.value = null
  }
})

onMounted(async () => {
  document.addEventListener('keydown', handleKeydown)

  // Sync notification counts from dashboard store (admin only)
  if (role.value === 'admin' && !dashboardStore.stats) {
    try {
      await dashboardStore.fetchDashboardStats()
    } catch {
      // Silently fail — notification badge just won't show
    }
  }
  if (dashboardStore.stats) {
    notificationStore.syncFromDashboard(dashboardStore.stats.pendingRegistrations)
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('keydown', handleFocusTrap)
})
</script>

<template>
  <nav
    ref="sidebarRef"
    :class="sidebarClasses"
    aria-label="Main navigation"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <!-- Collapse toggle button (desktop only, hide on mobile) -->
    <button
      class="sidebar__toggle"
      @click="handleToggleSidebar"
      :aria-label="sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      :aria-expanded="!sidebarCollapsed"
      type="button"
    >
      <span class="sidebar__toggle-icon">{{ sidebarCollapsed ? '›' : '‹' }}</span>
    </button>

    <!-- Navigation items -->
    <ul class="sidebar__nav" role="menubar" aria-label="Main menu">
      <li v-for="item in navigationItems" :key="item.path" class="sidebar__item" role="none">
        <router-link
          :to="item.path"
          class="sidebar__link"
          :class="{ 'sidebar__link--active': isItemActive(item.path) }"
          :aria-current="isItemActive(item.path) ? 'page' : undefined"
          @click="handleMobileMenuClose"
          role="menuitem"
          tabindex="-1"
        >
          <component v-if="getIcon(item.icon)" :is="getIcon(item.icon)" class="sidebar__icon" />
          <span v-else class="sidebar__icon sidebar__icon--placeholder" aria-hidden="true"></span>
          <span class="sidebar__label">{{ item.label }}</span>
          <NotificationBadgeWithTooltip
            v-if="item.path === '/users' && pendingRegistrations > 0"
            :count="pendingRegistrations"
            :tooltip-text="`${pendingRegistrations} pending user registration${pendingRegistrations > 1 ? 's' : ''}`"
          />
        </router-link>
      </li>
    </ul>

    <!-- Mobile close button -->
    <button
      class="sidebar__mobile-close"
      @click="handleMobileMenuClose"
      aria-label="Close menu"
      type="button"
    >
      <IconXMark class="sidebar__mobile-close-icon" />
    </button>
  </nav>

  <!-- Mobile backdrop -->
  <div
    v-if="isMobileMenuOpen"
    class="sidebar__backdrop"
    @click="handleMobileMenuClose"
    aria-hidden="true"
  ></div>
</template>

<style scoped>
.sidebar {
  position: fixed;
  top: 64px;
  left: 0;
  width: 256px;
  height: calc(100vh - 64px);
  background-color: var(--brand-light-1);
  border-right: 1px solid var(--brand-light-2);
  transition: width 0.3s ease;
  z-index: 40;
  display: flex;
  flex-direction: column;
}

.sidebar--collapsed {
  width: 64px;
}

.sidebar__toggle {
  position: absolute;
  top: 0.75rem;
  right: -12px;
  width: 24px;
  height: 24px;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 10;
  transition: background-color 0.2s, border-color 0.2s;
}

.sidebar__toggle:hover {
  background-color: var(--brand-light-1);
  border-color: var(--brand-light-2);
}

.sidebar__toggle-icon {
  font-size: 14px;
  color: var(--brand-dark-1);
  font-weight: bold;
}

.sidebar__nav {
  list-style: none;
  margin: 0;
  padding: 1rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.sidebar__item {
  margin: 0;
}

.sidebar__link {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem;
  padding-right: 2rem;
  border-radius: 0.375rem;
  color: var(--brand-dark-1);
  text-decoration: none;
  transition: background-color 0.2s, color 0.2s, outline-color 0.2s;
  min-height: 44px;
}

.sidebar__link:hover {
  background-color: var(--brand-primary-light);
  color: var(--brand-primary);
}

.sidebar__link--active {
  background-color: var(--brand-primary-light);
  color: var(--brand-primary);
  font-weight: 500;
}

.sidebar__link:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
  background-color: var(--brand-primary-light);
}

.sidebar__toggle:focus-visible,
.sidebar__mobile-close:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.sidebar__icon {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}

.sidebar__label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  opacity: 1;
  transition: opacity 0.2s;
}

.sidebar--collapsed:not(.sidebar--hovered) .sidebar__label {
  display: none;
}

.sidebar--collapsed:not(.sidebar--hovered) .sidebar__link {
  padding: 0.75rem;
  padding-right: 0.75rem;
  justify-content: center;
}

.sidebar__mobile-close {
  display: none;
  min-width: 44px;
  min-height: 44px;
}

.sidebar__mobile-close-icon {
  width: 24px;
  height: 24px;
}

.sidebar__backdrop {
  display: none;
}

/* Desktop styles (1024px+) */
@media (min-width: 1024px) {
  .sidebar {
    width: 256px;
  }

  .sidebar--collapsed:not(.sidebar--hovered) {
    width: 64px;
  }

  /* Show labels when expanded or hovered on desktop */
  .sidebar:not(.sidebar--collapsed) .sidebar__label,
  .sidebar--hovered .sidebar__label {
    display: inline;
  }
}

/* Tablet styles (768px - 1023px) */
@media (min-width: 768px) and (max-width: 1023px) {
  /* Default state: 64px */
  .sidebar {
    width: 64px;
  }

  /* Expanded state (not collapsed) or hovered: 256px */
  .sidebar:not(.sidebar--collapsed),
  .sidebar--hovered {
    width: 256px;
  }

  /* Default (not hovered): center icons, hide labels */
  .sidebar:not(.sidebar--hovered) .sidebar__link {
    justify-content: center;
    padding: 0.75rem;
  }

  .sidebar:not(.sidebar--hovered) .sidebar__label {
    display: none;
  }

  /* Hovered or expanded: left-align, show labels */
  .sidebar--hovered .sidebar__link,
  .sidebar:not(.sidebar--collapsed) .sidebar__link {
    justify-content: flex-start;
    padding: 0.75rem;
    padding-right: 2rem;
  }

  .sidebar--hovered .sidebar__label,
  .sidebar:not(.sidebar--collapsed) .sidebar__label {
    display: inline;
  }
}

/* Mobile styles (<768px) */
@media (max-width: 767px) {
  .sidebar {
    transform: translateX(-100%);
    width: 100%;
    box-shadow: 2px 0 8px rgba(0, 0, 0, 0.1);
    transition: transform 0.3s ease;
  }

  .sidebar--mobile-open {
    transform: translateX(0);
  }

  .sidebar--collapsed {
    width: 100%;
  }

  .sidebar__toggle {
    display: none;
  }

  .sidebar__mobile-close {
    position: absolute;
    top: 1rem;
    right: 1rem;
    min-width: 44px;
    min-height: 44px;
    background: none;
    border: none;
    color: var(--brand-dark-1);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .sidebar__backdrop {
    position: fixed;
    top: 64px;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    z-index: 35;
    display: block;
  }

  .sidebar__label {
    opacity: 1;
    width: auto;
  }
}
</style>
