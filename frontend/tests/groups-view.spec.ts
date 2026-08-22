import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/group', () => ({
  listGroups: vi.fn().mockResolvedValue([]),
  getGroup: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  deleteGroup: vi.fn(),
  updateGroupNodes: vi.fn(),
  updateGroupQuota: vi.fn(),
}))

import GroupsView from '@/views/admin/GroupsView.vue'
import { listGroups } from '@/api/group'

describe('GroupsView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
    ;(listGroups as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
  })

  it('空态展示暂无用户组', async () => {
    const wrapper = mount(GroupsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无用户组')
  })

  it('列表展示组名与节点数', async () => {
    ;(listGroups as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: '研发组', node_count: 3, user_count: 5 },
    ])
    const wrapper = mount(GroupsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('研发组')
    expect(wrapper.text()).toContain('3')
  })
})
