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

function mountStep() {
  const node = (id: number, name: string, renderName: string) => ({
    id, name, render_name: renderName, source: 'manual' as const, protocol: 'vless',
    host: 'example.com', port: 443, protocol_json: {}, is_public: false,
    enabled: true, allocatable: true, missing: false,
  })
  return mount(NodesGroupsStep, {
    props: {
      form: {
        node_names: ['node-a'],
        group_names: [],
        overseas_members: [],
        fallback_group_members: ['🚀直接连接', '🌎国外流量'],
      },
      groupNodeOrders: {},
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
    },
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
})
