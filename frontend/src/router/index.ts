// 路由表与守卫：emergency → configured → 登录态 → 登录页跳过（UI §7.2）
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'

// 路由表骨架（本 Build 范围）：全部路由级懒加载（代码分割）
const routes = [
  { path: '/setup', component: () => import('@/views/SetupView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/register', component: () => import('@/views/RegisterView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/forgot', component: () => import('@/views/ForgotView.vue'), meta: { layout: 'blank', public: true } }, // Step 7 建视图
  { path: '/reset/:token', component: () => import('@/views/ResetView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/pending', component: () => import('@/views/PendingView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/login/callback', component: () => import('@/views/OidcCallbackView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/', component: () => import('@/views/HomeView.vue'), meta: { layout: 'home' } },
  { path: '/:pathMatch(.*)*', component: () => import('@/views/NotFoundView.vue'), meta: { layout: 'blank', public: true } },
]

const router = createRouter({ history: createWebHistory(), routes })

// 顶部进度条：轻量自实现（2px 主色蓝固定定位条；禁止引入 NProgress 库，Build1 约束 4）
let barEl: HTMLElement | null = null

function progressStart() {
  if (barEl) return
  barEl = document.createElement('div')
  barEl.style.cssText = 'position:fixed;top:0;left:0;height:2px;background:#1677FF;z-index:9999;transition:width .2s ease;width:10%'
  document.body.appendChild(barEl)
  requestAnimationFrame(() => {
    if (barEl) barEl.style.width = '70%'
  })
}

function progressDone() {
  if (!barEl) return
  barEl.style.width = '100%'
  setTimeout(() => {
    barEl?.remove()
    barEl = null
  }, 200)
}

// 路由守卫（UI §7.2）：emergency → configured → 登录态，顺序执行
router.beforeEach(async (to) => {
  progressStart()
  const system = useSystemStore()
  const auth = useAuthStore()
  let status = system.status
  try {
    status = await system.fetchStatus() // 守卫内调 /api/system/status
  } catch {
    // 状态获取失败不阻断：仅依赖本地凭据判断，避免死循环
  }
  // 1) emergency：为 true 强制跳 /emergency（本 Build 恒 false，结构预留；白名单：/emergency 自身）
  if (status?.emergency && to.path !== '/emergency') return '/emergency'
  // 2) configured：未配置时任意路径跳 /setup；已配置时访问 /setup 跳 /login
  if (status && !status.configured && to.path !== '/setup') return '/setup'
  if (status?.configured && to.path === '/setup') return '/login'
  // 3) 登录态：无凭据访问受保护路由跳 /login
  if (!to.meta.public && !auth.token) return '/login'
  // 4) 登录页跳过：已登录访问 /login 跳 /
  if (to.path === '/login' && auth.token) return '/'
  return true
})
router.afterEach(() => progressDone())

export default router
