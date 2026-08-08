// home-view.spec.ts：首页平台卡片三态渲染（Build2 Step 6 验收：已分配/未分配/自定义）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'

vi.mock('@/api/home', () => ({
  homePlatforms: vi.fn(),
  homeUpdatedAt: vi.fn(),
  refreshHomeToken: vi.fn(),
}))
vi.mock('@/api/auth', () => ({
  me: vi.fn(),
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
}))
vi.mock('@/api/settings', () => ({
  siteInfoPublic: vi.fn().mockResolvedValue({ site_name: '', icon_url: '' }),
}))

import HomeView from '@/views/HomeView.vue'
import { homePlatforms, homeUpdatedAt } from '@/api/home'
import { me } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const mockHome = homePlatforms as unknown as ReturnType<typeof vi.fn>
const mockUpdated = homeUpdatedAt as unknown as ReturnType<typeof vi.fn>
const mockMe = me as unknown as ReturnType<typeof vi.fn>

// 三态卡片样例
const groupCard = {
  platform_id: 1, name: 'Clash Verge', description: '桌面端', schemes: ['clash://{url}'],
  installer_file_url: '', installer_url: '', status: 'group_selected',
  download_token: 't1', download_url: '/subscriptions/p1/download?token=t1', subscription_name: '主力订阅',
}
const customCard = {
  platform_id: 2, name: 'v2rayNG', description: 'Android', schemes: ['v2rayng://{url}'],
  installer_file_url: '', installer_url: '', status: 'custom',
  download_token: 't2', download_url: '/subscriptions/p2/download?token=t2', subscription_name: '自定义订阅',
}
const unassignedCard = {
  platform_id: 3, name: 'Shadowrocket', description: 'iOS', schemes: ['shadowrocket://{url}'],
  installer_file_url: '', installer_url: '', status: 'unassigned',
  download_token: 't3', download_url: '/subscriptions/p3/download?token=t3',
}

function makeRouter() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: HomeView }] })
}

describe('首页平台卡片三态渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mockHome.mockReset()
    mockUpdated.mockReset()
    mockMe.mockReset()
    mockMe.mockResolvedValue({ id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    mockUpdated.mockResolvedValue({ updated_at: '2026-08-07T10:00:00Z' })
  })

  it('已分配（组选定）卡片显示订阅名与操作按钮', async () => {
    mockHome.mockResolvedValue([groupCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('Clash Verge')
    expect(wrapper.text()).toContain('主力订阅')
    expect(wrapper.text()).toContain('一键导入')
    expect(wrapper.text()).toContain('刷新链接')
    expect(wrapper.text()).not.toContain('未分配，请联系管理员')
  })

  it('未分配卡片显示灰色占位且三按钮隐藏', async () => {
    mockHome.mockResolvedValue([unassignedCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('Shadowrocket')
    expect(wrapper.text()).toContain('未分配，请联系管理员')
    expect(wrapper.text()).not.toContain('一键导入')
    expect(wrapper.text()).not.toContain('复制链接')
    expect(wrapper.text()).not.toContain('刷新链接')
  })

  it('自定义卡片显示覆盖提示 alert', async () => {
    mockHome.mockResolvedValue([customCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('v2rayNG')
    expect(wrapper.text()).toContain('已被分配自定义订阅')
    expect(wrapper.text()).toContain('一键导入')
  })
})
