// protocol-field-editor.spec.ts：协议对象结构化编辑与高级 JSON 兼容入口。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { Select } from 'ant-design-vue'
import ProtocolFieldEditor from '@/components/ProtocolFieldEditor.vue'
import type { FieldSchema } from '@/api/node'

const objectField: FieldSchema = {
  name: 'ws-opts',
  type: 'object',
  required: false,
  label: 'WebSocket 参数',
  section: 'transport',
  object_kind: 'fields',
  allow_unknown: false,
  properties: [
    { name: 'path', type: 'text', required: false, label: '路径' },
    { name: 'headers', type: 'object', required: false, label: '请求头', object_kind: 'map', allow_unknown: true },
  ],
}

const stringMapField: FieldSchema = {
  name: 'plugin-opts', type: 'object', required: false, label: '自定义插件参数',
  object_kind: 'map', map_value_type: 'string', allow_unknown: true,
}

describe('ProtocolFieldEditor', () => {
  async function customEntryVisible(allowCustom?: boolean | null): Promise<boolean> {
    const field: FieldSchema = {
      name: 'security', type: 'select', required: true, label: '安全',
      option_items: [{ value: 'none', label: '无' }, { value: 'tls', label: 'TLS' }],
      ...(allowCustom === undefined ? {} : { allow_custom: allowCustom }),
    }
    const wrapper = mount(ProtocolFieldEditor, { props: { field, modelValue: 'none' }, attachTo: document.body })
    await wrapper.find('.ant-select-selector').trigger('mousedown')
    const visible = document.body.textContent?.includes('其他（自定义）') ?? false
    wrapper.unmount()
    return visible
  }

  it('只有 allow_custom=true 才开放自定义值', async () => {
    expect(await customEntryVisible(true)).toBe(true)
    expect(await customEntryVisible(false)).toBe(false)
    expect(await customEntryVisible(undefined)).toBe(false)
    expect(await customEntryVisible(null)).toBe(false)
  })

  it('标量字段只渲染所属类型的编辑入口', () => {
    const recommended = mount(ProtocolFieldEditor, {
      props: {
        field: {
          name: 'security', type: 'select', required: true, label: '安全',
          option_items: [{ value: 'none', label: '无' }, { value: 'tls', label: 'TLS' }],
          allow_custom: false,
        },
        modelValue: 'none',
      },
    })
    expect(recommended.findAll('input')).toHaveLength(1)

    const number = mount(ProtocolFieldEditor, {
      props: { field: { name: 'port', type: 'number', required: true, label: '端口' }, modelValue: 443 },
    })
    expect(number.findAll('.ant-input-number')).toHaveLength(1)
    expect(number.findAll('input')).toHaveLength(1)

    const list = mount(ProtocolFieldEditor, {
      props: { field: { name: 'alpn', type: 'text-list', required: false, label: 'ALPN' }, modelValue: ['h2', 'http/1.1'] },
    })
    expect(list.findAll('.protocol-list-editor input')).toHaveLength(2)
    expect(list.findAll('input')).toHaveLength(2)

    const emptyList = mount(ProtocolFieldEditor, {
      props: { field: { name: 'alpn', type: 'text-list', required: false, label: 'ALPN' }, modelValue: [] },
    })
    expect(emptyList.findAll('input')).toHaveLength(0)
  })

  it('可选数字保持未设置为空，显式 0 仍显示为 0，清空不写入 0', async () => {
    const field: FieldSchema = { name: 'mtu', type: 'number', required: false, label: 'MTU' }
    const unset = mount(ProtocolFieldEditor, { props: { field, modelValue: undefined } })
    expect((unset.find('input').element as HTMLInputElement).value).toBe('')
    await unset.findComponent({ name: 'AInputNumber' }).vm.$emit('change', null)
    expect(unset.emitted('update:modelValue')).toEqual([[undefined]])

    const zero = mount(ProtocolFieldEditor, { props: { field, modelValue: 0 } })
    expect((zero.find('input').element as HTMLInputElement).value).toBe('0')
  })

  it('整数列表新增先进入草稿，合法整数应用后才写入且不会自动生成 0', async () => {
    const field: FieldSchema = { name: 'reserved', type: 'int-list', required: false, label: '保留字节' }
    const wrapper = mount(ProtocolFieldEditor, { props: { field, modelValue: [] } })
    await wrapper.findAll('button').find((button) => button.text() === '新增条目')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).toContain('列表项草稿未应用')
    await wrapper.findComponent({ name: 'AInputNumber' }).vm.$emit('change', 7)
    await wrapper.findAll('button').find((button) => button.text() === '应用条目')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[[7]]])
  })

  it('列表推荐项可直接追加，其他条目仍需显式应用', async () => {
    const field: FieldSchema = {
      name: 'alpn', type: 'text-list', required: false, label: 'ALPN', allow_custom: true,
      option_items: [{ value: 'h2', label: 'h2' }, { value: 'http/1.1', label: 'http/1.1' }],
    }
    const recommended = mount(ProtocolFieldEditor, { props: { field, modelValue: [] } })
    await recommended.findComponent(Select).vm.$emit('change', 'h2')
    expect(recommended.emitted('update:modelValue')).toEqual([[['h2']]])

    const custom = mount(ProtocolFieldEditor, { props: { field, modelValue: [] } })
    await custom.findComponent(Select).vm.$emit('change', '__vpn_sub_list_custom_value__')
    expect(custom.emitted('update:modelValue')).toBeUndefined()
    await custom.find('.scalar-list-draft input').setValue('custom-alpn')
    await custom.findAll('button').find((button) => button.text() === '应用条目')!.trigger('click')
    expect(custom.emitted('update:modelValue')).toEqual([[['custom-alpn']]])
  })

  it('嵌套高级区默认折叠且有摘要，折叠不丢失参数或 JSON 草稿', async () => {
    const field: FieldSchema = { ...objectField, properties: [...objectField.properties!,
      { name: 'early', type: 'object', object_kind: 'fields', label: 'Early Data', required: false, advanced: true,
        properties: [{ name: 'limit', type: 'number', label: '上限', required: false }] },
      { name: 'upgrade', type: 'bool', label: 'Upgrade', required: false, advanced: true },
    ] }
    const wrapper = mount(ProtocolFieldEditor, { props: { field, centralizedSwitches: true, modelValue: { path: '/keep', early: { limit: 7 } } } })
    expect(wrapper.find('.ant-switch').exists()).toBe(false)
    const details = wrapper.find('.protocol-object-advanced')
    expect((details.element as HTMLDetailsElement).open).toBe(false)
    expect(details.text()).toContain('已配置 1 项')
    ;(details.element as HTMLDetailsElement).open = true
    const nested = wrapper.findAllComponents(ProtocolFieldEditor).find((item) => item.props('path') === 'ws-opts.early')!
    await nested.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await nested.find('textarea').setValue('{"limit":9}')
    ;(details.element as HTMLDetailsElement).open = false
    await wrapper.vm.$nextTick()
    ;(details.element as HTMLDetailsElement).open = true
    expect(nested.find('textarea').element.value).toBe('{"limit":9}')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('编辑模式使用按钮且切回结构化不清空已应用数据', async () => {
    const value = { path: '/keep', future: 7 }
    const wrapper = mount(ProtocolFieldEditor, { props: { field: objectField, modelValue: value } })
    expect(wrapper.find('.protocol-editor-mode').attributes('role')).toBe('group')
    expect(wrapper.find('.protocol-editor-mode .ant-switch').exists()).toBe(false)
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await wrapper.find('textarea').setValue('{"path":"/draft"}')
    await wrapper.findAll('button').find((button) => button.text() === '结构化编辑')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(value).toEqual({ path: '/keep', future: 7 })
    wrapper.unmount()
  })

  it('无关功能重置不丢弃本对象 JSON 草稿，重叠范围重置才丢弃', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: objectField, modelValue: { path: '/keep' } } })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await wrapper.find('textarea').setValue('{"path":"/draft"}')
    await wrapper.setProps({ jsonResetVersions: { smux: 1 } })
    expect(wrapper.text()).toContain('JSON 草稿未应用')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toContain('/draft')
    await wrapper.setProps({ jsonResetVersions: { smux: 1, 'ws-opts.path': 1 }, modelValue: {} })
    expect(wrapper.text()).not.toContain('JSON 草稿未应用')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('{}')
    wrapper.unmount()
  })
  it('固定对象默认结构化并明确提示未声明参数需删除', () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field: objectField,
        modelValue: { path: '/ws', future: { enabled: true } },
      },
    })

    expect(wrapper.text()).toContain('结构化编辑')
    expect(wrapper.text()).toContain('路径')
    expect(wrapper.text()).toContain('检测到 1 个未声明参数')
    expect(wrapper.text()).toContain('请在高级 JSON 中显式删除')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('固定对象高级 JSON 拒绝未知键并定位字段路径，删除后可应用', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: objectField, modelValue: { path: '/old' } } })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue(JSON.stringify({ path: '/ws', future: true }))
    expect(wrapper.text()).toContain('ws-opts.future')
    await wrapper.findAll('button').find((button) => button.text().replace(/\s/g, '').includes('应用'))!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await textarea.setValue(JSON.stringify({ path: '/ws' }))
    await wrapper.findAll('button').find((button) => button.text().replace(/\s/g, '').includes('应用'))!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[{ path: '/ws' }]])
  })

  it('开放 Map 高级 JSON 允许新增普通键，字符串 Map 仍拒绝非法类型', async () => {
    const headers: FieldSchema = {
      name: 'headers', type: 'object', required: false, label: '请求头',
      object_kind: 'map', allow_unknown: true,
    }
    const mapWrapper = mount(ProtocolFieldEditor, { props: { field: headers, modelValue: { Host: 'old.example.com' } } })
    await mapWrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await mapWrapper.find('textarea').setValue(JSON.stringify({ Host: 'new.example.com', 'X-Custom': 7 }))
    await mapWrapper.findAll('button').find((button) => button.text().replace(/\s/g, '').includes('应用'))!.trigger('click')
    expect(mapWrapper.emitted('update:modelValue')).toEqual([[{ Host: 'new.example.com', 'X-Custom': 7 }]])
    mapWrapper.unmount()

    const stringMapWrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: { flag: '' } } })
    await stringMapWrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await stringMapWrapper.find('textarea').setValue(JSON.stringify({ flag: '', bad: 1 }))
    expect(stringMapWrapper.text()).toContain('plugin-opts.bad 的值必须为字符串')
    await stringMapWrapper.findAll('button').find((button) => button.text().replace(/\s/g, '').includes('应用'))!.trigger('click')
    expect(stringMapWrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('后代 JSON 草稿存在时阻止父对象切换高级 JSON 并稳定定位首个后代', async () => {
    const field: FieldSchema = {
      ...objectField,
      properties: [
        ...objectField.properties!,
        { name: 'early', type: 'object', required: false, label: 'Early Data', object_kind: 'fields', allow_unknown: false,
          properties: [{ name: 'limit', type: 'number', required: false, label: '上限' }] },
      ],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field, modelValue: { path: '/ws', early: { limit: 1 } }, jsonDirtyPaths: ['ws-opts.headers', 'ws-opts.early'] },
    })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.emitted('advanced-json-blocked')).toEqual([[{ path: 'ws-opts', blockedBy: 'ws-opts.early' }]])
    const childPaths = wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('path'))
    expect(childPaths).toContain('ws-opts.early')
    expect(childPaths).toContain('ws-opts.headers')
  })

  it('三层后代 JSON 草稿同样阻止顶层切换并定位最深层路径', async () => {
    const field: FieldSchema = {
      ...objectField,
      properties: [
        ...objectField.properties!,
        { name: 'early', type: 'object', required: false, label: 'Early Data', object_kind: 'fields', allow_unknown: false,
          properties: [{ name: 'deep', type: 'object', required: false, label: 'Deep', object_kind: 'fields', allow_unknown: false, properties: [] }] },
      ],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field, modelValue: { path: '/ws', early: { deep: {} } }, jsonDirtyPaths: ['ws-opts.early.deep'] },
    })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.emitted('advanced-json-blocked')).toEqual([[{ path: 'ws-opts', blockedBy: 'ws-opts.early.deep' }]])
  })

  it('父路径与 foo-bar 不构成后代匹配，不误阻断高级 JSON', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/ws' }, jsonDirtyPaths: ['ws-opts-extra'] },
    })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    expect(wrapper.find('textarea').exists()).toBe(true)
    expect(wrapper.emitted('advanced-json-blocked')).toBeUndefined()
  })

  it('高级 JSON 解析失败时发出带路径的无效状态', async () => {
    const wrapper = mount(ProtocolFieldEditor, {
      props: { field: objectField, modelValue: { path: '/ws' } },
    })

    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
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
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
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
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
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
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
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

  it('递归敏感字段只按服务端已保存路径显示状态并上报替换/清空', async () => {
    const field: FieldSchema = {
      name: 'shadow-tls-opts', type: 'object', required: false, label: 'shadow-tls', object_kind: 'fields',
      properties: [{ name: 'password', type: 'password', required: false, label: '密码' }],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field,
        modelValue: { password: '' },
        sensitivePaths: ['shadow-tls-opts.password'],
        savedSensitivePaths: [],
      },
    })
    expect(wrapper.find('input').attributes('placeholder')).toBe('未配置')
    expect(wrapper.text()).toContain('未配置')
    await wrapper.find('input').setValue('new-secret')
    await wrapper.setProps({ modelValue: { password: 'new-secret' } })
    expect(wrapper.text()).toContain('待替换')
    await wrapper.find('input').setValue('')
    const events = wrapper.emitted('credential-change') ?? []
    expect(events[events.length - 1]).toEqual([{ path: 'shadow-tls-opts.password', value: '' }])

    await wrapper.setProps({ modelValue: { password: '' }, savedSensitivePaths: ['shadow-tls-opts.password'] })
    expect(wrapper.find('input').attributes('placeholder')).toBe('已保存（留空保留）')
  })

  it('对象数组使用稳定条目 ID 传递完整凭据路径，重排后状态不串项', async () => {
    const firstID = '11111111-1111-4111-8111-111111111111'
    const secondID = '22222222-2222-4222-8222-222222222222'
    const peers: FieldSchema = {
      name: 'peers', type: 'object', required: false, label: 'Peer 列表', object_kind: 'list', item_id_field: '_credential_id',
      properties: [{ name: 'pre-shared-key', type: 'password', required: false, label: '预共享密钥' }],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field: peers,
        modelValue: [
          { _credential_id: secondID, 'pre-shared-key': '' },
          { _credential_id: firstID, 'pre-shared-key': '' },
        ],
        sensitivePaths: ['peers[].pre-shared-key'],
        savedSensitivePaths: [`peers[${firstID}].pre-shared-key`],
      },
    })
    const editors = wrapper.findAllComponents(ProtocolFieldEditor).filter((item) => item.props('field').name === 'pre-shared-key')
    expect(editors.map((item) => item.props('path'))).toEqual([
      `peers[${secondID}].pre-shared-key`,
      `peers[${firstID}].pre-shared-key`,
    ])
    expect(editors[0].find('input').attributes('placeholder')).toBe('未配置')
    expect(editors[1].find('input').attributes('placeholder')).toBe('已保存（留空保留）')
  })

  it('对象数组高级 JSON 放行内部 item_id_field，重排后保留稳定身份', async () => {
    const firstID = '11111111-1111-4111-8111-111111111111'
    const secondID = '22222222-2222-4222-8222-222222222222'
    const peers: FieldSchema = {
      name: 'peers', type: 'object', required: false, label: 'Peer 列表', object_kind: 'list',
      item_id_field: '_credential_id', allow_unknown: false,
      properties: [
        { name: 'server', type: 'text', required: false, label: '服务器' },
        { name: 'pre-shared-key', type: 'password', required: false, label: '预共享密钥' },
      ],
    }
    const wrapper = mount(ProtocolFieldEditor, {
      props: {
        field: peers,
        modelValue: [
          { _credential_id: firstID, server: 'peer-a', 'pre-shared-key': '' },
          { _credential_id: secondID, server: 'peer-b', 'pre-shared-key': '' },
        ],
      },
    })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toContain(firstID)
    await textarea.setValue(JSON.stringify([
      { _credential_id: secondID, server: 'peer-b', 'pre-shared-key': '' },
      { _credential_id: firstID, server: 'peer-a', 'pre-shared-key': '' },
    ]))
    expect(wrapper.text()).not.toContain('未在协议注册表中声明')
    await wrapper.findAll('button').find((button) => button.text().replace(/\s/g, '') === '应用')!.trigger('click')
    const updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1][0]).toEqual([
      { _credential_id: secondID, server: 'peer-b', 'pre-shared-key': '' },
      { _credential_id: firstID, server: 'peer-a', 'pre-shared-key': '' },
    ])
  })

  it('对象数组高级 JSON 仍拒绝 item_id_field 之外的未知键', async () => {
    const id = '11111111-1111-4111-8111-111111111111'
    const peers: FieldSchema = {
      name: 'peers', type: 'object', required: false, label: 'Peer 列表', object_kind: 'list',
      item_id_field: '_credential_id', allow_unknown: false,
      properties: [{ name: 'server', type: 'text', required: false, label: '服务器' }],
    }
    const wrapper = mount(ProtocolFieldEditor, { props: { field: peers, modelValue: [{ _credential_id: id, server: 'peer' }] } })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await wrapper.find('textarea').setValue(JSON.stringify([{ _credential_id: id, server: 'peer', future: true }]))
    expect(wrapper.text()).toContain('peers[0].future')
    const apply = wrapper.find('button.ant-btn-sm.ant-btn-primary')
    expect((apply.element as HTMLButtonElement).disabled).toBe(true)
  })

  it('对象数组可新增条目并按子 schema 编辑', async () => {
    const peers: FieldSchema = {
      name: 'peers', type: 'object', required: false, label: 'Peer 列表', object_kind: 'list', allow_unknown: false,
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
      allow_unknown: false,
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

  it('字符串映射结构化编辑支持新增、改名、删除、空 flag 与特殊字符', async () => {
    const original = { mode: 'custom', flag: '', special: ':;=\\' }
    const wrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: original } })
    expect(wrapper.findAll('.protocol-map-entry')).toHaveLength(3)
    expect(wrapper.findAll('.protocol-map-entry .ant-switch')).toHaveLength(0)
    expect(wrapper.findAll('.protocol-map-entry .ant-input-number')).toHaveLength(0)
    expect(wrapper.findAll('.protocol-map-entry textarea')).toHaveLength(0)

    await wrapper.findAll('button').find((button) => button.text() === '新增参数')!.trigger('click')
    let updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1]).toEqual([{ ...original, '参数4': '' }])
    await wrapper.setProps({ modelValue: updates[updates.length - 1][0] })

    const firstName = wrapper.findAll('input[aria-label="参数名"]')[0]
    await firstName.setValue('renamed')
    updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1][0]).toEqual({ renamed: 'custom', flag: '', special: ':;=\\', '参数4': '' })
    await wrapper.setProps({ modelValue: updates[updates.length - 1][0] })
    await wrapper.findAll('.protocol-map-entry')[3].find('button').trigger('click')
    updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1][0]).toEqual({ renamed: 'custom', flag: '', special: ':;=\\' })
  })

  it('父级即时回写时逐字符改名保持同一输入框与焦点', async () => {
    const Host = defineComponent({
      components: { ProtocolFieldEditor },
      setup() {
        return { field: stringMapField, value: ref<Record<string, string>>({ mode: 'custom' }) }
      },
      template: '<ProtocolFieldEditor :field="field" v-model="value" />',
    })
    const wrapper = mount(Host, { attachTo: document.body })
    const editor = wrapper.findComponent(ProtocolFieldEditor)
    const originalInput = editor.find('input[aria-label="参数名"]').element
    ;(originalInput as HTMLInputElement).focus()

    for (const name of ['m', 'mo', 'mod', 'mode-next']) {
      await editor.find('input[aria-label="参数名"]').setValue(name)
      await nextTick()
      const currentInput = editor.find('input[aria-label="参数名"]').element
      expect(currentInput).toBe(originalInput)
      expect(document.activeElement).toBe(originalInput)
    }

    expect((wrapper.vm as unknown as { value: Record<string, string> }).value).toEqual({ 'mode-next': 'custom' })
    wrapper.unmount()
  })

  it('结构化参数名为空或重复时显示行内错误并阻止保存，修正后清除错误', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: { mode: 'custom', host: 'cdn.example.com' } } })
    const nameInputs = () => wrapper.findAll('input[aria-label="参数名"]')
    await nameInputs()[0].setValue('')
    expect(wrapper.text()).toContain('参数名不能为空')
    expect(nameInputs()[0].attributes('aria-invalid')).toBe('true')
    let validity = wrapper.emitted('validity-change') ?? []
    expect(validity[validity.length - 1]).toEqual([{ path: 'plugin-opts.mode', valid: false }])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await nameInputs()[0].setValue('host')
    expect(wrapper.text()).toContain('参数名重复')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await nameInputs()[0].setValue('transport-mode')
    expect(wrapper.text()).not.toContain('参数名重复')
    validity = wrapper.emitted('validity-change') ?? []
    expect(validity).toContainEqual([{ path: 'plugin-opts.mode', valid: true }])
    const updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1][0]).toEqual({ 'transport-mode': 'custom', host: 'cdn.example.com' })
  })

  it('切到高级 JSON 会放弃未提交的非法改名草稿并清除旧错误路径', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: { mode: 'custom' } } })
    await wrapper.find('input[aria-label="参数名"]').setValue('')
    expect(wrapper.text()).toContain('参数名不能为空')
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    expect(wrapper.text()).not.toContain('参数名不能为空')
    expect(wrapper.find('textarea').element.value).toContain('"mode"')
    expect(wrapper.emitted('validity-change')).toContainEqual([{ path: 'plugin-opts.mode', valid: true }])
  })

  it.each([
    ['数组', '[]', '请输入 JSON 对象', 'plugin-opts'],
    ['对象值', '{"mode":{"nested":"x"}}', 'plugin-opts.mode 的值必须为字符串', 'plugin-opts.mode'],
    ['数组值', '{"mode":["x"]}', 'plugin-opts.mode 的值必须为字符串', 'plugin-opts.mode'],
    ['数字值', '{"mode":1}', 'plugin-opts.mode 的值必须为字符串', 'plugin-opts.mode'],
    ['布尔值', '{"mode":true}', 'plugin-opts.mode 的值必须为字符串', 'plugin-opts.mode'],
  ])('字符串映射高级 JSON 拒绝%s并精确上报路径', async (_label, draft, message, path) => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: { mode: 'custom' } } })
    const modeButtons = wrapper.findAll('button')
    await modeButtons[1].trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue(draft)
    expect(wrapper.text()).toContain(message)
    const apply = wrapper.find('button.ant-btn-sm.ant-btn-primary')
    expect((apply.element as HTMLButtonElement).disabled).toBe(true)
    const validityEvents = wrapper.emitted('validity-change') ?? []
    expect(validityEvents[validityEvents.length - 1]).toEqual([{ path, valid: false }])
  })

  it('字符串映射高级 JSON 拒绝空参数名，保留空字符串值与特殊字符', async () => {
    const wrapper = mount(ProtocolFieldEditor, { props: { field: stringMapField, modelValue: {} } })
    await wrapper.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    const textarea = wrapper.find('textarea')
    await textarea.setValue('{"":"value"}')
    expect(wrapper.text()).toContain('plugin-opts 参数名不能为空')
    expect((wrapper.find('button.ant-btn-sm.ant-btn-primary').element as HTMLButtonElement).disabled).toBe(true)
    let validity = wrapper.emitted('validity-change') ?? []
    expect(validity[validity.length - 1]).toEqual([{ path: 'plugin-opts', valid: false }])

    await textarea.setValue(JSON.stringify({ flag: '', special: ':;=\\' }))
    expect(wrapper.text()).not.toContain('参数名不能为空')
    await wrapper.find('button.ant-btn-sm.ant-btn-primary').trigger('click')
    const updates = wrapper.emitted('update:modelValue') ?? []
    expect(updates[updates.length - 1]).toEqual([{ flag: '', special: ':;=\\' }])
    validity = wrapper.emitted('validity-change') ?? []
    expect(validity).toContainEqual([{ path: 'plugin-opts', valid: true }])
  })

  it('组件卸载不会为仍有效的高级 JSON 草稿发出清除事件', async () => {
    const events: Array<{ path: string; dirty: boolean }> = []
    const Host = defineComponent({
      components: { ProtocolFieldEditor },
      setup() {
        return {
          field: objectField,
          value: ref({ path: '/keep' }),
          show: ref(true),
          onJsonDirty: (payload: { path: string; dirty: boolean }) => events.push(payload),
        }
      },
      template: '<ProtocolFieldEditor v-if="show" :field="field" v-model="value" @json-dirty-change="onJsonDirty" />',
    })
    const host = mount(Host)
    const editor = host.findComponent(ProtocolFieldEditor)
    await editor.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await editor.find('textarea').setValue('{"path":"/draft"}')
    expect(events[events.length - 1]).toEqual({ path: 'ws-opts', dirty: true })
    const before = events.length
    ;(host.vm as any).show = false
    await nextTick()
    expect(events).toHaveLength(before)
    host.unmount()
  })
})
