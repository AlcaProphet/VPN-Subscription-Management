// version-manage-view.spec.ts：版本页以版本 ID 获取真实归属标题，空列表保留中文回退。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'

vi.mock('@/api/version', () => ({
  versionApi: vi.fn(() => ({ list: vi.fn(), create: vi.fn(), switchCurrent: vi.fn(), preview: vi.fn(), remove: vi.fn() })),
  getVersionBlueprint: vi.fn(),
  getVersionOwner: vi.fn(),
}))
vi.mock('@/api/subscription', () => ({ getSubscription: vi.fn() }))

import VersionManageView from '@/views/admin/VersionManageView.vue'
import { versionApi, getVersionOwner } from '@/api/version'

const list = vi.fn()

function router() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: VersionManageView }, { path: '/admin/:pathMatch(.*)*', component: { template: '<div />' } }] })
}

describe('VersionManageView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(versionApi as unknown as ReturnType<typeof vi.fn>).mockReturnValue({ list, create: vi.fn(), switchCurrent: vi.fn(), preview: vi.fn(), remove: vi.fn() })
  })

  it('有版本时展示真实名称和中文资源类型', async () => {
    list.mockResolvedValue([{ id: 20, version_no: 1, current: true, created_at: '2026-08-28T00:00:00Z', updated_at: '2026-08-28T00:00:00Z' }])
    ;(getVersionOwner as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ owner_type: 'subscription', owner_id: 3, name: '主力订阅', type_label: '订阅', back_path: '/admin/subscriptions' })
    const wrapper = mount(VersionManageView, { props: { ownerType: 'subscription', ownerId: 3, apiPrefix: '/admin/subscriptions', backPath: '/admin/subscriptions' }, global: { plugins: [router()] } })
    await flushPromises()
    expect(getVersionOwner).toHaveBeenCalledWith(20)
    expect(wrapper.find('h1').text()).toBe('主力订阅 · 版本管理')
    expect(wrapper.text()).toContain('订阅')
  })

  it('无版本时不请求归属接口并显示中文回退', async () => {
    list.mockResolvedValue([])
    const wrapper = mount(VersionManageView, { props: { ownerType: 'rule', ownerId: 9, apiPrefix: '/admin/rules' }, global: { plugins: [router()] } })
    await flushPromises()
    expect(getVersionOwner).not.toHaveBeenCalled()
    expect(wrapper.find('h1').text()).toBe('规则 #9 · 版本管理')
  })
})
