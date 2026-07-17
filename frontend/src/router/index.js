import { createRouter, createWebHistory, useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

// Inline component for /auth/callback — extracts JWT from URL query,
// stores it in localStorage and Pinia, then redirects.
const AuthCallbackComponent = {
  setup() {
    const route = useRoute()
    const router = useRouter()
    const userStore = useUserStore()

    const token = route.query.token
    if (token) {
      userStore.login(token)
      router.replace('/')
    } else {
      router.replace('/login')
    }
    return () => null
  },
  template: '<div></div>'
}

const routes = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/Home.vue')
  },
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/Setup.vue')
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue')
  },
  {
    path: '/auth/callback',
    name: 'AuthCallback',
    component: AuthCallbackComponent
  },
  {
    path: '/rules',
    name: 'Rules',
    component: () => import('@/views/Rules.vue')
  },
  {
    path: '/admin',
    component: () => import('@/views/Manage.vue'),
    children: [
      { path: '', redirect: '/admin/subscriptions' },
      { path: 'subscriptions', name: 'SubList', component: () => import('@/views/SubList.vue') },
      { path: 'subscriptions/:id/versions', name: 'SubVersions', component: () => import('@/views/SubVersions.vue') },
      { path: 'shares', name: 'ShareList', component: () => import('@/views/ShareList.vue') },
      { path: 'shares/:id/versions', name: 'ShareVersions', component: () => import('@/views/ShareVersions.vue') },
      { path: 'platforms', name: 'PlatformManage', component: () => import('@/views/PlatformManage.vue') },
      { path: 'users', name: 'UserManage', component: () => import('@/views/UserManage.vue') },
      { path: 'rules', name: 'RulesManage', component: () => import('@/views/RulesManage.vue') },
      { path: 'rules/:id/versions', name: 'RuleVersions', component: () => import('@/views/RuleVersions.vue') },
      { path: 'oidc', name: 'OIDCConfig', component: () => import('@/views/OIDCConfig.vue') },
      { path: 'logs', name: 'Logs', component: () => import('@/views/Logs.vue') }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// ============================================================================
// Navigation Guard (triple guard)
// ============================================================================
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()

  // 1. /auth/callback — always pass through
  if (to.path === '/auth/callback') {
    return next()
  }

  // 2. Check system status (cached after first call)
  await userStore.checkSystemStatus()

  // 3. System not configured → must go to /setup
  if (userStore.isConfigured === false) {
    if (to.path !== '/setup') {
      return next('/setup')
    }
    return next()
  }

  // 4. System configured but on /setup → redirect to /
  if (userStore.isConfigured === true && to.path === '/setup') {
    return next('/')
  }

  // 5. Restore token from localStorage and fetch user
  if (!userStore.user && userStore.token) {
    await userStore.fetchUser()
  }

  // 6. Not logged in → /login (except /login itself)
  if (!userStore.isLoggedIn) {
    if (to.path !== '/login') {
      return next('/login')
    }
    return next()
  }

  // 7. Already logged in but on /login → redirect /
  if (to.path === '/login') {
    return next('/')
  }

  // 8. /admin/* but not admin → redirect /
  if (to.path.startsWith('/admin') && !userStore.isAdmin) {
    return next('/')
  }

  next()
})

export default router
