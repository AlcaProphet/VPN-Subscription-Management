// 路由表与守卫：emergency → configured → 登录态 → 登录页跳过（UI §7.2）
import { createRouter, createWebHistory } from 'vue-router'
import { message } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { me } from '@/api/auth'

// 版本管理子路由：四条均复用 VersionManageView，按 ownerType 传参（UI §7.1）；
// prefix 为相对路径（axios baseURL=/api），禁止带 /api 前缀（否则拼成 /api/api/ 被 SPA 回退吞掉）
const versionRoutes = [
  { path: '/admin/subscriptions/:id/versions', ownerType: 'subscription', prefix: '/admin/subscriptions', backPath: '/admin/subscriptions' },
  { path: '/admin/shares/:id/versions', ownerType: 'share', prefix: '/admin/shares', backPath: '/admin/shares' },
  { path: '/admin/rules/:id/versions', ownerType: 'rule', prefix: '/admin/rules', backPath: '/admin/rules' },
  { path: '/admin/customs/:id/versions', ownerType: 'custom', prefix: '/admin/customs', backPath: '/admin/users' }, // R04-01：Build3 用户管理接通后返回用户列表
].map((r) => ({
  path: r.path,
  component: () => import('@/views/admin/VersionManageView.vue'),
  props: (route: any) => ({
    ownerType: r.ownerType,
    ownerId: Number(route.params.id),
    apiPrefix: r.prefix,
    resourceName: r.ownerType,
    backPath: r.backPath,
  }),
  meta: { layout: 'admin', requiresAdmin: true },
}))

// 管理路由（懒加载；路由级代码分割）
const adminRoutes = [
  { path: '/admin', component: () => import('@/views/admin/AdminOverviewView.vue') },
  { path: '/admin/subscriptions', component: () => import('@/views/admin/SubscriptionsView.vue') },
  { path: '/admin/groups', component: () => import('@/views/admin/GroupsView.vue') },
  { path: '/admin/shares', component: () => import('@/views/admin/SharesView.vue') },
  { path: '/admin/platforms', component: () => import('@/views/admin/PlatformsView.vue') },
  { path: '/admin/platforms/:id/edit', component: () => import('@/views/admin/PlatformEditView.vue') },
  { path: '/admin/platforms/new', component: () => import('@/views/admin/PlatformEditView.vue') },
  { path: '/admin/rules', component: () => import('@/views/admin/RulesView.vue') },
  { path: '/admin/assembly', component: () => import('@/views/admin/AssemblyView.vue') },
  { path: '/admin/nodes', component: () => import('@/views/admin/NodesView.vue') },
  { path: '/admin/xray', component: () => import('@/views/admin/XrayInstancesView.vue') },
  // Build3 Step 1：用户管理
  { path: '/admin/users', component: () => import('@/views/admin/UsersView.vue') },
  // Build3 Step 2：审批中心
  { path: '/admin/approvals', component: () => import('@/views/admin/ApprovalsView.vue') },
  // Build3 Step 3：面板配置
  { path: '/admin/settings', component: () => import('@/views/admin/SettingsView.vue') },
  // Build3 Step 5：日志查看
  { path: '/admin/logs', component: () => import('@/views/admin/LogsView.vue') },
].map((r) => ({ ...r, meta: { layout: 'admin', requiresAdmin: true } }))

// 用户端路由（懒加载）
const userRoutes = [
  { path: '/', component: () => import('@/views/HomeView.vue'), meta: { layout: 'home' } },
  { path: '/rules', component: () => import('@/views/RulesView.vue'), meta: { layout: 'home' } },
  { path: '/profile', component: () => import('@/views/ProfileView.vue'), meta: { layout: 'home' } },
]

const routes = [
  // 公开（blank 布局）
  { path: '/emergency', component: () => import('@/views/EmergencyView.vue'), meta: { layout: 'blank', public: true } }, // Build3 Step 6：应急恢复页（守卫 emergency=true 强制跳转）
  { path: '/setup', component: () => import('@/views/SetupView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/register', component: () => import('@/views/RegisterView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/forgot', component: () => import('@/views/ForgotView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/reset', component: () => import('@/views/ResetView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/pending', component: () => import('@/views/PendingView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/login/callback', component: () => import('@/views/OidcCallbackView.vue'), meta: { layout: 'blank', public: true } },
  { path: '/:pathMatch(.*)*', component: () => import('@/views/NotFoundView.vue'), meta: { layout: 'blank', public: true } },
  // 用户端
  ...userRoutes,
  // 管理端（含版本子路由）
  ...adminRoutes,
  ...versionRoutes,
]

const router = createRouter({ history: createWebHistory(), routes })

// 顶部进度条：轻量自实现（2px 主色蓝固定定位条；禁止引入 NProgress 库，Build1 约束 4）
let barEl: HTMLElement | null = null

function progressStart() {
  if (barEl) return
  barEl = document.createElement('div')
  barEl.style.cssText = 'position:fixed;top:0;left:0;height:2px;background:var(--ui-primary);z-index:9999;transition:width .2s ease;width:10%'
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

// 路由守卫（UI §7.2）：emergency → configured → 登录态 → 管理员，顺序执行
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
  // 5) 角色守卫：/admin/** 实时校验管理员（与后端中间件双保险，后端为准）
  if (to.meta.requiresAdmin) {
    if (!auth.user) {
      try {
        auth.user = await me()
      } catch {
        return '/login'
      }
    }
    if (auth.user.role !== 'admin') return '/' // 非管理员访问管理路由 → 回首页
  }
  // 6) 高级模式路由守卫：advanced_mode 非 true 时（含状态未加载）视为 off 并重定向
  if ((to.path === '/admin/groups' || to.path === '/admin/xray') && status?.advanced_mode !== true) {
    message.warning('高级功能未开启，请在面板配置中开启高级模式')
    return '/admin/subscriptions'
  }
  return true
})
router.afterEach(() => progressDone())

export default router
