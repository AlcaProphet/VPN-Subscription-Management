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

describe('UsersView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
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
})
