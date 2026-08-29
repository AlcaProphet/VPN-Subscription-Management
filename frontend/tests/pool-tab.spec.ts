// pool-tab.spec.ts：素材池列表加载/空态/同步防重与卸载取消轮询（Build4 Step6 / R12-05）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PoolTab from '@/views/admin/assembly/PoolTab.vue'

vi.mock('@/api/pool', () => ({
  listPools: vi.fn(),
  createPool: vi.fn(),
  updatePool: vi.fn(),
  deletePool: vi.fn(),
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

import { listPools } from '@/api/pool'
import { pollTask } from '@/api/request'
import { Notify } from '@/components/Notify'

const mockList = listPools as unknown as ReturnType<typeof vi.fn>
const mockPollTask = pollTask as unknown as ReturnType<typeof vi.fn>

const pool = {
  id: 1, name: '苹果域名', urls: ['https://example.com/rules.txt'], entry_count: 3,
  last_synced_at: '2026-08-19T10:00:00Z', sync_status: 'succeeded', sync_error: '',
  auto_sync: true, sync_time: '04:00',
}

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('PoolTab', () => {
  beforeEach(() => {
    mockList.mockReset()
    mockPollTask.mockReset()
    vi.mocked(Notify.warning).mockClear()
    vi.mocked(Notify.success).mockClear()
    vi.mocked(Notify.error).mockClear()
  })

  it('空列表显示空态文案', async () => {
    mockList.mockResolvedValue([])
    const wrapper = mount(PoolTab)
    await flushPromises()
    expect(wrapper.text()).toContain('还没有规则素材池')
  })

  it('列表加载后显示池名与定时时刻', async () => {
    mockList.mockResolvedValue([pool])
    const wrapper = mount(PoolTab)
    await flushPromises()
    expect(wrapper.text()).toContain('苹果域名')
    expect(wrapper.text()).toContain('每日 04:00 UTC')
    expect(wrapper.emitted('pools-changed')?.[0]).toEqual([[pool]])
  })

  it('同步进行中再次点击提示 warning', async () => {
    mockList.mockResolvedValue([pool])
    const d = deferred<{ status: string }>()
    const cancel = vi.fn()
    const run = vi.fn(() => d.promise)
    mockPollTask.mockReturnValue({ run, cancel })
    const wrapper = mount(PoolTab)
    await flushPromises()
    const vm = wrapper.vm as unknown as { doSync: (p: typeof pool) => Promise<void> }
    const first = vm.doSync(pool)
    await Promise.resolve()
    await vm.doSync(pool)
    expect(Notify.warning).toHaveBeenCalledWith('同步进行中，请等待完成')
    d.resolve({ status: 'succeeded' })
    await first
    expect(wrapper.emitted('pool-content-changed')).toEqual([[pool.id]])
  })

  it('组件卸载时取消正在进行的轮询', async () => {
    mockList.mockResolvedValue([pool])
    const d = deferred<{ status: string }>()
    const cancel = vi.fn()
    const run = vi.fn(() => d.promise)
    mockPollTask.mockReturnValue({ run, cancel })
    const wrapper = mount(PoolTab)
    await flushPromises()
    const vm = wrapper.vm as unknown as { doSync: (p: typeof pool) => Promise<void> }
    const first = vm.doSync(pool)
    await Promise.resolve()
    wrapper.unmount()
    expect(cancel).toHaveBeenCalled()
    d.resolve({ status: 'succeeded' })
    await first
  })
})
