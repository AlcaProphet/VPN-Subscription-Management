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

import { createEntry, listEntries, listSyncTasks } from '@/api/pool'
import { pollTask } from '@/api/request'

const mockListEntries = listEntries as unknown as ReturnType<typeof vi.fn>
const mockListTasks = listSyncTasks as unknown as ReturnType<typeof vi.fn>
const mockPollTask = pollTask as unknown as ReturnType<typeof vi.fn>
const mockCreateEntry = createEntry as unknown as ReturnType<typeof vi.fn>

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
})
