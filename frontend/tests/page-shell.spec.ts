// page-shell.spec.ts：管理页骨架只输出一个页面级 h1，并代理四态容器。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import PageShell from '@/components/PageShell.vue'

function router() {
  return createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }] })
}

describe('PageShell', () => {
  it('使用唯一 h1 并展示业务内容', () => {
    const wrapper = mount(PageShell, {
      props: { title: '节点管理', description: '管理可用于订阅的节点。' },
      slots: { default: '<p>节点列表</p>' },
      global: { plugins: [router()] },
    })
    expect(wrapper.findAll('h1')).toHaveLength(1)
    expect(wrapper.find('h1').text()).toBe('节点管理')
    expect(wrapper.text()).toContain('节点列表')
  })

  it('空数据时提供统一空态', () => {
    const wrapper = mount(PageShell, {
      props: { title: '规则', empty: true, emptyTitle: '还没有规则', emptyText: '请先创建一条规则。' },
      global: { plugins: [router()] },
    })
    expect(wrapper.text()).toContain('还没有规则')
    expect(wrapper.text()).toContain('请先创建一条规则。')
  })
})
