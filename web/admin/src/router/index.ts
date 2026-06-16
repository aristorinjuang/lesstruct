import { createRouter, createWebHistory } from 'vue-router'
import { getAuthStatus } from '@/composables/useAuth'
import request from '@/utils/request'
import FullLayout from '@/layouts/FullLayout.vue'
import NarrowLayout from '@/layouts/NarrowLayout.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      redirect: '/content?type=post',
    },
    {
      path: '/',
      component: FullLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('../views/DashboardView.vue'),
          meta: { title: 'Dashboard' },
        },
        {
          path: 'content',
          name: 'content-list',
          component: () => import('../views/ContentListView.vue'),
          meta: { title: 'Content' },
        },
        {
          path: 'content/create',
          name: 'content-create',
          component: () => import('../views/ContentCreateView.vue'),
          meta: { title: 'Create Content' },
        },
        {
          path: 'content/:id/edit',
          name: 'content-edit',
          component: () => import('../views/ContentListView.vue'),
          meta: { title: 'Edit Content' },
        },
        {
          path: 'content/:id/comments',
          name: 'content-comments',
          component: () => import('../views/CommentsView.vue'),
          meta: { title: 'Comments' },
        },
        {
          path: 'media',
          name: 'media',
          component: () => import('../views/MediaView.vue'),
          meta: { title: 'Media' },
        },
        {
          path: 'comment',
          name: 'comment',
          component: () => import('../views/MyCommentsView.vue'),
          meta: { title: 'Comments' },
        },
        {
          path: 'users',
          name: 'admin-users',
          component: () => import('../views/UserManagementView.vue'),
          meta: { title: 'User Management' },
        },
        {
          path: 'import',
          name: 'import',
          component: () => import('../views/ImportView.vue'),
          meta: { title: 'Import' },
        },
        {
          path: 'import/wordpress',
          name: 'import-wordpress',
          component: () => import('../views/WordPressImportView.vue'),
          meta: { title: 'Import from WordPress' },
        },
      ],
    },
    {
      path: '/',
      component: NarrowLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: 'profile',
          name: 'profile',
          component: () => import('../views/ProfileView.vue'),
          meta: { title: 'Profile' },
        },
        {
          path: 'profile/api-keys',
          name: 'profile-api-keys',
          component: () => import('../views/ApiKeys.vue'),
          meta: { title: 'API Keys' },
        },
      ],
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { title: 'Login' },
    },
    {
      path: '/forgot-password',
      name: 'forgot-password',
      component: () => import('../views/ForgotPasswordView.vue'),
      meta: { title: 'Forgot Password' },
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: () => import('../views/ResetPasswordView.vue'),
      meta: { title: 'Reset Password' },
    },
    {
      path: '/first-login',
      name: 'first-login',
      component: () => import('../views/FirstLoginSetupView.vue'),
      meta: { title: 'First Login Setup' },
    },
  ],
})

const FIRST_LOGIN_CACHE_TTL = 30_000

async function checkFirstLoginStatus(): Promise<boolean | null> {
  const cached = sessionStorage.getItem('first_login_complete')
  if (cached === 'true') return true
  if (cached === 'false') return false

  const cachedTs = sessionStorage.getItem('first_login_complete_ts')
  if (cachedTs && Date.now() - Number(cachedTs) < FIRST_LOGIN_CACHE_TTL) {
    return null
  }

  try {
    const response = await request.get<{ data: { firstLoginComplete: boolean } }>(
      '/api/auth/first-login'
    )
    const complete = response.data.data.firstLoginComplete
    sessionStorage.setItem('first_login_complete', String(complete))
    sessionStorage.removeItem('first_login_complete_ts')
    return complete
  } catch {
    sessionStorage.setItem('first_login_complete_ts', String(Date.now()))
    return null
  }
}

router.beforeEach(async (to, from, next) => {
  const isAuthenticated = getAuthStatus()

  // Redirect to login if authentication is required but user is not authenticated
  if (to.meta.requiresAuth && !isAuthenticated) {
    next('/login')
    return
  }

  // If authenticated and on a protected route, check if first-login setup is needed
  if (isAuthenticated && to.meta.requiresAuth) {
    const setupComplete = await checkFirstLoginStatus()
    if (setupComplete === false && to.name !== 'first-login') {
      next('/first-login')
      return
    }
  }

  // Redirect away from first-login page if setup is already complete
  if (to.name === 'first-login') {
    if (!isAuthenticated) {
      next('/login')
      return
    }
    const setupComplete = await checkFirstLoginStatus()
    if (setupComplete === true) {
      next('/dashboard')
      return
    }
  }

  // Redirect to dashboard if already authenticated and trying to access login page
  if (to.name === 'login' && isAuthenticated) {
    next('/dashboard')
    return
  }

  next()
})

export default router
