// pool-tab.spec.ts：素材池列表加载/空态（Build4 Step6）
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

import { listPools } from '@/api/pool'

const mockList = listPools as unknown as ReturnType<typeof vi.fn>

describe('PoolTab', () => {
  beforeEach(() => {
    mockList.mockReset()
  })

  it('空列表显示空态文案', async () => {
    mockList.mockResolvedValue([])
    const wrapper = mount(PoolTab)
    await flushPromises()
    expect(wrapper.text()).toContain('还没有规则素材池')
  })

  it('列表加载后显示池名与定时时刻', async () => {
    mockList.mockResolvedValue([{
      id: 1, name: '苹果域名', urls: ['https://example.com/rules.txt'], entry_count: 3,
      last_synced_at: '2026-08-19T10:00:00Z', sync_status: 'succeeded', sync_error: '',
      auto_sync: true, sync_time: '04:00',
    }])
    const wrapper = mount(PoolTab)
    await flushPromises()
    expect(wrapper.text()).toContain('苹果域名')
    expect(wrapper.text()).toContain('每日 04:00 UTC')
  })
})
