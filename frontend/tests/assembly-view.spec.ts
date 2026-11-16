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
  setPoolAutoSync: vi.fn(),
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
import PreviewStep from '@/views/admin/assembly/PreviewStep.vue'
import { getAssemblyContext, previewAssembly, generateAssembly } from '@/api/assembly'
import { listPools } from '@/api/pool'
import type { PoolItem } from '@/api/pool'
import { Notify } from '@/components/Notify'
import { ASSEMBLY_CONTEXT_KEY } from '@/utils/assemblyDraft'
import { ApiError } from '@/api/request'

const mockContext = getAssemblyContext as unknown as ReturnType<typeof vi.fn>
const mockPreview = previewAssembly as unknown as ReturnType<typeof vi.fn>
const mockGenerate = generateAssembly as unknown as ReturnType<typeof vi.fn>
const mockListPools = listPools as unknown as ReturnType<typeof vi.fn>
const mockNotifyWarning = Notify.warning as unknown as ReturnType<typeof vi.fn>
const oneRule = {
  input: 1, direct_output: 1, equivalent_conversions: 0,
  skipped_unsupported: 0, target_validation_failed: 0, final_output: 1,
}

const context = {
  nodes: [],
  proxy_groups: [],
  pools: [{ id: 1, name: '池A', urls: [], entry_count: 0, last_synced_at: '', sync_status: '', sync_error: '', auto_sync: false, sync_time: '04:00' }],
  platforms: [{ id: 1, name: '平台A', product_type: 'yaml', is_default: true }],
  rules: [{ id: 2, name: '规则A', current_version: 0 }],
  subscriptions: [{ id: 11, platform_id: 1, name: '订阅A' }],
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
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

  it('preview 回执保存、传给预览步骤并渲染六项数字', async () => {
    const receipt = {
      input: 7, direct_output: 4, equivalent_conversions: 2,
      skipped_unsupported: 1, target_validation_failed: 0, final_output: 6,
    }
    mockPreview.mockResolvedValue({ content: 'proxies:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt })
    const wrapper = await mountWith('tab=sr-conf')
    const vm = wrapper.vm as unknown as { addPool: () => void; doPreview: () => Promise<void>; previewReceipt: unknown }
    vm.addPool()
    await vm.doPreview()
    await wrapper.vm.$nextTick()
    expect(vm.previewReceipt).toEqual(receipt)
    expect(wrapper.findComponent(PreviewStep).props('receipt')).toEqual(receipt)
    expect(wrapper.text()).toContain('输入 7')
    expect(wrapper.text()).toContain('直接输出 4')
    expect(wrapper.text()).toContain('等价转换 2')
    expect(wrapper.text()).toContain('目标不支持跳过 1')
    expect(wrapper.text()).toContain('校验失败 0')
    expect(wrapper.text()).toContain('最终输出 6')
  })

  it('目标或输入变化后旧 preview 回执被清除且不显示', async () => {
    const receipt = {
      input: 7, direct_output: 4, equivalent_conversions: 2,
      skipped_unsupported: 1, target_validation_failed: 0, final_output: 6,
    }
    mockPreview.mockResolvedValue({ content: 'proxies:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt })
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number; fixed_params_text: string }
      doPreview: () => Promise<void>
      previewStale: boolean
      previewReceipt: unknown
    }
    vm.form.platform_id = 1
    await vm.doPreview()
    await wrapper.vm.$nextTick()
    expect(vm.previewReceipt).toEqual(receipt)

    vm.form.platform_id = 2
    await wrapper.vm.$nextTick()
    expect(vm.previewStale).toBe(true)
    expect(vm.previewReceipt).toBeNull()

    vm.form.platform_id = 1
    vm.form.fixed_params_text = '{"port":7891}'
    await wrapper.vm.$nextTick()
    expect(vm.previewStale).toBe(true)
    expect(vm.previewReceipt).toBeNull()
    expect(wrapper.text()).not.toContain('转换回执')
  })

  it('generate 使用本次服务端回执，不与 preview 回执混用', async () => {
    const previewReceipt = {
      input: 7, direct_output: 4, equivalent_conversions: 2,
      skipped_unsupported: 1, target_validation_failed: 0, final_output: 6,
    }
    const generateReceipt = {
      input: 9, direct_output: 5, equivalent_conversions: 3,
      skipped_unsupported: 1, target_validation_failed: 0, final_output: 8,
    }
    mockPreview.mockResolvedValue({ content: 'proxies:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt: previewReceipt })
    mockGenerate.mockResolvedValue({
      version_id: 1, version_no: 1, auto_activated: true, skipped: [], warnings: [], receipt: generateReceipt,
    })
    const wrapper = await mountWith('tab=sr-conf')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number; sr_rule_mode: 'existing' | 'new'; rule_name: string }
      doPreview: () => Promise<void>
      doGenerate: () => Promise<void>
      addPool: () => void
      generateResult: { receipt?: unknown } | null
    }
    vm.form.platform_id = 1
    vm.form.sr_rule_mode = 'new'
    vm.form.rule_name = 'receipt-test'
    vm.addPool()
    await vm.doPreview()
    await vm.doGenerate()
    await wrapper.vm.$nextTick()
    expect(vm.generateResult?.receipt).toEqual(generateReceipt)
    expect(wrapper.text()).toContain('最终输出 8')
    expect(wrapper.text()).not.toContain('最终输出 6')
  })

  it('preview 与 generate 缺省 receipt 时保持兼容', async () => {
    mockPreview.mockResolvedValue({ content: 'proxies:', preview_hash: 'preview-hash', skipped: [], warnings: [] })
    mockGenerate.mockResolvedValue({ version_id: 1, version_no: 1, auto_activated: true, skipped: [], warnings: [] })
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as {
      form: { platform_id?: number }
      doPreview: () => Promise<void>
      doGenerate: () => Promise<void>
      previewReceipt: unknown
    }
    vm.form.platform_id = 1
    await vm.doPreview()
    await wrapper.vm.$nextTick()
    expect(vm.previewReceipt).toBeNull()
    expect(wrapper.text()).not.toContain('转换回执')
    await vm.doGenerate()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).not.toContain('转换回执')
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

  it('已返回的池列表保留到首次装配上下文就绪', async () => {
    const initial = deferred<typeof context>()
    const newPool = { ...context.pools[0], id: 2, name: '新同步素材池', entry_count: 12 }
    mockContext.mockReturnValueOnce(initial.promise)
    mockListPools.mockResolvedValueOnce([]).mockResolvedValue([newPool])
    const wrapper = await mountWith('tab=pool')
    const manager = wrapper.findComponent(PoolTab)
    await manager.props('refreshPools')()
    await wrapper.vm.$router.replace('/admin/assembly?tab=clash-yaml')
    await flushPromises()
    initial.resolve({ ...context, pools: [] })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      context: typeof context
      form: { pools: Array<{ pool_id: number; target: string }> }
      addPool: () => void
    }
    expect(vm.context.pools).toEqual([newPool])
    vm.addPool()
    expect(vm.form.pools[0].pool_id).toBe(2)
  })

  it('忽略晚到的旧池列表，保留候选、选择与有效预览', async () => {
    const oldList = deferred<typeof context.pools>()
    const newPool = { ...context.pools[0], id: 2, name: '新素材池', entry_count: 1 }
    mockListPools.mockReturnValueOnce(oldList.promise).mockResolvedValue([newPool])
    mockPreview.mockResolvedValue({ content: 'rules:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt: oneRule })
    const wrapper = await mountWith('tab=pool')
    await wrapper.vm.$router.replace('/admin/assembly?tab=clash-yaml')
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      context: typeof context
      form: { pools: Array<{ pool_id: number; target: string }> }
      addPool: () => void
      doPreview: () => Promise<void>
      previewStale: boolean
    }
    vm.addPool()
    expect(vm.form.pools[0].pool_id).toBe(newPool.id)
    await vm.doPreview()
    expect(vm.previewStale).toBe(false)
    oldList.resolve([])
    await flushPromises()
    expect(wrapper.findComponent(PoolTab).props('pools')).toEqual([newPool])
    expect(vm.context.pools).toEqual([newPool])
    expect(vm.form.pools[0].pool_id).toBe(newPool.id)
    expect(vm.previewStale).toBe(false)
    expect(mockNotifyWarning).not.toHaveBeenCalled()
  })

  it('已选素材池内容变化后旧预览立即过期', async () => {
    mockPreview.mockResolvedValue({ content: 'rules:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt: oneRule })
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

  it('只在已选池的活动来源快照发生变化时使预览过期', async () => {
    const pool: PoolItem = {
      ...context.pools[0],
      entry_count: 2,
      sources: [
        { id: 1, pool_id: 1, kind: 'url', sort_order: 0, source_mode: 'auto', active_snapshot_id: 10 },
        { id: 2, pool_id: 1, kind: 'url', sort_order: 1, source_mode: 'auto', active_snapshot_id: 20 },
      ],
    }
    mockContext.mockResolvedValue({ ...context, pools: [pool] })
    mockListPools.mockResolvedValue([pool])
    mockPreview.mockResolvedValue({ content: 'rules:', preview_hash: 'preview-hash', skipped: [], warnings: [], receipt: oneRule })
    const wrapper = await mountWith('tab=pool')
    const vm = wrapper.vm as unknown as {
      form: { pools: Array<{ pool_id: number; target: string }> }
      doPreview: () => Promise<void>
      canGenerate: boolean
    }
    vm.form.pools = [{ pool_id: 1, target: '🚀直接连接' }]
    await vm.doPreview()
    expect(vm.canGenerate).toBe(true)

    const refresh = wrapper.findComponent(PoolTab).props('refreshPools')
    const renamed = { ...pool, name: '改名', auto_sync: true }
    mockListPools.mockResolvedValueOnce([renamed])
    await refresh()
    expect(vm.canGenerate).toBe(true)

    mockListPools.mockResolvedValueOnce([{ ...renamed, sources: [...pool.sources!].reverse() }])
    await refresh()
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
    mockPreview.mockResolvedValue({
      content: 'vless://example', preview_hash: 'node-hash', skipped: [],
      warnings: ['未选择任何规则素材池或手动规则，将生成空规则', '节点名称已变化'],
    })
    const wrapper = await mountWith('tab=sr-subs')
    const vm = wrapper.vm as unknown as {
      doPreview: () => Promise<void>
      visiblePreviewWarnings: string[]
    }
    await vm.doPreview()
    expect(vm.visiblePreviewWarnings).toEqual(['节点名称已变化'])
  })

  it('Clash 无池可到覆盖层，有效结果决定生成，旧表单恢复后保留原回执', async () => {
    mockContext.mockResolvedValue({ ...context, pools: [], nodes: [{ name: '节点A', render_name: '节点A', source: 'manual', enabled: true }] })
    mockPreview.mockResolvedValue({
      content: 'rules: [MATCH,DIRECT]', preview_hash: 'zero', skipped: [],
      warnings: [], receipt: { ...oneRule, final_output: 0 },
    })
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as {
      currentStep: number; currentStepKey: string; canGenerate: boolean
      form: { pools: unknown[]; overseas_members: string[]; overlay: { rules_yaml: string } }
      doPreview: () => Promise<void>; doGenerate: () => Promise<void>; addPool: () => void
    }
    vm.currentStep = 2
    vm.form.overseas_members = ['节点A']
    await wrapper.vm.$nextTick()
    const button = (label: string) => wrapper.findAll('button').find((item) => item.text() === label && item.isVisible())!
    expect(button('添加素材池').attributes('disabled')).toBeDefined()
    expect(button('前往规则素材池').exists()).toBe(true)
    vm.addPool()
    expect(vm.form.pools).toEqual([])
    await button('下一步').trigger('click')
    expect(vm.currentStepKey).toBe('overlay')
    await button('下一步').trigger('click')
    await flushPromises()
    expect(button('确认生成').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('当前目标没有可输出的非系统规则')
    vm.form.overlay.rules_yaml = 'prepend: ["DOMAIN,overlay.test,DIRECT"]'
    await vm.doGenerate()
    vm.form.overlay.rules_yaml = ''
    await vm.doGenerate()
    expect(vm.canGenerate).toBe(false)
    expect(mockGenerate).not.toHaveBeenCalled()

    vm.form.overlay.rules_yaml = 'append: ["DOMAIN,overlay.test,DIRECT"]'
    mockPreview.mockResolvedValue({
      content: 'rules: ["MATCH,DIRECT", "DOMAIN,overlay.test,DIRECT"]',
      preview_hash: 'overlay', skipped: [], warnings: ['可能被前面的 MATCH 覆盖'], receipt: { ...oneRule, input: 0, direct_output: 0 },
    })
    await vm.doPreview()
    expect(vm.canGenerate).toBe(true)
    await vm.doGenerate()
    expect(mockGenerate).toHaveBeenCalledWith(expect.objectContaining({ pools: [], custom_rules: [], preview_hash: 'overlay' }))
  })

  it('SR 分流规则单页和分步拦截空输入，添加素材池使用 PROXY', async () => {
    const wrapper = await mountWith('tab=sr-conf')
    const vm = wrapper.vm as unknown as {
      currentStep: number; layoutMode: string; canGenerate: boolean
      form: { pools: Array<{ pool_id: number; target: string }> }; doPreview: () => Promise<void>; addPool: () => void
    }
    vm.currentStep = 1
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('button').find((item) => item.text() === '下一步')?.attributes('disabled')).toBeDefined()
    vm.layoutMode = 'page'
    await flushPromises()
    expect(mockPreview).not.toHaveBeenCalled()
    expect(wrapper.findAll('button').find((item) => item.text() === '刷新预览')?.attributes('disabled')).toBeDefined()
    vm.addPool()
    expect(vm.form.pools).toEqual([{ pool_id: 1, target: 'PROXY' }])
    mockPreview.mockResolvedValue({ content: '[Rule]', preview_hash: 'sr', skipped: [], warnings: [], receipt: { ...oneRule, final_output: 0 } })
    await vm.doPreview()
    expect(vm.canGenerate).toBe(false)
  })

  it('并发预览只采用当前结果，静态错误阻止提交', async () => {
    const wrapper = await mountWith('tab=clash-yaml')
    const vm = wrapper.vm as unknown as {
      form: { fixed_params_text: string }
      canGenerate: boolean; doPreview: () => Promise<void>; doGenerate: () => Promise<void>
    }
    let resolveOld!: (value: unknown) => void
    const base = { content: 'rules:', preview_hash: 'hash', skipped: [], warnings: [], receipt: oneRule }
    mockPreview.mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve }))
    const old = vm.doPreview()
    vm.form.fixed_params_text = '{"port":7100}'
    mockPreview.mockResolvedValueOnce({ ...base, issues: [{ severity: 'error', path: '$.rules[0]', message: '规则目标不存在' }] })
    await vm.doPreview()
    resolveOld(base)
    await old
    expect(vm.canGenerate).toBe(false)
    await vm.doGenerate()
    expect(mockGenerate).not.toHaveBeenCalled()
    expect(mockNotifyWarning).toHaveBeenCalledWith(expect.stringContaining('规则目标不存在'))
  })

  it.each(['sr-subs', 'generic-subs'])('节点订阅 %s 忽略隐藏规则和代理组草稿', async (tab) => {
    const wrapper = await mountWith(`tab=${tab}`)
    const vm = wrapper.vm as unknown as {
      form: { pools: unknown[]; custom_rules: unknown[]; group_names: string[] }
      doPreview: () => Promise<void>; canGenerate: boolean
    }
    vm.form.pools = [{ pool_id: 99, target: '旧 Clash 组' }]
    vm.form.custom_rules = [{ rule_type: 'DOMAIN', match_value: '', target: '' }]
    vm.form.group_names = ['旧 Clash 组']
    await vm.doPreview()
    expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({ pools: [], custom_rules: [], group_names: [] }))
    expect(vm.form.pools).toHaveLength(1)
    expect(vm.canGenerate).toBe(true)
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
