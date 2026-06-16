<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import { useNavigation } from '@/composables/useNavigation'
import Sidebar from '@/components/organisms/Sidebar.vue'
import Header from '@/components/organisms/Header.vue'
import BottomNav from '@/components/organisms/BottomNav.vue'

const route = useRoute()
const { updateActiveItem, closeMobileMenu, sidebarCollapsed } = useNavigation()

// Update active navigation item when route changes
watch(
  () => route.path,
  () => {
    updateActiveItem()
    // Close mobile menu on route change
    closeMobileMenu()
  },
  { immediate: true }
)

// Handle escape key to close mobile menu
function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeMobileMenu()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
})
</script>

<template>
  <div class="narrow-layout">
    <!-- Skip to content link for accessibility -->
    <a href="#main-content" class="skip-to-content">
      Skip to main content
    </a>

    <Header />
    <div class="narrow-layout__container">
      <Sidebar />
      <main
        id="main-content"
        class="narrow-layout__main"
        :class="{ 'narrow-layout__main--sidebar-collapsed': sidebarCollapsed }"
        tabindex="-1"
      >
        <div class="narrow-layout__content">
          <RouterView :key="route.fullPath" />
        </div>
      </main>
    </div>

    <!-- Bottom navigation for mobile -->
    <BottomNav />
  </div>
</template>

<style scoped>
.skip-to-content {
  position: absolute;
  top: -40px;
  left: 0;
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
  padding: 8px 16px;
  text-decoration: none;
  z-index: 100;
  border-radius: 0 0 4px 0;
  font-weight: 500;
  transition: top 0.3s ease;
}

.skip-to-content:focus {
  top: 0;
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.narrow-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background-color: var(--brand-light-1);
  padding-top: 64px;
}

.narrow-layout__container {
  display: flex;
  flex: 1;
  position: relative;
}

.narrow-layout__main {
  flex: 1;
  padding: 1.5rem;
  overflow-y: auto;
  margin-left: 256px;
  transition: margin-left 0.3s ease, width 0.3s ease;
  width: calc(100% - 256px);
}

.narrow-layout__content {
  max-width: 800px;
  margin: 0 auto;
}

/* Desktop - sidebar always visible */
@media (min-width: 1024px) {
  .narrow-layout__main--sidebar-collapsed {
    margin-left: 64px;
    width: calc(100% - 64px);
  }
}

/* Tablet - sidebar collapsed by default (64px) */
@media (min-width: 768px) and (max-width: 1023px) {
  .narrow-layout__main {
    margin-left: 64px;
    width: calc(100% - 64px);
  }

  .narrow-layout__main--sidebar-collapsed {
    margin-left: 64px;
    width: calc(100% - 64px);
  }
}

/* Mobile - sidebar hidden by default */
@media (max-width: 767px) {
  .narrow-layout__main {
    margin-left: 0;
    width: 100%;
    padding: 1rem;
    padding-bottom: 80px; /* Extra padding for bottom navigation */
  }

  .narrow-layout__content {
    max-width: 100%;
  }
}
</style>
