// admin-overview-view.spec.ts：概览只使用单一汇总请求，并按 checklist / 动态摘要渲染。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'

vi.mock('@/api/overview', () => ({ getAdminOverview: vi.fn() }))

import AdminOverviewView from '@/views/admin/AdminOverviewView.vue'
import { getAdminOverview } from '@/api/overview'

const overview = {
  status: { app_mode: 'prod', advanced_mode: true, emergency: false },
  counts: { platforms: 1, subscriptions: 2, nodes: 4, usable_nodes: 3, manual_nodes: 1, xray_nodes: 3, rules: 1, shares: 0, users: 7, pending_users: 1, pools: 2, proxy_groups: 1, xray_instances: 1, ext_accounts: 0 },
  checklist: [
    { key: 'platforms', done: true, label: '创建至少一个平台', action_path: '/admin/platforms', action_label: '创建平台' },
    { key: 'member_check', done: false, manual: true, label: '以普通用户身份检查', action_path: '/', action_label: '查看用户首页' },
  ],
  recent: {
    pending_users: [{ id: 1, username: 'pending-user', email: 'pending@example.com', source: 'selfreg', oidc_claims: '', created_at: '2026-08-28T12:00:00Z' }],
    access_logs: [{ id: 2, user_id: 1, username: 'alice', user_email: '', ip: '127.0.0.1', download_type: 'subscription', platform: '', platform_name: '', resource_slug: 'main', resource_name: '主订阅', status: 'success', fail_reason: '', created_at: '2026-08-28T12:05:00Z' }],
  },
}

function router() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: AdminOverviewView }, { path: '/admin/:pathMatch(.*)*', component: { template: '<div />' } }] })
}

describe('AdminOverviewView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(getAdminOverview as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(overview)
  })

  it('仅拉取概览汇总，并渲染状态、清单和两类动态摘要', async () => {
    const wrapper = mount(AdminOverviewView, { global: { plugins: [router()] } })
    await flushPromises()
    expect(getAdminOverview).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Production')
    expect(wrapper.text()).toContain('以普通用户身份检查')
    expect(wrapper.text()).toContain('pending-user')
    expect(wrapper.text()).toContain('主订阅')
  })

  it('请求失败时显示重试入口', async () => {
    ;(getAdminOverview as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('概览加载失败'))
    const wrapper = mount(AdminOverviewView, { global: { plugins: [router()] } })
    await flushPromises()
    expect(wrapper.text()).toContain('概览加载失败')
    expect(wrapper.text().replace(/\s/g, '')).toContain('重试')
  })
})
