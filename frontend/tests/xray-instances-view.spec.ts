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
import { listInstances, listExtAccounts, detectNodes, reconcile, testConnection, retryExtSync, updateInstance } from '@/api/xray'

describe('XrayInstancesView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.body.innerHTML = ''
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

  it('实例开关失败回滚', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(updateInstance as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('保存失败'))
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const sw = wrapper.find('.ant-switch')
    expect(sw.classes()).toContain('ant-switch-checked')
    await sw.trigger('click')
    await flushPromises()
    expect(updateInstance).toHaveBeenCalled()
    expect(wrapper.find('.ant-switch').classes()).toContain('ant-switch-checked')
  })

  it('检测无变化时不弹回执', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(detectNodes as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      added: 0, updated: 0, missing: 0, skipped: [], added_nodes: [],
    })
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const refresh = wrapper.findAll('button').find((b) => b.text().includes('刷新节点'))
    await refresh?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('刷新节点结果')
  })

  it('对账四分区全空显示成功态', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(reconcile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      to_push: [], orphans: [], ext_orphans: [], credential_mismatches: [], to_remove: [],
    })
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const reconcileBtn = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('对账'))
    expect(reconcileBtn).toBeTruthy()
    await reconcileBtn!.trigger('click')
    await flushPromises()
    expect(document.body.textContent ?? '').toContain('账号已一致，无需处理')
  })

  it('测试连接成功展示结果', async () => {
    ;(testConnection as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(undefined)
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('新增实例'))
    await addBtn?.trigger('click')
    await flushPromises()
    const addrInput = document.querySelector('input[placeholder="api_addr (host:port)"]') as HTMLInputElement
    addrInput.value = '127.0.0.1:10086'
    addrInput.dispatchEvent(new Event('input', { bubbles: true }))
    const testBtn = Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.includes('测试连接')) as HTMLButtonElement
    testBtn.click()
    await flushPromises()
    expect(document.body.textContent ?? '').toContain('连接成功')
  })

  it('检测有变化时展示回执', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(detectNodes as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      added: 1, updated: 1, missing: 0, skipped: [], added_nodes: [{ node_id: 1, tag: 'inbound-1', name: 'node-1' }],
    })
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const refresh = wrapper.findAll('button').find((b) => b.text().includes('刷新节点'))
    await refresh?.trigger('click')
    await flushPromises()
    const body = document.body.textContent ?? ''
    expect(body).toContain('刷新节点结果')
    expect(body).toContain('新增 1')
    expect(body).toContain('更新 1')
  })

  it('对账四分区非空时渲染各分区', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(reconcile as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      to_push: [{ email: 'user-1@vpn.local', inbound_tag: 'tag', source: 'user' }],
      orphans: [{ email: 'user-2@vpn.local', inbound_tag: 'tag' }],
      ext_orphans: [{ email: 'ext-9@vpn.local', inbound_tag: 'tag' }],
      credential_mismatches: [{ email: 'user-3@vpn.local', inbound_tag: 'tag' }],
      to_remove: [],
    })
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const reconcileBtn = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('对账'))
    await reconcileBtn?.trigger('click')
    await flushPromises()
    const body = document.body.textContent ?? ''
    expect(body).toContain('待补推 1')
    expect(body).toContain('无头 1')
    expect(body).toContain('疑似独立账号残留')
    expect(body).toContain('凭据不一致 1')
  })

  it('独立账号创建弹窗展示双轨模式', async () => {
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const tab = wrapper.findAll('.ant-tabs-tab').find((t) => t.text().includes('独立账号'))
    await tab?.trigger('click')
    await flushPromises()
    const createBtn = wrapper.findAll('button').find((b) => b.text().includes('创建独立账号'))
    await createBtn?.trigger('click')
    await flushPromises()
    const body = document.body.textContent ?? ''
    expect(body).toContain('自动生成')
    expect(body).toContain('推送目标')
  })

  it('独立账号重试调用接口', async () => {
    ;(listInstances as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'smoke-instance', slug: 'instance-smoke', api_addr: '127.0.0.1:10086', api_tag: 'tag', enabled: true },
    ])
    ;(listExtAccounts as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: 'ext1', email: 'ext-1@vpn.local', push_targets: [] },
    ])
    ;(retryExtSync as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ added: 1, add_failed: 0, removed: 0, remove_failed: 0 })
    const wrapper = mount(XrayInstancesView)
    await flushPromises()
    const tab = wrapper.findAll('.ant-tabs-tab').find((t) => t.text().includes('独立账号'))
    await tab?.trigger('click')
    await flushPromises()
    const retryBtn = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('重试'))
    await retryBtn?.trigger('click')
    await flushPromises()
    expect(retryExtSync).toHaveBeenCalledWith(1)
  })
})
