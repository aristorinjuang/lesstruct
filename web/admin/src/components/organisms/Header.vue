<script setup lang="ts">

import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth, setAuthToken, clearAuth } from '@/composables/useAuth'
import { useNavigation } from '@/composables/useNavigation'
import IconMenu from '@/components/icons/IconMenu.vue'
import IconUser from '@/components/icons/IconUser.vue'
import IconLogout from '@/components/icons/IconLogout.vue'
import IconKey from '@/components/icons/IconKey.vue'
import ThemeToggle from '@/components/atoms/ThemeToggle.vue'
import api from '@/utils/request'

const router = useRouter()
const { userId, isAuthenticated, role } = useAuth()
const { isMobileMenuOpen, sidebarCollapsed, toggleSidebar } = useNavigation()

const profileDropdownOpen = ref(false)
const profileDropdownRef = ref<HTMLElement | null>(null)
const profilePicture = ref<string | null>(null)

function handleClickOutside(event: MouseEvent) {
  if (profileDropdownRef.value && !profileDropdownRef.value.contains(event.target as Node)) {
    profileDropdownOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  fetchProfilePicture()
})

async function fetchProfilePicture() {
  try {
    const response = await api.get<{ data: { profile: { profilePicture?: string } } }>('/api/profile')
    profilePicture.value = response.data.data.profile.profilePicture || null
  } catch {
    // Ignore - fall back to icon
  }
}

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

function handleLogoClick() {
  if (role.value === 'Admin') {
    router.push('/dashboard')
  } else if (role.value === 'Commentator') {
    router.push('/comment')
  } else {
    router.push('/content?type=post')
  }
}

function toggleProfileDropdown() {
  profileDropdownOpen.value = !profileDropdownOpen.value
}

function handleProfileClick() {
  profileDropdownOpen.value = false
  router.push('/profile')
}

function handleApiKeysClick() {
  profileDropdownOpen.value = false
  router.push('/profile/api-keys')
}

function handleLogout() {
  profileDropdownOpen.value = false
  clearAuth()
  router.push('/login')
}

function handleMobileMenuToggle() {
  toggleSidebar()
}
</script>

<template>
  <header class="header">
    <div class="header__container">
      <!-- Left side: Logo -->
      <div class="header__left">
        <button class="header__logo" @click="handleLogoClick">
          <img src="/favicon-96x96.png" alt="Lesstruct logo" class="header__logo-image" />
          <h1 class="header__logo-text">Lesstruct</h1>
        </button>
      </div>

      <!-- Right side: Hamburger menu, Theme toggle, and User profile -->
      <div class="header__right">
        <button
          class="header__menu-toggle"
          @click="handleMobileMenuToggle"
          :aria-label="sidebarCollapsed ? 'Open menu' : 'Close menu'"
          :aria-expanded="!sidebarCollapsed"
        >
          <IconMenu class="header__menu-icon" />
        </button>
        <ThemeToggle class="header__theme-toggle" />
        <div v-if="isAuthenticated" class="header__user" ref="profileDropdownRef">
          <button
            class="header__user-button"
            @click="toggleProfileDropdown"
            aria-label="User profile"
            :aria-expanded="profileDropdownOpen"
          >
            <img
              v-if="profilePicture"
              :src="profilePicture"
              alt=""
              class="header__user-avatar"
            />
            <IconUser v-else class="header__user-icon" />
          </button>
          <div v-if="profileDropdownOpen" class="header__dropdown">
            <button
              class="header__dropdown-item"
              @click="handleProfileClick"
            >
              <IconUser class="header__dropdown-icon" />
              <span>Profile</span>
            </button>
            <button
              class="header__dropdown-item"
              @click="handleApiKeysClick"
            >
              <IconKey class="header__dropdown-icon" />
              <span>API Keys</span>
            </button>
            <hr class="header__dropdown-divider" />
            <button
              class="header__dropdown-item header__dropdown-item--danger"
              @click="handleLogout"
            >
              <IconLogout class="header__dropdown-icon" />
              <span>Logout</span>
            </button>
          </div>
        </div>
        <button v-else class="header__login-btn" @click="() => router.push('/login')">
          Login
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 64px;
  background-color: var(--brand-light-1);
  border-bottom: 1px solid var(--brand-light-2);
  z-index: 50;
}

.header__container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
  padding: 0 1.5rem;
  max-width: 100%;
}

.header__left {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.header__logo {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.header__logo-image {
  width: 32px;
  height: 32px;
  object-fit: contain;
}

.header__logo-text {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--brand-dark-1);
}

.header__right {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.header__menu-toggle {
  display: none;
  background: none;
  border: none;
  padding: 0.5rem;
  cursor: pointer;
  color: var(--brand-dark-1);
  border-radius: 0.375rem;
  transition: background-color 0.2s;
  min-width: 44px;
  min-height: 44px;
}

.header__menu-toggle:hover {
  background-color: var(--brand-primary-light);
}

.header__menu-icon {
  width: 24px;
  height: 24px;
}

.header__user {
  position: relative;
  display: flex;
  align-items: center;
}

.header__user-button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.5rem;
  background: none;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  cursor: pointer;
  transition: background-color 0.2s;
  min-height: 44px;
  color: var(--brand-dark-1);
}

.header__user-button:hover {
  background-color: var(--brand-primary-light);
}

.header__user-icon {
  width: 20px;
  height: 20px;
}

.header__user-avatar {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  object-fit: cover;
}

.header__dropdown {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  min-width: 200px;
  background-color: var(--brand-light-1);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
  z-index: 100;
  padding: 0.5rem 0;
}

.header__dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  padding: 0.75rem 1rem;
  background: none;
  border: none;
  cursor: pointer;
  transition: background-color 0.2s;
  text-align: left;
  font-size: 0.875rem;
  color: var(--brand-dark-1);
}

.header__dropdown-item:hover {
  background-color: var(--brand-primary-light);
}

.header__dropdown-item--danger {
  color: var(--color-error);
}

.header__dropdown-item--danger:hover {
  background-color: rgba(239, 68, 68, 0.1);
}

.header__dropdown-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.header__dropdown-divider {
  margin: 0.5rem 0;
  border: none;
  border-top: 1px solid var(--brand-light-2);
}

.header__login-btn {
  padding: 0.5rem 1rem;
  background-color: var(--brand-primary);
  color: var(--brand-dark-1);
  border: none;
  border-radius: 0.375rem;
  cursor: pointer;
  font-weight: 500;
  transition: background-color 0.2s;
  min-height: 44px;
}

.header__login-btn:hover {
  background-color: var(--brand-primary-hover);
}

/* Tablet styles (768px - 1023px) */
@media (min-width: 768px) and (max-width: 1023px) {
  .header__menu-toggle {
    display: block;
  }
}

/* Mobile styles */
@media (max-width: 767px) {
  .header__container {
    padding: 0 1rem;
  }

  .header__logo-text {
    font-size: 1.125rem;
  }

  .header__right {
    gap: 0.5rem;
  }
}
</style>
