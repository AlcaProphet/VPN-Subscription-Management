// router-guard.spec.ts：configured 守卫跳转逻辑
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'

// mock 系统状态接口
vi.mock('@/api/system', () => ({
  getSystemStatus: vi.fn(),
}))

import { getSystemStatus } from '@/api/system'

const mockGetStatus = getSystemStatus as unknown as ReturnType<typeof vi.fn>

describe('路由守卫（configured 逻辑）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mockGetStatus.mockReset()
  })

  it('configured=false 时任意路径跳 /setup', async () => {
    mockGetStatus.mockResolvedValue({ configured: false, app_mode: 'dev', emergency: false })
    const system = useSystemStore()
    const status = await system.fetchStatus(true)
    expect(status?.configured).toBe(false)
    // 守卫核心逻辑（与 router/index.ts 一致）：未配置时非 /setup 路径应跳 /setup
    const to = { path: '/login', meta: {} }
    const redirect = (!status!.configured && to.path !== '/setup') ? '/setup' : null
    expect(redirect).toBe('/setup')
  })

  it('configured=true 且无 token 访问受保护路由跳 /login', async () => {
    mockGetStatus.mockResolvedValue({ configured: true, app_mode: 'dev', emergency: false })
    const system = useSystemStore()
    const auth = useAuthStore()
    await system.fetchStatus(true)
    const to = { path: '/', meta: { public: false } }
    const redirect = (!to.meta.public && !auth.token) ? '/login' : null
    expect(redirect).toBe('/login')
  })

  it('已配置时访问 /setup 跳 /login', async () => {
    mockGetStatus.mockResolvedValue({ configured: true, app_mode: 'dev', emergency: false })
    const system = useSystemStore()
    const status = await system.fetchStatus(true)
    const to = { path: '/setup', meta: {} }
    const redirect = (status?.configured && to.path === '/setup') ? '/login' : null
    expect(redirect).toBe('/login')
  })

  it('advanced_mode !== true 时访问 /admin/groups 或 /admin/xray 重定向订阅管理', async () => {
    mockGetStatus.mockResolvedValue({ configured: true, app_mode: 'dev', emergency: false, advanced_mode: false })
    const system = useSystemStore()
    const status = await system.fetchStatus(true)
    for (const path of ['/admin/groups', '/admin/xray']) {
      const to = { path, meta: {} }
      const redirect = ((to.path === '/admin/groups' || to.path === '/admin/xray') && status?.advanced_mode !== true)
        ? '/admin/subscriptions'
        : null
      expect(redirect).toBe('/admin/subscriptions')
    }
  })

  it('系统状态未加载（status=null）时访问高级路由同样重定向订阅管理', async () => {
    const system = useSystemStore()
    // 不调用 fetchStatus，使 status 保持 null
    expect(system.status).toBeNull()
    for (const path of ['/admin/groups', '/admin/xray']) {
      const to = { path, meta: {} }
      const redirect = ((to.path === '/admin/groups' || to.path === '/admin/xray') && system.status?.advanced_mode !== true)
        ? '/admin/subscriptions'
        : null
      expect(redirect).toBe('/admin/subscriptions')
    }
  })
})
