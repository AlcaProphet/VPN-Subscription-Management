// pool-detail.spec.ts：素材池详情同步历史分页与卸载取消轮询（R12-05）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PoolDetail from '@/views/admin/assembly/PoolDetail.vue'

vi.mock('@/api/pool', () => ({
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  submitSync: vi.fn(),
  getSyncStatus: vi.fn(),
  listSyncTasks: vi.fn(),
  clearSyncTasks: vi.fn(),
}))

vi.mock('@/api/rulespec', () => ({
  listCapabilityMeta: vi.fn().mockResolvedValue({ legacy: [], capabilities: [] }),
}))

vi.mock('@/api/request', () => {
  class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  }
  return { pollTask: vi.fn(), ApiError }
})

vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import { createEntry, listEntries, listSyncTasks, clearSyncTasks } from '@/api/pool'
import { listCapabilityMeta } from '@/api/rulespec'
import { pollTask } from '@/api/request'

const mockListEntries = listEntries as unknown as ReturnType<typeof vi.fn>
const mockListTasks = listSyncTasks as unknown as ReturnType<typeof vi.fn>
const mockPollTask = pollTask as unknown as ReturnType<typeof vi.fn>
const mockCreateEntry = createEntry as unknown as ReturnType<typeof vi.fn>
const mockListCapabilityMeta = listCapabilityMeta as unknown as ReturnType<typeof vi.fn>
const mockClearSyncTasks = clearSyncTasks as unknown as ReturnType<typeof vi.fn>

const pool = {
  id: 1, name: '苹果域名', urls: ['https://example.com/rules.txt'], entry_count: 2,
  last_synced_at: '2026-08-19T10:00:00Z', sync_status: 'succeeded', sync_error: '',
  auto_sync: true, sync_time: '04:00',
}
const entry = {
  id: 1, pool_id: 1, rule_type: 'DOMAIN-SUFFIX', match_value: 'example.com', source: 'manual' as const, sort_order: 1,
}
const urlEntry = {
  id: 2, pool_id: 1, rule_type: 'DOMAIN-SUFFIX', match_value: 'url.example.com', source: 'url' as const, sort_order: 100000,
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((res) => { resolve = res })
  return { promise, resolve }
}

describe('PoolDetail', () => {
  beforeEach(() => {
    mockListEntries.mockReset()
    mockListTasks.mockReset()
    mockPollTask.mockReset()
    mockCreateEntry.mockReset()
  })

  it('同步历史支持分页加载', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 2 })
    mockListTasks
      .mockResolvedValueOnce({
        list: [{
          task_id: 1, pool_id: 1, status: 'succeeded', per_url: [], error: '',
          started_at: '2026-08-19T10:00:00Z', finished_at: '2026-08-19T10:01:00Z',
        }],
        total: 2,
      })
      .mockResolvedValueOnce({
        list: [{
          task_id: 2, pool_id: 1, status: 'failed', per_url: [], error: '拉取失败',
          started_at: '2026-08-19T11:00:00Z', finished_at: '2026-08-19T11:01:00Z',
        }],
        total: 2,
      })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(wrapper.text()).toContain('成功')
    const vm = wrapper.vm as unknown as { taskPage: number }
    vm.taskPage = 2
    await flushPromises()
    expect(mockListTasks).toHaveBeenCalledWith(1, 2, 20)
    expect(wrapper.text()).toContain('失败')
    expect(wrapper.text()).toContain('原因')
  })

  it('URL 同步条目默认不查询，展开后按来源单独加载', async () => {
    mockListEntries
      .mockResolvedValueOnce({ list: [entry], total: 1 })
      .mockResolvedValueOnce({ list: [urlEntry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(mockListEntries).toHaveBeenCalledTimes(1)
    expect(mockListEntries).toHaveBeenLastCalledWith(1, 1, 20, 'manual')
    expect(wrapper.text()).not.toContain('url.example.com')

    await wrapper.get('[data-testid="toggle-url-entries"]').trigger('click')
    await flushPromises()
    expect(mockListEntries).toHaveBeenLastCalledWith(1, 1, 20, 'url')
    expect(wrapper.text()).toContain('url.example.com')
  })

  it('组件卸载时取消正在进行的同步轮询', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    const d = deferred<{ status: string }>()
    const cancel = vi.fn()
    const run = vi.fn(() => d.promise)
    mockPollTask.mockReturnValue({ run, cancel })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { doSync: () => Promise<void> }
    const first = vm.doSync()
    await Promise.resolve()
    wrapper.unmount()
    expect(cancel).toHaveBeenCalled()
    d.resolve({ status: 'succeeded' })
    await first
  })

  it('手动条目新增后向上通知素材池内容变化', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    mockCreateEntry.mockResolvedValue(entry)
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      entryForm: { rule_type: string; match_value: string }
      saveEntry: () => Promise<void>
    }
    vm.entryForm.match_value = 'new.example.com'
    await vm.saveEntry()
    expect(wrapper.emitted('changed')).toEqual([[pool.id]])
  })

  it('素材池规则类型下拉从后端 snake_case 元数据生成', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    mockListCapabilityMeta.mockResolvedValue({
      legacy: [
        { rule_type: 'DOMAIN', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: false },
        { rule_type: 'DOMAIN-SUFFIX', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: false },
        { rule_type: 'IP-CIDR', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: true },
        { rule_type: 'GEOSITE', scope: 'clash_only', material_pool: false, advanced: true, supports_no_resolve: false },
      ],
      capabilities: [],
    })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { manualRuleTypes: string[] }
    expect(vm.manualRuleTypes).toContain('DOMAIN')
    expect(vm.manualRuleTypes).toContain('DOMAIN-SUFFIX')
    expect(vm.manualRuleTypes).toContain('IP-CIDR')
    expect(vm.manualRuleTypes).not.toContain('GEOSITE')
  })

  it('清理已完成历史调用当前池接口并刷新历史', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks
      .mockResolvedValueOnce({ list: [{ task_id: 1, pool_id: 1, status: 'succeeded', per_url: [], error: '', started_at: '', finished_at: '' }], total: 1 })
      .mockResolvedValueOnce({ list: [], total: 0 })
    mockClearSyncTasks.mockResolvedValue({ cleared: 1 })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { clearOpen: boolean; confirmClearTasks: () => Promise<void> }
    vm.clearOpen = true
    await vm.confirmClearTasks()
    expect(mockClearSyncTasks).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('暂无同步任务')
  })

  it('同步历史展示后端实际返回的解析统计而非空计数', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({
      list: [{
        task_id: 1, pool_id: 1, status: 'succeeded',
        per_url: [{ url: 'https://example.com/rules.txt', ok: true, accepted: 112, excluded: 0, rejected: 2, duplicates: 3, pending: false, error: '' }],
        error: '', started_at: '', finished_at: '',
      }],
      total: 1,
    })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(wrapper.text()).toContain('接受 112')
    expect(wrapper.text()).toContain('拒绝 2')
    expect(wrapper.text()).toContain('重复 3')
    expect(wrapper.text()).not.toContain('新增 undefined')
  })


})
