// rules-view.spec.ts：规则列表移除客户端类型列，首页默认统一收口到操作区（R24-09/R24-10）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

vi.mock('@/api/rule', () => ({
  listAdminRules: vi.fn(),
  createRule: vi.fn(),
  renameRule: vi.fn(),
  deleteRule: vi.fn(),
  refreshRuleToken: vi.fn(),
  setHomeDefault: vi.fn(),
}))
vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import RulesView from '@/views/admin/RulesView.vue'
import { listAdminRules } from '@/api/rule'

const rules = [
  { id: 1, slug: 'rule-a', name: '规则A', client_type: 'shadowrocket', schemes: [], token: 'tk', current_version: 1, is_home_default: true, created_at: '', refreshed_at: null },
  { id: 2, slug: 'rule-b', name: '规则B', client_type: 'shadowrocket', schemes: [], token: 'tk', current_version: 0, is_home_default: false, created_at: '', refreshed_at: null },
]

function router() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: RulesView },
      { path: '/admin/rules/:id/versions', component: { template: '<div />' } },
      { path: '/admin/assembly', component: { template: '<div />' } },
    ],
  })
}

describe('RulesView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(listAdminRules as unknown as ReturnType<typeof vi.fn>).mockResolvedValue(rules)
  })

  it('列表不渲染客户端类型列与首页默认单选列，默认操作收口到操作区', async () => {
    const appRouter = router()
    await appRouter.push('/')
    await appRouter.isReady()
    const wrapper = mount(RulesView, { global: { plugins: [appRouter] } })
    await flushPromises()

    const headers = wrapper.findAll('th').map((cell) => cell.text())
    expect(headers).not.toContain('客户端类型')
    expect(headers).not.toContain('首页默认')

    const buttons = wrapper.findAll('button').map((button) => button.text())
    expect(buttons.filter((text) => text === '设为首页默认')).toHaveLength(2)
    expect(buttons.filter((text) => text === '取消默认')).toHaveLength(2)
    expect(wrapper.find('input[type="radio"]').exists()).toBe(false)
  })
})
