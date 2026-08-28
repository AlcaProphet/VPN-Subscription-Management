// context-bar.spec.ts：装配跨页草稿提示与过期清理（Build11 Step 2）
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ContextBar from '@/components/ContextBar.vue'
import { ASSEMBLY_CONTEXT_KEY, saveAssemblyDraft } from '@/utils/assemblyDraft'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/nodes', component: ContextBar },
      { path: '/admin/assembly', component: { template: '<div>assembly</div>' } },
    ],
  })
}

function draft() {
  return {
    sourceLabel: 'Clash YAML · 至少一个可用节点',
    returnPath: '/admin/assembly',
    mainTab: 'build',
    subTab: 'clash-yaml' as const,
    currentStep: 1,
    layoutMode: 'step' as const,
    form: {
      rule_name: '', sr_rule_mode: 'new' as const, fixed_params_text: '{}', node_names: [], group_names: [],
      group_node_orders: {}, overseas_members: [], fallback_group_members: ['🚀直接连接', '🌎国外流量'], pools: [], custom_rules: [], final_direction: 'PROXY',
      overlay: { merge_yaml: '', rules_yaml: '', proxies_yaml: '', groups_yaml: '' },
    },
  }
}

describe('ContextBar', () => {
  beforeEach(() => sessionStorage.clear())

  it('展示已保存的装配任务，并可一次返回装配页', async () => {
    saveAssemblyDraft(draft())
    const router = makeRouter()
    await router.push('/admin/nodes')
    await router.isReady()
    const wrapper = mount(ContextBar, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('正在补充构建前置条件')
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/assembly')
  })

  it('过期草稿不展示，并从 sessionStorage 清理', async () => {
    sessionStorage.setItem(ASSEMBLY_CONTEXT_KEY, JSON.stringify({
      ...draft(), version: 1, createdAt: Date.now() - 31 * 60 * 1000, expiresAt: Date.now() - 60 * 1000,
    }))
    const router = makeRouter()
    await router.push('/admin/nodes')
    await router.isReady()
    const wrapper = mount(ContextBar, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toBe('')
    expect(sessionStorage.getItem(ASSEMBLY_CONTEXT_KEY)).toBeNull()
  })
})
