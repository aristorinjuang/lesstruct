<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getIcon } from '@/components/icons'
import { useNavigation } from '@/composables/useNavigation'

const { navigationItems, isItemActive } = useNavigation()
const route = useRoute()
const navRef = ref<HTMLElement | null>(null)

/**
 * Check if a navigation item is currently active
 * @param path - The path to check
 * @returns true if the path is active
 */
function isActive(path: string): boolean {
  return isItemActive(path)
}

/**
 * Get icon component for a navigation item
 * @param iconName - The name of the icon
 * @returns The icon component or null
 */
function getNavItemIcon(iconName: string) {
  return getIcon(iconName)
}

/**
 * Scroll the currently active nav item into view, centered horizontally.
 * Lets the user see which section they are in even when the nav overflows.
 */
function scrollActiveIntoView(): void {
  const nav = navRef.value
  if (!nav) return
  const activeItem = nav.querySelector<HTMLElement>('.bottom-nav__item--active')
  if (!activeItem) return
  activeItem.scrollIntoView({
    inline: 'center',
    block: 'nearest',
    behavior: 'smooth',
  })
}

onMounted(() => {
  nextTick(scrollActiveIntoView)
})

watch(
  () => [route.path, route.query],
  () => {
    nextTick(scrollActiveIntoView)
  },
)
</script>

<template>
  <nav ref="navRef" class="bottom-nav" aria-label="Bottom navigation">
    <router-link
      v-for="item in navigationItems"
      :key="item.path"
      :to="item.path"
      class="bottom-nav__item"
      :class="{ 'bottom-nav__item--active': isActive(item.path) }"
      :aria-current="isActive(item.path) ? 'page' : undefined"
    >
      <component
        v-if="getNavItemIcon(item.icon)"
        :is="getNavItemIcon(item.icon)"
        class="bottom-nav__icon"
      />
      <span class="bottom-nav__label">{{ item.label }}</span>
    </router-link>
  </nav>
</template>

<style scoped>
.bottom-nav {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 64px;
  background-color: var(--brand-light-1);
  border-top: 1px solid var(--brand-light-2);
  display: flex;
  justify-content: flex-start;
  align-items: center;
  z-index: 40;
  padding: 0 0.5rem;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  scroll-behavior: smooth;
}

.bottom-nav::-webkit-scrollbar {
  display: none;
}

.bottom-nav__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-width: 44px;
  min-height: 44px;
  padding: 0.5rem;
  text-decoration: none;
  color: var(--brand-dark-1);
  transition: color 0.2s;
  flex-shrink: 0;
}

.bottom-nav__item:hover {
  color: var(--brand-primary);
}

.bottom-nav__item--active {
  color: var(--brand-primary);
}

.bottom-nav__item:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
  border-radius: 0.25rem;
}

.bottom-nav__icon {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}

.bottom-nav__label {
  font-size: 12px;
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 60px;
}

/* Hide on tablet and desktop */
@media (min-width: 768px) {
  .bottom-nav {
    display: none;
  }
}
</style>
