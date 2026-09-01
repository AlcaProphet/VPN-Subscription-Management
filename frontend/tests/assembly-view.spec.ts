// assembly-view.spec.ts：装配页核心交互（R14-15 Build5 Step6 item 7）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'

vi.mock('@/api/assembly', () => ({
  getAssemblyContext: vi.fn(),
  previewAssembly: vi.fn(),
  generateAssembly: vi.fn(),
  getBlueprint: vi.fn(),
}))

vi.mock('@/api/pool', () => ({
  listPools: vi.fn().mockResolvedValue([]),
  createPool: vi.fn(),
  updatePool: vi.fn(),
  deletePool: vi.fn(),
  submitSync: vi.fn(),
  getSyncStatus: vi.fn(),
  listEntries: vi.fn().mockResolvedValue({ list: [], total: 0 }),
  listSyncTasks: vi.fn().mockResolvedValue({ list: [], total: 0 }),
  clearSyncTasks: vi.fn(),
}))

vi.mock('@/api/rulespec', () => ({
  listCapabilityMeta: vi.fn().mockResolvedValue({ legacy: [], capabilities: [] }),
}))

vi.mock('@/api/subscription', () => ({
  listSubscriptions: vi.fn().mockResolvedValue([]),
  getSubscription: vi.fn(),
}))

vi.mock('@/api/version', () => ({
  versionApi: vi.fn(() => ({
    list: vi.fn().mockResolvedValue([]),
    preview: vi.fn().mockResolvedValue(''),
  })),
  getVersionBlueprint: vi.fn(),
}))

vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import AssemblyView from '@/views/admin/AssemblyView.vue'
import PoolTab from '@/views/admin/assembly/PoolTab.vue'
import { getAssemblyContext, previewAssembly, generateAssembly } from '@/api/assembly'
import { listPools } from '@/api/pool'
import { Notify } from '@/components/Notify'
import { ASSEMBLY_CONTEXT_KEY } from '@/utils/assemblyDraft'
import { ApiError } from '@/api/request'

const mockContext = getAssemblyContext as unknown as ReturnType<typeof vi.fn>
const mockPreview = previewAssembly as unknown as ReturnType<typeof vi.fn>
const mockGenerate = generateAssembly as unknown as ReturnType<typeof vi.fn>
const mockListPools = listPools as unknown as ReturnType<typeof vi.fn>
const mockNotifyWarning = Notify.warning as unknown as ReturnType<typeof vi.fn>

const context = {
  nodes: [],
  proxy_groups: [],
  pools: [{ id: 1, name: '池A', urls: [], entry_count: 0, last_synced_at: '', sync_status: '', sync_error: '', auto_sync: false, sync_time: '04:00' }],
  platforms: [{ id: 1, name: '平台A', product_type: 'yaml', is_default: true }],
  rules: [{ id: 2, name: '规则A', current_version: 0 }],
  subscriptions: [{ id: 11, platform_id: 1, name: '订阅A' }],
}

function makeRouter(query = 'tab=bad'): Router {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/assembly', component: AssemblyView }],
  })
  router.push(`/admin/assembly?${query}`)
  return router
}

