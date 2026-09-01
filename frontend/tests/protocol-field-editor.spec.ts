// protocol-field-editor.spec.ts：协议对象结构化编辑与高级 JSON 兼容入口。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ProtocolFieldEditor from '@/components/ProtocolFieldEditor.vue'
import type { FieldSchema } from '@/api/node'

const objectField: FieldSchema = {
  name: 'ws-opts',
  type: 'object',
  required: false,
  label: 'WebSocket 参数',
  section: 'transport',
  object_kind: 'fields',
  allow_unknown: true,
  properties: [
    { name: 'path', type: 'text', required: false, label: '路径' },
    { name: 'headers', type: 'object', required: false, label: '请求头', object_kind: 'map', allow_unknown: true },
  ],
}

describe('ProtocolFieldEditor', () => {
  it('固定对象默认结构化并保留未知参数提示', () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field: objectField,
        modelValue: { path: '/ws', future: { enabled: true } },
      },
    })

    expect(wrapper.text()).toContain('结构化编辑')
    expect(wrapper.text()).toContain('路径')
    expect(wrapper.text()).toContain('已保留 1 个未识别参数')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('高级 JSON 解析失败时发出带路径的无效状态', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/ws' } },
    })

    await wrapper.find('.ant-switch').trigger('click')
    const textarea = wrapper.find('textarea')
    expect(textarea.exists()).toBe(true)
    await textarea.setValue('{')

    expect(wrapper.text()).toContain('请输入 JSON 对象')
    const validityEvents = wrapper.emitted('validity-change') ?? []
    expect(validityEvents[validityEvents.length - 1]).toEqual([{ path: 'ws-opts', valid: false }])
  })

  it('对象数组可新增条目并按子 schema 编辑', async () => {
    const peers: FieldSchema = {
      name: 'peers', type: 'object', required: false, label: 'Peer 列表', object_kind: 'list', allow_unknown: true,
      properties: [{ name: 'server', type: 'text', required: false, label: '服务器' }],
    }
    const wrapper = mount(ProtocolFieldEditor, { props: { field: peers, modelValue: [] } })

    const buttons = wrapper.findAll('button')
    await buttons[buttons.length - 1].trigger('click')
    const updateEvents = wrapper.emitted('update:modelValue') ?? []
    expect(updateEvents[updateEvents.length - 1]).toEqual([[{}]])
  })
})
