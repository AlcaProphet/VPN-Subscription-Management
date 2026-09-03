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
  it('无关功能重置不丢弃本对象 JSON 草稿，重叠范围重置才丢弃', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: objectField, modelValue: { path: '/keep' } } })
    await wrapper.find('.ant-switch').trigger('click')
    await wrapper.find('textarea').setValue('{"path":"/draft"}')
    await wrapper.setProps({ jsonResetVersions: { smux: 1 } })
    expect(wrapper.text()).toContain('JSON 草稿未应用')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toContain('/draft')
    await wrapper.setProps({ jsonResetVersions: { smux: 1, 'ws-opts.path': 1 }, modelValue: {} })
    expect(wrapper.text()).not.toContain('JSON 草稿未应用')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('{}')
    wrapper.unmount()
  })
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
    expect(wrapper.text()).toContain('高级 JSON')
    const validityEvents = wrapper.emitted('validity-change') ?? []
    expect(validityEvents[validityEvents.length - 1]).toEqual([{ path: 'ws-opts', valid: false }])
  })

  it('高级 JSON 草稿未应用前不修改模型值，点击应用后才回写', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/old' } },
    })
    await wrapper.find('.ant-switch').trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('{"path":"/new"}')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).toContain('JSON 草稿未应用')

    const buttons = wrapper.findAll('button')
    const applyButton = buttons.find((button) => button.text().replace(/\s/g, '').includes('应用'))
    expect(applyButton).toBeTruthy()
    await applyButton!.trigger('click')

    const updateEvents = wrapper.emitted('update:modelValue') ?? []
    expect(updateEvents[updateEvents.length - 1]).toEqual([{ path: '/new' }])
    expect(wrapper.text()).not.toContain('JSON 草稿未应用')
  })

  it('高级 JSON 放弃按钮恢复原结构化值', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/keep' } },
    })
    await wrapper.find('.ant-switch').trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('{"path":"/draft"}')

    const buttons = wrapper.findAll('button')
    const discardButton = buttons.find((button) => button.text().replace(/\s/g, '').includes('放弃'))
    expect(discardButton).toBeTruthy()
    await discardButton!.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect((textarea.element as HTMLTextAreaElement).value).toContain('/keep')
  })

  it('高级 JSON 草稿变化发出 json-dirty-change，应用后恢复未脏', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/old' } },
    })
    await wrapper.find('.ant-switch').trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('{"path":"/new"}')

    const dirtyEvents = wrapper.emitted('json-dirty-change') ?? []
    expect(dirtyEvents[dirtyEvents.length - 1]).toEqual([{ path: 'ws-opts', dirty: true }])

    const buttons = wrapper.findAll('button')
    const applyButton = buttons.find((button) => button.text().replace(/\s/g, '').includes('应用'))
    await applyButton!.trigger('click')

    const appliedEvents = wrapper.emitted('json-dirty-change') ?? []
    expect(appliedEvents[appliedEvents.length - 1]).toEqual([{ path: 'ws-opts', dirty: false }])
  })

  it('敏感字段按凭据状态显示已保存或未配置', async () => {
    const passwordField: FieldSchema = {
      name: 'password', type: 'password', required: true, label: '密码',
    }
    const saved = mount(ProtocolFieldEditor, {
      props: { field: passwordField, modelValue: '', sensitivePaths: ['password'], credentialState: 'saved' },
    })
    expect(saved.find('input').attributes('placeholder')).toBe('已保存（留空保留）')

    const cleared = mount(ProtocolFieldEditor, {
      props: { field: passwordField, modelValue: '', sensitivePaths: ['password'], credentialState: 'cleared' },
    })
    expect(cleared.find('input').attributes('placeholder')).toBe('未配置')
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

  it('对象子字段按当前插件状态递归执行 when', async () => {
    const pluginField: FieldSchema = {
      name: 'plugin-opts',
      type: 'object',
      required: false,
      label: '插件参数',
      object_kind: 'fields',
      allow_unknown: true,
      properties: [
        { name: 'mode', type: 'select', required: false, label: '模式', when: { plugin: ['obfs'] } },
        { name: 'host', type: 'text', required: false, label: 'Host', when: { plugin: ['obfs', 'v2ray-plugin'] } },
        { name: 'tls', type: 'bool', required: false, label: 'TLS', when: { plugin: ['v2ray-plugin'] } },
      ],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field: pluginField,
        modelValue: { mode: 'http', host: 'cdn.example.com' },
        currentState: { plugin: 'obfs' },
      },
    })

    expect(wrapper.text()).toContain('模式')
    expect(wrapper.text()).toContain('Host')
    expect(wrapper.text()).not.toContain('TLS')
    wrapper.unmount()
  })
})
