// nodes-groups-step.spec.ts：Clash 强制组成员选择契约（R24-18）
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import NodesGroupsStep from '@/views/admin/assembly/NodesGroupsStep.vue'

const ButtonStub = {
  props: ['disabled'],
  emits: ['click'],
  template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
}
const CheckboxStub = {
  props: ['checked', 'disabled'],
  emits: ['change'],
  template: '<label><input type="checkbox" :checked="checked" :disabled="disabled" @change="$emit(\'change\')"><slot /></label>',
}
const AppModalStub = {
  props: ['open', 'title'],
  template: '<section v-if="open" data-testid="member-modal"><h2>{{ title }}</h2><slot /></section>',
}

function mountStep(overrides: any = {}) {
  const node = (id: number, name: string, renderName: string) => ({
    id, name, render_name: renderName, source: 'manual' as const, protocol: 'vless',
    host: 'example.com', port: 443, protocol_json: {}, is_public: false,
    enabled: true, allocatable: true, missing: false,
  })
  const baseProps = {
    form: {
      node_names: ['node-a'],
      group_names: [],
      overseas_members: [],
      fallback_group_members: ['🚀直接连接', '🌎国外流量'],
    },
    groupNodeOrders: {},
    groupMemberOrders: {},
    context: null,
    targetSyntax: 'clash-yaml',
    invalidRefs: [],
    showXray: true,
    manualNodes: [
      node(1, 'node-a', '节点A'),
      node(2, 'node-b', '节点B'),
    ],
    xrayNodes: [],
    presetGroups: [],
    customGroups: [],
  }
  return mount(NodesGroupsStep, {
    props: { ...baseProps, ...overrides },
    global: {
      stubs: {
        Button: ButtonStub,
        Checkbox: CheckboxStub,
        Tag: { template: '<span><slot /></span>' },
        AppModal: AppModalStub,
      },
    },
  })
}

describe('NodesGroupsStep Clash 强制组', () => {
  it('在组旁明确展示各自可选成员范围', () => {
    const wrapper = mountStep()
    expect(wrapper.get('[data-testid="force-overseas-group"]').text()).toContain('可选成员：本次已勾选节点、🚀直接连接')
    expect(wrapper.get('[data-testid="force-fallback-group"]').text()).toContain('可选成员：本次已勾选节点、🚀直接连接、🌎国外流量')
  })

  it('成员弹窗只展示已勾选节点和允许引用的强制组，不展示底层 DIRECT', async () => {
    const wrapper = mountStep()
    await wrapper.get('[data-testid="select-overseas-members"]').trigger('click')
    let modal = wrapper.get('[data-testid="member-modal"]')
    expect(modal.text()).toContain('节点A')
    expect(modal.text()).not.toContain('节点B')
    expect(modal.text()).toContain('🚀直接连接')
    expect(modal.text()).not.toContain('DIRECT')

    await wrapper.get('[data-testid="select-fallback-members"]').trigger('click')
    modal = wrapper.get('[data-testid="member-modal"]')
    expect(modal.text()).toContain('🚀直接连接')
    expect(modal.text()).toContain('🌎国外流量')
    expect(modal.text()).not.toContain('DIRECT')
  })

  it('保存无法归属组时提交有序成员', async () => {
    const wrapper = mountStep()
    await wrapper.get('[data-testid="select-fallback-members"]').trigger('click')
    await wrapper.get('[data-testid="save-member-selector"]').trigger('click')
    expect(wrapper.emitted('update-force-members')).toEqual([
      ['🛟无法归属的流量', ['🚀直接连接', '🌎国外流量']],
    ])
  })

  it('使用中文分区标题并保留强制组成员能力说明', () => {
    const wrapper = mountStep()
    expect(wrapper.text()).toContain('手动添加的节点')
    expect(wrapper.text()).not.toContain('manual 节点')
    expect(wrapper.text()).toContain('Xray 节点')
    expect(wrapper.text()).toContain('代理组')
  })

  it('已选普通代理组展示默认携带的子组', () => {
    const group = {
      id: 1, name: '影音', type: 'custom' as const, enabled: true,
      definition: { type: 'select' as const, nodes: [], groups: ['🌎国外流量'] },
    }
    const wrapper = mountStep({
      customGroups: [group],
      form: {
        node_names: ['node-a'],
        group_names: ['影音'],
        overseas_members: [],
        fallback_group_members: ['🚀直接连接', '🌎国外流量'],
      },
    })
    expect(wrapper.text()).toContain('默认携带：🌎国外流量')
  })

  it('普通代理组选择排序包含默认子组并提交合并成员顺序', async () => {
    const group = {
      id: 1, name: '影音', type: 'custom' as const, enabled: true,
      definition: { type: 'select' as const, nodes: [], groups: ['🌎国外流量'] },
    }
    const wrapper = mountStep({
      customGroups: [group],
      form: {
        node_names: ['node-a'],
        group_names: ['影音'],
        overseas_members: [],
        fallback_group_members: ['🚀直接连接', '🌎国外流量'],
      },
      groupNodeOrders: { '影音': ['node-a'] },
    })
    await wrapper.get('[data-testid="select-group-影音"]').trigger('click')
    const modal = wrapper.get('[data-testid="member-modal"]')
    expect(modal.text()).toContain('默认携带的代理组')
    expect(modal.text()).toContain('🌎国外流量')
    await wrapper.get('[data-testid="save-member-selector"]').trigger('click')
    expect(wrapper.emitted('update-group-node-order')).toEqual([['影音', ['node-a']]])
    expect(wrapper.emitted('update-group-member-order')).toEqual([['影音', ['node-a', '🌎国外流量']]])
  })

})
