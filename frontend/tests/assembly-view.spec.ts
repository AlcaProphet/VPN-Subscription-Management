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
import { getAssemblyContext, previewAssembly } from '@/api/assembly'
import { Notify } from '@/components/Notify'

const mockContext = getAssemblyContext as unknown as ReturnType<typeof vi.fn>
const mockPreview = previewAssembly as unknown as ReturnType<typeof vi.fn>
const mockNotifyWarning = Notify.warning as unknown as ReturnType<typeof vi.fn>

const context = {
  nodes: [],
  proxy_groups: [],
  pools: [{ id: 1, name: '池A', urls: [], entry_count: 0, last_synced_at: '', sync_status: '', sync_error: '', auto_sync: false, sync_time: '04:00' }],
  platforms: [{ id: 1, name: '平台A', product_type: 'yaml' }],
  rules: [{ id: 2, name: '规则A', current_version: 0 }],
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
    mockContext.mockReset()
    mockPreview.mockReset()
    mockNotifyWarning.mockReset()
    mockContext.mockResolvedValue(context)
    mockPreview.mockResolvedValue({ content: 'proxies:', skipped: [], warnings: [] })
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

  it('未选择目标时下一步被拦截', async () => {
    const wrapper = await mountWith()
    const vm = wrapper.vm as unknown as { currentStep: number; nextStep: () => void }
    vm.currentStep = 0
    vm.nextStep()
    expect(vm.currentStep).toBe(0)
    expect(mockNotifyWarning).toHaveBeenCalled()
  })

  it('preview 调用后端预览接口', async () => {
    const wrapper = await mountWith()
    const vm = wrapper.vm as unknown as { form: { platform_id?: number }; doPreview: () => Promise<void> }
    vm.form.platform_id = 1
    await vm.doPreview()
    expect(mockPreview).toHaveBeenCalled()
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
})
