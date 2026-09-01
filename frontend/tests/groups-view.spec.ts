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
import { listGroups, getGroup } from '@/api/group'

describe('GroupsView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.body.innerHTML = ''
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

  it('编辑弹窗渲染候选集、公共节点与排序区', async () => {
    ;(listGroups as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, name: '研发组', is_default: false, node_count: 1, user_count: 0 },
    ])
    ;(getGroup as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 1,
      name: '研发组',
      default_quota: null,
      nodes: [
        { node_id: 1, node_name: 'node-1', render_name: '节点一', is_public: false },
      ],
      candidate_nodes: [
        { node_id: 1, name: 'node-1', render_name: '节点一', is_public: false, in_partial_blueprint: false },
        { node_id: 2, name: 'public-node', render_name: '公共节点', is_public: true, in_partial_blueprint: false },
      ],
    })
    const wrapper = mount(GroupsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    const editBtn = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('编辑'))
    expect(editBtn).toBeTruthy()
    await editBtn!.trigger('click')
    await flushPromises()
    const body = document.body.textContent ?? ''
    expect(body).toContain('节点分配（候选集）')
    expect(body).toContain('分配排序')
    expect(body).toContain('节点一')
    expect(body).toContain('公共·免分配')
    expect(body.replace(/\s/g, '')).toContain('保存')
  })
})