async function mountWith(query = 'tab=bad') {
  const router = makeRouter(query)
  await router.isReady()
  const wrapper = mount(AssemblyView, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('AssemblyView 装配页核心交互', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    sessionStorage.clear()
    mockContext.mockReset()
    mockPreview.mockReset()
    mockGenerate.mockReset()
    mockListPools.mockReset()
    mockNotifyWarning.mockReset()
    mockContext.mockResolvedValue(context)
    mockPreview.mockResolvedValue({ content: 'proxies:', preview_hash: 'preview-hash', skipped: [], warnings: [] })
    mockGenerate.mockResolvedValue({ version_id: 1, version_no: 1, auto_activated: true, skipped: [], warnings: [] })
    mockListPools.mockResolvedValue(context.pools)
  })

  it('无效 tab 回退到 pool', async () => {
    const wrapper = await mountWith('tab=bad')
    const vm = wrapper.vm as unknown as { mainTab: string }
    expect(vm.mainTab).toBe('pool')
  })

  it('sr-conf 步骤定义跳过 nodes 步骤', async () => {
    const wrapper = await mountWith('tab=sr-conf')
    const vm = wrapper.vm as unknown as { stepDefs: Array<{ key: string }> }
    expect(vm.stepDefs.some((s) => s.key === 'nodes')).toBe(false)
    expect(vm.stepDefs.some((s) => s.key === 'rules')).toBe(true)
  })

  it('sr-conf 在无节点时不显示节点前置条件', async () => {
    const wrapper = await mountWith('tab=sr-conf')
    const vm = wrapper.vm as unknown as { buildPreflightMissing: Array<{ id: string }> }
    expect(vm.buildPreflightMissing.some((i) => i.id === 'nodes')).toBe(false)
  })

  it('节点类装配目标在无节点时仍显示节点前置条件', async () => {
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as { buildPreflightMissing: Array<{ id: string }> }
    expect(vm.buildPreflightMissing.some((i) => i.id === 'nodes')).toBe(true)
  })


  it('仅有默认平台时自动选择目标并隐藏目标卡片', async () => {
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as { currentStep: number; form: { platform_id?: number }; nextStep: () => void }
    expect(vm.form.platform_id).toBe(1)
    expect(wrapper.text()).not.toContain('目标选择')
    vm.currentStep = 0
    vm.nextStep()
    expect(vm.currentStep).toBe(1)
    expect(mockNotifyWarning).not.toHaveBeenCalled()
  })

  it('存在自定义平台时显示目标卡片并默认选择已有订阅的平台', async () => {
    mockContext.mockResolvedValue({
      ...context,
      platforms: [
        ...context.platforms,
        { id: 2, name: '自定义平台', product_type: 'yaml', is_default: false },
      ],
      subscriptions: [{ id: 12, platform_id: 2, name: '订阅B' }],
    })
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as { form: { platform_id?: number } }
    expect(vm.form.platform_id).toBe(2)
    expect(wrapper.text()).toContain('目标选择')
  })

  it('preview 调用后端预览接口', async () => {
    const wrapper = await mountWith()
    const vm = wrapper.vm as unknown as { form: { platform_id?: number }; doPreview: () => Promise<void> }
    vm.form.platform_id = 1
    await vm.doPreview()
    expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({
      overseas_members: [],
      fallback_group_members: ['🚀直接连接', '🌎国外流量'],
    }))
  })

  it('构建请求携带代理组成员顺序 group_member_orders', async () => {
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number; group_names: string[]; group_node_orders: Record<string, string[]>; group_member_orders: Record<string, string[]> }
      doPreview: () => Promise<void>
    }
    vm.form.platform_id = 1
    vm.form.group_names = ['组A']
    vm.form.group_node_orders = { '组A': ['节点A'] }
    vm.form.group_member_orders = { '组A': ['节点A', '🚀直接连接'] }
    await vm.doPreview()
    expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({
      group_member_orders: { '组A': ['节点A', '🚀直接连接'] },
    }))
  })

  it('素材池页刷新列表后构建候选立即更新，无需重新加载页面', async () => {
    const wrapper = await mountWith('tab=pool')
    const newPool = { ...context.pools[0], id: 2, name: '新同步素材池', entry_count: 12 }
    wrapper.findComponent(PoolTab).vm.$emit('pools-changed', [newPool])
    await wrapper.vm.$nextTick()

    const vm = wrapper.vm as unknown as {
      context: typeof context
      form: { pools: Array<{ pool_id: number; target: string }> }
      addPool: () => void
    }
    expect(vm.context.pools).toEqual([newPool])
    vm.addPool()
    expect(vm.form.pools[0].pool_id).toBe(2)
  })

  it('已选素材池内容变化后旧预览立即过期', async () => {
    const wrapper = await mountWith('tab=pool')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number; pools: Array<{ pool_id: number; target: string }> }
      doPreview: () => Promise<void>
      previewStale: boolean
      canGenerate: boolean
    }
    vm.form.platform_id = 1
    vm.form.pools = [{ pool_id: 1, target: '🚀直接连接' }]
    await vm.doPreview()
    expect(vm.previewStale).toBe(false)
    expect(vm.canGenerate).toBe(true)

    wrapper.findComponent(PoolTab).vm.$emit('pool-content-changed', 1)
    await wrapper.vm.$nextTick()
    expect(vm.previewStale).toBe(true)
    expect(vm.canGenerate).toBe(false)
  })

  it('生成端发现预览摘要冲突后将旧预览转为过期态', async () => {
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number }
      doPreview: () => Promise<void>
      doGenerate: () => Promise<void>
      previewStale: boolean
      canGenerate: boolean
    }
    vm.form.platform_id = 1
    await vm.doPreview()
    mockGenerate.mockRejectedValueOnce(new ApiError(409, '装配依赖已变化，请重新预览'))
    await vm.doGenerate()
    expect(vm.previewStale).toBe(true)
    expect(vm.canGenerate).toBe(false)
  })

  it('预览完成后可生成；任一产物字段变更后旧预览过期', async () => {
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number; fixed_params_text: string }
      doPreview: () => Promise<void>
      doGenerate: () => Promise<void>
      previewStale: boolean
      canGenerate: boolean
    }
    vm.form.platform_id = 1
    await vm.doPreview()
    expect(vm.previewStale).toBe(false)
    expect(vm.canGenerate).toBe(true)
    await vm.doGenerate()
    expect(mockGenerate).toHaveBeenCalledTimes(1)

    vm.form.fixed_params_text = '{"remarks":"已修改"}'
    await wrapper.vm.$nextTick()
    expect(vm.previewStale).toBe(true)
    expect(vm.canGenerate).toBe(false)
    await vm.doGenerate()
    expect(mockGenerate).toHaveBeenCalledTimes(1)
    expect(mockNotifyWarning).toHaveBeenCalledWith('配置已变化，请重新预览')
  })

  it('节点订阅本地过滤错误返回的空规则警告', async () => {
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as {
      previewWarnings: string[]
      visiblePreviewWarnings: string[]
    }
    vm.previewWarnings = ['未选择任何规则素材池或手动规则，将生成空规则', '节点名称已变化']
    await wrapper.vm.$nextTick()
    expect(vm.visiblePreviewWarnings).toEqual(['节点名称已变化'])
  })

  it('从前置条件页返回时恢复草稿并清理上下文', async () => {
    sessionStorage.setItem(ASSEMBLY_CONTEXT_KEY, JSON.stringify({
      version: 1,
      createdAt: Date.now(),
      expiresAt: Date.now() + 30 * 60 * 1000,
      sourceLabel: 'SR 节点订阅 · 至少一个可用节点',
      returnPath: '/admin/assembly',
      mainTab: 'build',
      subTab: 'sr-subs',
      currentStep: 1,
      layoutMode: 'step',
      form: {
        platform_id: 1,
        rule_name: '',
        sr_rule_mode: 'new',
        fixed_params_text: '{"remarks":"草稿"}',
        node_names: ['node-a'],
        group_names: [],
        group_node_orders: {},
        overseas_members: [],
        fallback_group_members: ['🚀直接连接', '🌎国外流量'],
        pools: [],
        custom_rules: [],
        final_direction: 'PROXY',
        overlay: { merge_yaml: '', rules_yaml: '', proxies_yaml: '', groups_yaml: '' },
      },
    }))
    const wrapper = await mountWith('tab=bad')
    const vm = wrapper.vm as unknown as { mainTab: string; subTab: string; currentStep: number; form: { fixed_params_text: string; node_names: string[] } }
    expect(vm.mainTab).toBe('build')
    expect(vm.subTab).toBe('sr-subs')
    expect(vm.currentStep).toBe(1)
    expect(vm.form.fixed_params_text).toBe('{"remarks":"草稿"}')
    expect(vm.form.node_names).toEqual(['node-a'])
    expect(sessionStorage.getItem(ASSEMBLY_CONTEXT_KEY)).toBeNull()
  })

  it('一键剔除失效引用', async () => {
    const wrapper = await mountWith()
    const vm = wrapper.vm as unknown as {
      invalidRefs: Array<{ kind: string; name: string }>
      form: { node_names: string[]; pools: Array<{ pool_id: number; target: string }> }
      removeAllInvalidRefs: () => void
    }
    vm.form.node_names = ['node-a', 'node-b']
    vm.form.pools = [{ pool_id: 1, target: 'PROXY' }, { pool_id: 99, target: 'PROXY' }]
    vm.invalidRefs = [
      { kind: 'node', name: 'node-a' },
      { kind: 'pool', name: '99' },
    ]
    vm.removeAllInvalidRefs()
    expect(vm.form.node_names).toEqual(['node-b'])
    expect(vm.form.pools).toEqual([{ pool_id: 1, target: 'PROXY' }])
    expect(vm.invalidRefs).toEqual([])
  })

  it('目标类型 Tab 使用面向用户展示文案', async () => {
    const wrapper = await mountWith('tab=clash-yaml')
    expect(wrapper.text()).toContain('Clash - V2Ray/Mihomo（新版）')
    expect(wrapper.text()).toContain('Shadowrocket 订阅组')
    expect(wrapper.text()).toContain('通用V2Ray格式')
    expect(wrapper.text()).toContain('Shadowrocket规则组')
  })
})
