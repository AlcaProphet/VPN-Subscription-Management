// home-view.spec.ts：首页新卡片模型渲染（Build4 Step4：流量/分流规则/平台三态/管理员预览）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'

vi.mock('@/api/home', () => ({
  homePlatforms: vi.fn(),
  homeUpdatedAt: vi.fn(),
  refreshHomeToken: vi.fn(),
  getHomeSummary: vi.fn(),
}))
vi.mock('@/api/auth', () => ({
  me: vi.fn(),
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
}))
vi.mock('@/api/settings', () => ({
  siteInfoPublic: vi.fn().mockResolvedValue({ site_name: '', icon_url: '' }),
  getPublicAnnouncement: vi.fn().mockResolvedValue({ content: '' }),
}))
vi.mock('@/api/subscription', () => ({
  previewSubscriptionByPlatform: vi.fn(),
}))

import HomeView from '@/views/HomeView.vue'
import { homePlatforms, homeUpdatedAt, getHomeSummary } from '@/api/home'
import { me } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const mockHome = homePlatforms as unknown as ReturnType<typeof vi.fn>
const mockUpdated = homeUpdatedAt as unknown as ReturnType<typeof vi.fn>
const mockSummary = getHomeSummary as unknown as ReturnType<typeof vi.fn>
const mockMe = me as unknown as ReturnType<typeof vi.fn>

const readyCard = {
  platform_id: 1, slug: 'platform-x', name: 'Clash Verge', description: '桌面端', schemes: ['clash://{url}'],
  installer_files: [], installer_urls: [], status: 'ready', preview_available: false,
  download_token: 't1', download_url: '/subscriptions/platform-x/download?token=t1',
  subscription_name: '主力订阅', subscription_product_type: 'yaml', version_updated_at: '2026-08-07T10:00:00Z',
}
const customCard = {
  platform_id: 2, slug: 'platform-y', name: 'v2rayNG', description: 'Android', schemes: ['v2rayng://{url}'],
  installer_files: [], installer_urls: [], status: 'custom', preview_available: false,
  download_token: 't2', download_url: '/subscriptions/platform-y/download?token=t2', subscription_name: '自定义订阅',
}
const unassignedCard = {
  platform_id: 3, slug: 'platform-z', name: 'Shadowrocket', description: 'iOS', schemes: ['shadowrocket://{url}'],
  installer_files: [], installer_urls: [], status: 'unassigned', preview_available: false,
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
    mockSummary.mockReset()
    mockMe.mockReset()
    mockMe.mockResolvedValue({ id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    mockUpdated.mockResolvedValue({ updated_at: '2026-08-07T10:00:00Z' })
    mockSummary.mockResolvedValue({ traffic: { unlimited: true }, home_rule: null })
  })

  it('ready 卡片显示订阅名与操作按钮；流量卡与分流规则卡可见', async () => {
    mockHome.mockResolvedValue([readyCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('Clash Verge')
    expect(wrapper.text()).toContain('主力订阅')
    expect(wrapper.text()).toContain('一键导入')
    expect(wrapper.text()).toContain('刷新链接')
    expect(wrapper.text()).toContain('不限流量')
    expect(wrapper.text()).toContain('分流规则为 Shadowrocket 客户端专用')
    expect(wrapper.text()).toContain('使用指引：先添加订阅获取节点，再导入分流规则')
    const summary = wrapper.find('.home-summary-grid')
    expect(summary.exists()).toBe(true)
    expect(summary.classes()).toContain('grid-cols-1')
    expect(summary.classes()).toContain('md:grid-cols-2')
  })

  it('未分配卡片显示灰色占位且三按钮隐藏', async () => {
    mockHome.mockResolvedValue([unassignedCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'user', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('Shadowrocket')
    expect(wrapper.text()).toContain('暂无可用版本，请联系管理员')
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

  it('管理员：平台卡为预览形态，仅模板信息与预览按钮', async () => {
    const adminCard = {
      platform_id: 4, slug: 'platform-a', name: 'Clash Verge', description: '桌面端', schemes: ['clash://{url}'],
      installer_files: [], installer_urls: [], status: 'admin_preview', preview_available: true,
      subscription: {
        name: '订阅A', product_type: 'yaml', content_kind: 'upload', current_version: 1,
        version_updated_at: '2026-08-07T10:00:00Z',
      },
    }
    mockHome.mockResolvedValue([adminCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'admin', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('订阅A')
    expect(wrapper.text()).toContain('按平台预览当前版本')
    expect(wrapper.text()).toContain('直接上传')
    expect(wrapper.text()).not.toContain('一键导入')
    expect(wrapper.text()).not.toContain('复制链接')
    expect(wrapper.text()).not.toContain('刷新链接')
  })

  it('管理员：无订阅条目时显示待添加空态且预览按钮禁用', async () => {
    const emptyCard = {
      platform_id: 5, slug: 'platform-b', name: 'Shadowrocket', description: 'iOS', schemes: ['shadowrocket://{url}'],
      installer_files: [], installer_urls: [], status: 'admin_preview', preview_available: false, subscription: null,
    }
    mockHome.mockResolvedValue([emptyCard])
    const auth = useAuthStore()
    auth.setSession('tok', { id: 1, username: 'u1', email: 'u1@x.com', role: 'admin', group_id: 1, status: 'active', user_source: 'local' })
    const wrapper = mount(HomeView, { global: { plugins: [makeRouter()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无版本，等待添加')
    expect(wrapper.text()).not.toContain('暂无可用版本，请联系管理员')
    expect(wrapper.text()).toContain('按平台预览当前版本')
    expect(wrapper.text()).not.toContain('一键导入')
  })
})
