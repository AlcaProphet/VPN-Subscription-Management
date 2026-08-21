import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/xray', () => ({
  listInstances: vi.fn().mockResolvedValue([]),
  listExtAccounts: vi.fn().mockResolvedValue([]),
  createInstance: vi.fn(),
  updateInstance: vi.fn(),
  deleteInstance: vi.fn(),
  detectNodes: vi.fn(),
  testConnection: vi.fn(),
  runInit: vi.fn(),
  reconcile: vi.fn(),
  pushRepair: vi.fn(),
  cleanOrphans: vi.fn(),
  repairCredentials: vi.fn(),
  createExtAccount: vi.fn(),
  updateExtAccount: vi.fn(),
  deleteExtAccount: vi.fn(),
  retryExtSync: vi.fn(),
  resetExtQuota: vi.fn(),
  getExtCredentials: vi.fn(),
  pushOne: vi.fn(),
  repairCredentialsOne: vi.fn(),
}))
vi.mock('@/api/node', () => ({
  listNodes: vi.fn().mockResolvedValue([]),
  setNodeDisplayName: vi.fn(),
}))
vi.mock('@/api/settings', () => ({
  getAdminTask: vi.fn(),
}))

import XrayInstancesView from '@/views/admin/XrayInstancesView.vue'
import { listInstances, listExtAccounts } from '@/api/xray'

describe('XrayInstancesView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
    ;(listExtAccounts as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
  })

  it('空态展示新增实例入口', async () => {
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    expect(wrapper.text()).toContain('还没有 Xray 实例')
    expect(wrapper.text()).toContain('新增实例')
  })

  it('实例列表展示实例名称与 slug', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    expect(wrapper.text()).toContain('smoke-instance')
    expect(wrapper.text()).toContain('instance-smoke')
  })
})
