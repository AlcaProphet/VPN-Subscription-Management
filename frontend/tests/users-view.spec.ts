import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/user', () => ({
  listUsers: vi.fn().mockResolvedValue({ list: [], total: 0 }),
  createUser: vi.fn(),
  updateUser: vi.fn(),
  changeRole: vi.fn(),
  revokeTokens: vi.fn(),
  resetPassword: vi.fn(),
  clearOidc: vi.fn(),
  setStatus: vi.fn(),
  deleteUser: vi.fn(),
  sendPasswordLinks: vi.fn(),
  setUserQuota: vi.fn(),
}))
vi.mock('@/api/group', () => ({
  listGroups: vi.fn().mockResolvedValue([]),
}))
vi.mock('@/api/platform', () => ({
  listPlatforms: vi.fn().mockResolvedValue([]),
}))
vi.mock('@/api/custom', () => ({
  upsertCustom: vi.fn(),
  upsertCustomText: vi.fn(),
  deleteCustom: vi.fn(),
}))
vi.mock('@/api/settings', () => ({
  getSMTP: vi.fn().mockResolvedValue({}),
}))
vi.mock('@/api/xray', () => ({
  retryUserSync: vi.fn(),
  resetQuota: vi.fn(),
}))

vi.mock('@/api/system', () => ({
  getSystemStatus: vi.fn().mockResolvedValue({ configured: true, app_mode: 'dev', advanced_mode: false }),
}))

import UsersView from '@/views/admin/UsersView.vue'
import { listUsers } from '@/api/user'
import { getSystemStatus } from '@/api/system'

describe('UsersView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.body.innerHTML = ''
    vi.clearAllMocks()
    ;(listUsers as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ list: [], total: 0 })
  })

  it('空态展示暂无用户', async () => {
    const wrapper = mount(UsersView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无用户')
  })

  it('列表展示用户名与邮箱', async () => {
    ;(listUsers as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      list: [{ id: 1, username: 'admin1', email: 'admin1@x.com', role: 'admin', status: 'active', custom_subs: [] }],
      total: 1,
    })
    const wrapper = mount(UsersView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('admin1')
    expect(wrapper.text()).toContain('admin1@x.com')
  })

  it('高级模式下展示用量、配额与同步状态列', async () => {
    ;(getSystemStatus as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ configured: true, app_mode: 'dev', advanced_mode: true })
    ;(listUsers as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      list: [{
        id: 1, username: 'u1', email: 'u1@x.com', role: 'user', status: 'active', custom_subs: [],
        group_name: '默认组', used_bytes: 1024, effective_quota: 10, quota_override: null, quota_exceeded: false, sync_status: 'synced',
      }],
      total: 1,
    })
    const wrapper = mount(UsersView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('本月用量')
    expect(wrapper.text()).toContain('同步状态')
    expect(wrapper.text()).toContain('默认组')
    expect(wrapper.text()).toContain('已同步')
  })
})
