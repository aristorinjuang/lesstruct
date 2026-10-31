import { createRouter, createWebHistory } from 'vue-router'
import { refreshSession, ADMIN_BOOT_ROLES, homePathFor } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
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
          component: () => import('../views/ContentEditView.vue'),
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
          path: 'orders',
          name: 'admin-orders',
          component: () => import('../views/OrdersView.vue'),
          meta: { title: 'Orders' },
        },
        {
          path: 'orders/:id',
          name: 'admin-order-detail',
          component: () => import('../views/OrderDetailView.vue'),
          meta: { title: 'Order Detail' },
        },
        {
          path: 'payments',
          name: 'admin-payments',
          component: () => import('../views/PaymentsView.vue'),
          meta: { title: 'Payments' },
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
        {
          path: 'import/hugo',
          name: 'import-hugo',
          component: () => import('../views/HugoImportView.vue'),
          meta: { title: 'Import from Hugo' },
        },
        {
          path: 'export',
          name: 'export',
          component: () => import('../views/ExportView.vue'),
          meta: { title: 'Export' },
        },
        {
          path: 'settings/invoice',
          name: 'settings-invoice',
          component: () => import('../views/InvoiceSettingsView.vue'),
          meta: { title: 'Invoice Settings' },
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
    {
      // Unknown sub-routes (e.g. /admin/content/2 — the edit route is
      // /admin/content/6/edit) land on the dashboard instead of a blank
      // shell. The guard below still applies to the target, so visitors
      // without a session end up at login as usual.
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      redirect: '/dashboard',
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
  // The session lives in the HttpOnly cookie: probe the server identity
  // (cached after the first navigation) instead of reading a token.
  const sessionUser = await refreshSession()
  const isAuthenticated = sessionUser !== null

  // A storefront session shares the same cookie but must not boot the admin
  // shell: bounce to the login view with a message instead of a broken
  // dashboard.
  if (isAuthenticated && to.meta.requiresAuth) {
    const role = sessionUser?.role ?? null
    if (role && !ADMIN_BOOT_ROLES.has(role)) {
      next({ path: '/login', query: { reason: 'storefront-session' } })
      return
    }
  }

  // Role-aware home: `/` and unknown URLs redirect statically (route config
  // cannot see the session), so resolve the landing page here from the
  // session role instead of stranding non-admin roles on the dashboard or
  // the content list they cannot use.
  if (isAuthenticated) {
    const home = homePathFor(sessionUser?.role ?? null)
    if (
      (to.redirectedFrom?.path === '/' || (to.redirectedFrom && to.path === '/dashboard')) &&
      to.fullPath !== home
    ) {
      next(home)
      return
    }
  }

  // Comment surfaces (my comments, per-content comments) require the comment
  // system; when disabled, bounce to the content list.
  if (to.name === 'comment' || to.name === 'content-comments') {
    const { commentsEnabled, fetchConfig } = useConfig()
    await fetchConfig()
    if (!commentsEnabled.value) {
      next('/content?type=post')
      return
    }
  }

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

  // First-login setup is a bootstrap page: the seeded admin has no email
  // address yet, so it opens without a session. Once setup is complete it
  // requires authentication like every other admin page.
  if (to.name === 'first-login') {
    const setupComplete = await checkFirstLoginStatus()
    if (setupComplete === true) {
      next(isAuthenticated ? '/dashboard' : '/login')
      return
    }
  }

  // Redirect to dashboard if already authenticated and trying to access login page.
  // The storefront-session bounce landing is exempt: its navigation arrives
  // authenticated (the stale token is still in storage) and must reach the
  // login view so it can clear the stale session — otherwise the bounce
  // loops back to the dashboard forever.
  if (to.name === 'login' && isAuthenticated && to.query.reason !== 'storefront-session') {
    next(homePathFor(sessionUser?.role ?? null))
    return
  }

  next()
})

export default router
