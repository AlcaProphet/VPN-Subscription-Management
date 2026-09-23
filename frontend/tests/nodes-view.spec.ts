// nodes-view.spec.ts：节点管理页前端单测（Build5 Step5 item7 / N3）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('@/api/node', () => ({
  NODE_CHECK_TARGETS: ['clash-yaml', 'sr-subs', 'generic-subs'],
  NODE_CHECK_TARGET_LABELS: {
    'clash-yaml': 'clash-yaml（Clash YAML）',
    'sr-subs': 'sr-subs（Shadowrocket 订阅）',
    'generic-subs': 'generic-subs（通用订阅）',
  },
  listNodes: vi.fn(),
  getProtocols: vi.fn(),
  createNode: vi.fn(),
  updateNode: vi.fn(),
  deleteNode: vi.fn(),
  toggleNode: vi.fn(),
  setNodeDisplayName: vi.fn(),
  importNodes: vi.fn(),
  parseOpenVPN: vi.fn(),
}))

vi.mock('@/api/request', () => {
  class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  }
  return { ApiError }
})

vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import NodesView from '@/views/admin/NodesView.vue'
import { listNodes, getProtocols, createNode, updateNode, importNodes, parseOpenVPN, type FieldSchema } from '@/api/node'
import { ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'
import ProtocolFieldEditor from '@/components/ProtocolFieldEditor.vue'
import NodeCheckPanel from '@/components/NodeCheckPanel.vue'
import OpenVPNImportPanel from '@/components/OpenVPNImportPanel.vue'
import { smuxSchema, smuxValue } from './fixtures/smux'

const mockListNodes = listNodes as unknown as ReturnType<typeof vi.fn>
const mockGetProtocols = getProtocols as unknown as ReturnType<typeof vi.fn>
const mockCreateNode = createNode as unknown as ReturnType<typeof vi.fn>
const mockUpdateNode = updateNode as unknown as ReturnType<typeof vi.fn>
const mockImportNodes = importNodes as unknown as ReturnType<typeof vi.fn>
const mockParseOpenVPN = parseOpenVPN as unknown as ReturnType<typeof vi.fn>

const node = {
  id: 1,
  source: 'manual' as const,
  name: 'node-a',
  display_name: null,
  render_name: 'node-a',
  protocol: 'ss',
  host: '1.2.3.4',
  port: 8388,
  protocol_json: {},
  is_public: false,
  enabled: true,
  allocatable: true,
  missing: false,
  edit_revision: 3,
  state_format_version: 1,
  current_state: { security: 'none' },
  extensions: [],
  saved_sensitive_paths: [],
}

const protocols = [
  {
    protocol: 'ss',
    label: 'Shadowsocks',
    form_schema: [
      { name: 'cipher', type: 'text', required: true, label: '加密方式', section: 'transport' },
      { name: 'password', type: 'password', required: true, label: '密码', section: 'auth' },
      {
        name: 'plugin', type: 'select', required: false, label: '插件', group: 'connection', default: '', allow_custom: true,
        reset_on: ['plugin'],
        option_items: [
          { value: '', label: '不使用插件' },
          { value: 'obfs', label: 'obfs' },
          { value: 'v2ray-plugin', label: 'v2ray-plugin' },
          { value: 'shadow-tls', label: 'shadow-tls' },
          { value: 'restls', label: 'restls' },
        ],
      },
      {
        name: 'plugin-opts', type: 'object', required: false, label: '自定义插件参数', group: 'connection',
        object_kind: 'map', map_value_type: 'string', allow_unknown: true, reset_on: ['plugin'],
        when: { plugin_not: ['', 'obfs', 'v2ray-plugin', 'shadow-tls', 'restls'] },
      },
      ...['obfs', 'v2ray-plugin', 'shadow-tls', 'restls'].map((plugin) => ({
        name: `${plugin}-opts`, type: 'object', required: false, label: `${plugin} 参数`, group: 'connection',
        object_kind: 'fields', allow_unknown: false, reset_on: ['plugin'], when: { plugin: [plugin] }, properties: [],
      })),
      { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
      { name: 'routing-mark', type: 'number', required: false, label: '路由标记', section: 'advanced' },
    ],
    sensitive_fields: ['password'],
    link_mappings: { sr: true, generic: true },
  },
]

describe('NodesView 节点管理页', () => {
  beforeEach(() => {
    mockListNodes.mockReset()
    mockGetProtocols.mockReset()
    mockCreateNode.mockReset()
    mockUpdateNode.mockReset()
    mockImportNodes.mockReset()
    ;(Notify.warning as unknown as ReturnType<typeof vi.fn>).mockClear()
    mockListNodes.mockResolvedValue([node])
    mockGetProtocols.mockResolvedValue(protocols)
  })

  it('新建节点内持久展示分支清空规则，不使用顶部动态消息', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as { openCreate: () => void }
    vm.openCreate()
    await nextTick()

    const warning = document.body.querySelector<HTMLElement>('.node-reset-warning')
    expect(warning).not.toBeNull()
    expect(warning?.textContent).toContain('切回或重新开启不会恢复')
    expect(warning?.textContent).toContain('仍保留名称、服务器和端口')
    expect(Notify.warning).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('批量导入回执为长 URI 提供局部桌面表格和手机卡片布局', async () => {
    const raw = 'vless://uuid@example.com:443?' + 'transport-parameter='.repeat(20)
    const reason = 'URI 参数无法识别：' + 'unknown-parameter='.repeat(20)
    mockImportNodes.mockResolvedValue({
      list: [
        { line: 1, raw, ok: true, name: 'US-2' },
        { line: 2, raw: 'invalid://node', ok: false, name: 'failed-node', reason },
      ],
      total: 2,
    })
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openImport()
    vm.importText = raw
    await vm.doImport()
    await flushPromises()

    expect(mockImportNodes).toHaveBeenCalledWith(raw)
    const receipt = document.body.querySelector('.import-receipt')!
    const table = receipt.querySelector('.import-receipt-table')!
    const mobile = receipt.querySelector('.import-receipt-mobile')!
    expect(receipt.classList).toContain('overflow-y-auto')
    expect(table.classList).toContain('hidden')
    expect(table.classList).toContain('md:block')
    expect(mobile.classList).toContain('md:hidden')
    const details = [...receipt.querySelectorAll('.import-receipt-detail')]
    expect(details).toHaveLength(4)
    expect(details.filter((detail) => detail.textContent === raw)).toHaveLength(2)
    expect(details.filter((detail) => detail.textContent === reason)).toHaveLength(2)
    expect(mobile.textContent).toContain('第 1 行')
    expect(mobile.textContent).toContain('第 2 行')
    expect(mobile.textContent).toContain('US-2')
    expect(mobile.textContent).toContain('成功')
    expect(mobile.textContent).toContain('跳过')
    wrapper.unmount()
  })

  it('SMux 开关只出现一次并集中于更多开关，参数仍位于高级结构化区', async () => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol: 'vless', form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { uuid: 'keep', smux: smuxValue() }
    await nextTick()
    const enabled = wrapper.findAllComponents(ProtocolFieldEditor).filter((field) => field.props('path') === 'smux.enabled')
    expect(enabled).toHaveLength(1)
    expect(enabled[0].element.closest('.node-more-switches')).not.toBeNull()
    const maximum = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('path') === 'smux.max-connections')!
    const advancedRegion = maximum.element.closest('.node-advanced-fields')!
    expect(advancedRegion).not.toBeNull()
    expect(advancedRegion.querySelector('.ant-switch')).toBeNull()
    expect((enabled[0].element.closest('.node-more-switches') as HTMLDetailsElement).open).toBe(false)
    await enabled[0].find('.ant-switch').trigger('click')
    expect(vm.form.protocol_json).toEqual({ uuid: 'keep', smux: { enabled: false } })
    expect(vm.checkRequest.protocol_json).toEqual(vm.form.protocol_json)
    expect(vm.resetScopesArray()).toContain('feature.smux')
    wrapper.unmount()
  })

  it('保存时展开包含未应用 JSON 的折叠区域并定位编辑器', async () => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol: 'vless', form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { smux: smuxValue() }
    await nextTick()
    const smux = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'smux')!
    await smux.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await smux.find('textarea').setValue('{')
    await vm.save()
    expect((smux.element.closest('.node-advanced-fields') as HTMLDetailsElement).open).toBe(true)
    expect(document.activeElement).toBe(smux.find('textarea').element)
    expect(mockCreateNode).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('多个未应用 JSON 草稿保存时按稳定路径排序定位首个', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'vless',
      label: 'VLESS',
      form_schema: [
        { name: 'uuid', type: 'password', required: true, label: 'UUID', group: 'auth' },
        { name: 'a-opts', type: 'object', required: false, label: 'A 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, properties: [{ name: 'value', type: 'text', required: false, label: '值' }] },
        { name: 'z-opts', type: 'object', required: false, label: 'Z 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, properties: [{ name: 'value', type: 'text', required: false, label: '值' }] },
      ],
      sensitive_fields: ['uuid'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { 'a-opts': { value: 'a' }, 'z-opts': { value: 'z' } }
    await nextTick()
    // 故意按稳定排序的逆序加入，证明保存没有沿用 Set 插入序。
    vm.handleJsonDirty({ path: 'z-opts', dirty: true })
    vm.handleJsonDirty({ path: 'a-opts', dirty: true })
    await vm.save()
    const focusedPath = (document.activeElement as HTMLElement | null)?.closest('[data-field-path]')?.getAttribute('data-field-path')
    expect(focusedPath?.startsWith('a-opts')).toBe(true)
    expect(mockCreateNode).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('业务条件隐藏清理失效 dirty/validity，并保留无关有效草稿', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'vless',
      label: 'VLESS',
      form_schema: [
        { name: 'uuid', type: 'password', required: true, label: 'UUID', group: 'auth' },
        { name: 'network', type: 'select', required: true, label: '传输', group: 'connection', options: ['ws', 'tcp'] },
        {
          name: 'ws-opts', type: 'object', required: false, label: 'WebSocket 参数', group: 'connection',
          object_kind: 'fields', allow_unknown: false, when: { network: ['ws'] }, reset_on: ['network'],
          properties: [{ name: 'headers', type: 'object', required: false, label: '请求头', object_kind: 'map', allow_unknown: true }],
        },
        { name: 'stable-opts', type: 'object', required: false, label: '稳定参数', group: 'connection', object_kind: 'fields', allow_unknown: false, reset_on: ['protocol'], properties: [{ name: 'value', type: 'text', required: false, label: '值' }] },
      ],
      sensitive_fields: ['uuid'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { network: 'ws', 'ws-opts': { headers: {} }, 'stable-opts': { value: 'keep' } }
    await nextTick()
    const ws = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'ws-opts')!
    const stable = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'stable-opts')!
    await ws.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await ws.find('textarea').setValue('{')
    await stable.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await stable.find('textarea').setValue('{"value":"draft"}')
    await nextTick()
    expect(vm.invalidProtocolPaths.size).toBeGreaterThan(0)
    expect(vm.unappliedJsonPaths.has('ws-opts')).toBe(true)
    expect(vm.unappliedJsonPaths.has('stable-opts')).toBe(true)

    vm.setField('network', 'tcp')
    await nextTick()
    expect(vm.form.protocol_json['ws-opts']).toBeUndefined()
    expect(vm.form.protocol_json['stable-opts']).toEqual({ value: 'keep' })
    expect(vm.invalidProtocolPaths.size).toBe(0)
    expect(Array.from(vm.unappliedJsonPaths)).toEqual(['stable-opts'])
    wrapper.unmount()
  })

  it('集中开关修改使重叠 JSON 草稿失效，不能重新应用旧值', async () => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol: 'vless', form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { smux: smuxValue() }
    await nextTick()
    const smux = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'smux')!
    await smux.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await smux.find('textarea').setValue('{"enabled":true,"padding":true,"max-connections":99}')
    const padding = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('path') === 'smux.padding')!
    await padding.find('.ant-switch').trigger('click')
    const json = JSON.parse(smux.find('textarea').element.value)
    expect(json.padding).toBe(false)
    expect(json['max-connections']).toBe(7)
    expect(vm.unappliedJsonPaths.size).toBe(0)
    wrapper.unmount()
  })

  it.each(['ss', 'vless', 'vmess'])('%s 嵌套开关关闭并重开不恢复旧参数，检查和保存提交相同草稿', async (protocol) => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol, form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = protocol
    vm.form.protocol_json = { password: 'keep', smux: smuxValue() }
    await nextTick()
    const enabled = () => wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('path') === 'smux.enabled')!
    await enabled().find('.ant-switch').trigger('click')
    expect(vm.form.protocol_json).toEqual({ password: 'keep', smux: { enabled: false } })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((field) => field.props('path') === 'smux.max-connections')).toBe(false)
    expect(vm.resetScopesArray()).toContain('feature.smux')
    await enabled().find('.ant-switch').trigger('click')
    expect(vm.form.protocol_json.smux).toEqual({ enabled: true })
    expect(vm.checkRequest.protocol_json.smux).toEqual({ enabled: true })
    await vm.save()
    expect(mockCreateNode.mock.calls[0][0].protocol_json.smux).toEqual({ enabled: true })
    expect(mockCreateNode.mock.calls[0][0].reset_scopes).toContain('feature.smux')
    wrapper.unmount()
  })

  it('关闭 Brutal 清除所属扩展和未应用 JSON，重开不恢复；SMux 参数和公共扩展保留', async () => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol: 'vless', form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    const original = { ...node, protocol: 'vless', protocol_json: { uuid: 'keep', smux: smuxValue() }, extensions: [
      { id: 'parent', scope: 'feature.smux', configured: true },
      { id: 'child', scope: 'feature.smux.brutal', configured: true },
      { id: 'common', scope: 'node', configured: true },
    ] }
    vm.openEdit(original)
    await nextTick()
    const brutal = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('path') === 'smux.brutal-opts')!
    await brutal.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await brutal.find('textarea').setValue('{"enabled":true,"up":"stale-draft"}')
    vm.openExtensionReplace(original.extensions[1])
    vm.extensionDraft.payload = 'stale-extension'
    vm.setField('smux', { ...smuxValue(), 'brutal-opts': { ...smuxValue()['brutal-opts'], enabled: false } })
    await nextTick()
    expect(vm.form.protocol_json.smux['brutal-opts']).toEqual({ enabled: false })
    expect(vm.form.protocol_json.smux['max-connections']).toBe(7)
    expect(vm.extensionDraft.open).toBe(false)
    expect(vm.checkRequest.extension_ops).toEqual([{ op: 'clear', id: 'child' }])
    expect(brutal.find('textarea').element.value).not.toContain('stale-draft')
    expect(JSON.parse(brutal.find('textarea').element.value)).toEqual({ enabled: false })
    vm.setField('smux', { ...vm.form.protocol_json.smux, 'brutal-opts': { enabled: true, up: '50 Mbps' } })
    await vm.save()
    expect(mockUpdateNode.mock.calls[0][1].protocol_json.smux['brutal-opts']).toEqual({ enabled: true, up: '50 Mbps' })
    expect(original.protocol_json.smux).toEqual(smuxValue())
    wrapper.unmount()
  })

  it('子对象未应用 JSON 草稿阻止父对象进入高级 JSON 并定位子编辑器', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'vless',
      label: 'VLESS',
      form_schema: [
        { name: 'uuid', type: 'password', required: true, label: 'UUID', group: 'auth' },
        { name: 'network', type: 'select', required: true, label: '传输', group: 'connection', options: ['tcp', 'ws'] },
        {
          name: 'ws-opts', type: 'object', required: false, label: 'WebSocket 参数', group: 'connection',
          object_kind: 'fields', allow_unknown: false, when: { network: ['ws'] },
          properties: [
            { name: 'path', type: 'text', required: false, label: '路径' },
            { name: 'headers', type: 'object', required: false, label: '请求头', object_kind: 'map', allow_unknown: true },
          ],
        } as FieldSchema,
      ],
      sensitive_fields: ['uuid'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 50))
    vm.form.protocol_json = { uuid: 'child-json-secret', network: 'ws', 'ws-opts': { path: '/ws' } }
    await nextTick()

    const child = wrapper.findAllComponents(ProtocolFieldEditor).find((item) => item.props('path') === 'ws-opts.headers')!
    const parent = wrapper.findAllComponents(ProtocolFieldEditor).find((item) => item.props('field').name === 'ws-opts')!
    await child.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await child.find('textarea').setValue('{"X-Test":"draft"}')
    await nextTick()
    expect(Array.from(vm.unappliedJsonPaths)).toContain('ws-opts.headers')
    expect(parent.props('jsonDirtyPaths')).toContain('ws-opts.headers')
    expect((parent.vm as any).descendantJsonDirtyPaths).toEqual(['ws-opts.headers'])

    await parent.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await nextTick()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect((parent.vm as any).advanced).toBe(false)
    expect(child.find('textarea').exists()).toBe(true)
    expect(Notify.warning).toHaveBeenCalledWith(expect.stringContaining('后代 JSON 草稿'))
    const focusedPath = (document.activeElement as HTMLElement | null)?.closest('[data-field-path]')?.getAttribute('data-field-path')
    expect(focusedPath).toBe('ws-opts.headers')

    // 子草稿应用后，父对象才可进入高级 JSON。
    await child.findAll('button').find((button) => button.text().replace(/\s/g, '').includes('应用'))!.trigger('click')
    await nextTick()
    await parent.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await nextTick()
    expect((parent.vm as any).advanced).toBe(true)
    wrapper.unmount()
  })

  it.each([true, false])('高级 JSON 清除残留参数并丢弃旧文本（原启用状态 %s）', async (enabled) => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol: 'vless', form_schema: [smuxSchema] }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol_json = { smux: enabled ? smuxValue() : { enabled: false } }
    await nextTick()
    const smux = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'smux')!
    await smux.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    await smux.find('textarea').setValue(JSON.stringify({ ...smuxValue(), enabled: false }))
    const apply = () => smux.findAll('button').find((button) => button.text().replace(/\s/g, '') === '应用')!
    await apply().trigger('click')
    expect(vm.form.protocol_json.smux).toEqual({ enabled: false })
    expect(JSON.parse(smux.find('textarea').element.value)).toEqual({ enabled: false })
    await apply().trigger('click')
    expect(vm.form.protocol_json.smux).toEqual({ enabled: false })
    wrapper.unmount()
  })

  it('动态表单按协议渲染，敏感字段显示“留空 = 保留原凭据”', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      form: { protocol: string; protocol_json: Record<string, unknown> }
    }
    vm.openCreate()
    vm.form.protocol = 'ss'
    vm.form.protocol_json = { plugin: 'custom-plugin' }
    await nextTick()
    expect(document.body.textContent).toContain('加密方式')
    expect(document.body.textContent).toContain('密码')
    expect(document.body.querySelector('input[placeholder="未配置"]')).not.toBeNull()
    expect(document.body.textContent).toContain('认证与密钥')
    expect(document.body.textContent).toContain('连接方式与当前参数')
    expect(document.body.textContent).toContain('独立开关')
    expect(document.body.querySelector('.node-switch-fields')?.textContent).toContain('UDP')
    expect(document.body.querySelector('.protocol-object-field')?.textContent).toContain('结构化编辑')
    expect(document.body.querySelector('.node-advanced-fields')?.textContent).toContain('路由标记')
    expect(document.body.textContent).toContain('当前组合')
    wrapper.unmount()
  })

  it('嵌套凭据仅按 saved_sensitive_paths 回显，分支往返后不恢复旧凭据', async () => {
    const shadowPath = 'shadow-tls-opts.password'
    mockGetProtocols.mockResolvedValue([{
      protocol: 'ss', label: 'Shadowsocks', sensitive_fields: [shadowPath, 'restls-opts.password'], link_mappings: { sr: true, generic: true },
      form_schema: [
        { name: 'password', type: 'password', required: true, label: '主密码', group: 'auth' },
        { name: 'plugin', type: 'select', required: false, label: '插件', group: 'connection', options: ['shadow-tls', 'restls'] },
        {
          name: 'shadow-tls-opts', type: 'object', required: false, label: 'shadow-tls 参数', group: 'connection', object_kind: 'fields', reset_on: ['plugin'], when: { plugin: ['shadow-tls'] },
          properties: [{ name: 'password', type: 'password', required: false, label: '密码' }],
        },
        {
          name: 'restls-opts', type: 'object', required: false, label: 'restls 参数', group: 'connection', object_kind: 'fields', reset_on: ['plugin'], when: { plugin: ['restls'] },
          properties: [{ name: 'password', type: 'password', required: false, label: '密码' }],
        },
      ],
    }])
    const editNode = {
      ...node,
      protocol_json: { password: '', plugin: 'shadow-tls', 'shadow-tls-opts': { password: '' } },
      current_state: { security: 'none', plugin: 'shadow-tls' },
      saved_sensitive_paths: ['password', shadowPath],
    }
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit(editNode)
    await nextTick()
    const nestedInput = () => wrapper.findAllComponents(ProtocolFieldEditor)
      .find((field) => field.props('path') === shadowPath)!.find('input')
    expect(nestedInput().attributes('placeholder')).toBe('已保存（留空保留）')

    vm.setField('plugin', 'restls')
    vm.setField('plugin', 'shadow-tls')
    await nextTick()
    expect(nestedInput().attributes('placeholder')).toBe('未配置')
    await nestedInput().setValue('replacement')
    expect(document.body.textContent).toContain('待替换')
    await nestedInput().setValue('')
    expect(vm.checkRequest.credential_ops).toContainEqual({ path: shadowPath, op: 'clear' })
    wrapper.unmount()
  })

  it('手机端新建节点表单使用全屏 Drawer，字段仍可渲染', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: true,
        media: '(max-width: 767px)',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as { openCreate: () => void }
    vm.openCreate()
    await nextTick()

    expect(document.body.querySelector('.ant-modal')).toBeNull()
    expect(document.body.querySelector('.ant-drawer')).not.toBeNull()
    expect(document.body.querySelector('input[placeholder="域名或 IP"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('结构化对象存在格式错误时阻止保存', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      handleFieldValidity: (payload: { path: string; valid: boolean }) => void
      save: () => Promise<void>
    }
    vm.openCreate()
    vm.handleFieldValidity({ path: 'plugin-opts', valid: false })
    await vm.save()

    expect(mockCreateNode).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('切换网络分支会清空旧传输参数并记录 network reset', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'vless',
      label: 'VLESS',
      form_schema: [
        { name: 'uuid', type: 'password', required: true, label: 'UUID', group: 'auth' },
        { name: 'network', type: 'select', required: true, label: '传输', group: 'connection', options: ['tcp', 'ws'] },
        {
          name: 'ws-opts', type: 'object', required: false, label: 'WebSocket 参数', group: 'connection',
          object_kind: 'fields', allow_unknown: false, reset_on: ['network'],
          properties: [{ name: 'path', type: 'text', required: false, label: '路径' }],
        },
      ],
      sensitive_fields: ['uuid'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      form: { protocol_json: Record<string, unknown> }
      setField: (key: string, value: unknown) => void
      resetScopesArray: () => string[]
    }
    vm.openCreate()
    vm.form.protocol_json = { network: 'ws', 'ws-opts': { path: '/ws' } }
    vm.setField('network', 'tcp')
    await nextTick()

    expect(vm.form.protocol_json.network).toBe('tcp')
    expect(vm.form.protocol_json['ws-opts']).toBeUndefined()
    expect(vm.resetScopesArray()).toContain('network')
    expect(Notify.warning).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('切换协议清空协议参数并记录 protocol reset', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'ss',
      label: 'Shadowsocks',
      form_schema: [
        { name: 'cipher', type: 'text', required: true, label: '加密方式', group: 'connection' },
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
      ],
      sensitive_fields: ['password'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      form: { protocol_json: Record<string, unknown>; protocol: string }
      updateProtocol: (protocol: string) => void
      resetScopesArray: () => string[]
    }
    vm.openCreate()
    vm.form.protocol = 'ss'
    vm.form.protocol_json = { cipher: 'aes-128-gcm', password: 'secret' }
    vm.updateProtocol('vless')
    await nextTick()

    expect(vm.form.protocol_json).toEqual({})
    expect(vm.resetScopesArray()).toContain('protocol')
    expect(Notify.warning).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('保存 payload 包含 current_state、reset_scopes 与 base_revision', async () => {
    mockCreateNode.mockResolvedValue(node)
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      form: { name: string; host: string; port: number; protocol_json: Record<string, unknown> }
      save: () => Promise<void>
    }
    vm.openCreate()
    vm.form.name = 'new-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.form.protocol_json = { cipher: 'aes-128-gcm' }
    await vm.save()

    const payload = mockCreateNode.mock.calls[0][0] as Record<string, unknown>
    expect(payload.current_state).toBeDefined()
    expect(Array.isArray(payload.reset_scopes)).toBe(true)
    wrapper.unmount()
  })

  it('409 时保留当前草稿并提示重新加载', async () => {
    mockUpdateNode.mockRejectedValue(new ApiError(409, '节点已被其他编辑更新，请重新加载后重试'))
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openEdit: (target: any) => void
      form: { protocol_json: Record<string, unknown> }
      save: () => Promise<void>
      conflictError: string
    }
    vm.openEdit(node)
    vm.form.protocol_json = { cipher: 'changed' }
    await vm.save()

    expect(mockUpdateNode).toHaveBeenCalled()
    const payload = mockUpdateNode.mock.calls[0][1] as Record<string, unknown>
    expect(payload.base_revision).toBe(3)
    expect(vm.conflictError).toContain('重新加载')
    expect(vm.form.protocol_json.cipher).toBe('changed')
    wrapper.unmount()
  })

  it.each(['vless', 'vmess'])('编辑 %s TLS 节点时只用 security，切换后检查与保存均不带旧 tls', async (protocol) => {
    const editNode = {
      ...node,
      protocol,
      protocol_json: { uuid: 'u', tls: true },
      current_state: { network: 'tcp', security: 'tls' },
    }
    mockGetProtocols.mockResolvedValue([{
      protocol,
      label: 'VLESS',
      form_schema: [
        { name: 'uuid', type: 'password', required: true, label: 'UUID', group: 'auth' },
        {
          name: 'security', type: 'select', default: 'none', label: '安全', group: 'connection',
          options: ['none', 'tls'],
          option_items: [{ value: 'none', label: '无' }, { value: 'tls', label: 'TLS' }],
        },
        { name: 'servername', type: 'text', label: 'SNI', group: 'connection' },
      ],
      sensitive_fields: ['uuid'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit(editNode)
    expect(vm.form.protocol_json.security).toBe('tls')
    expect(vm.form.protocol_json).not.toHaveProperty('tls')
    expect(editNode.protocol_json.tls).toBe(true)
    vm.setField('security', 'none')
    expect(vm.checkRequest.protocol_json.security).toBe('none')
    expect(vm.checkRequest.protocol_json).not.toHaveProperty('tls')
    expect(vm.checkRequest.reset_scopes).toContain('security')
    mockUpdateNode.mockResolvedValue({ ...editNode, edit_revision: 4 })
    await vm.save()
    const payload = mockUpdateNode.mock.calls[0]![1]
    expect(payload.protocol_json.security).toBe('none')
    expect(payload.protocol_json).not.toHaveProperty('tls')
    wrapper.unmount()
  })

  it.each(['http', 'socks5', 'ss'])('编辑 %s 保留其自身有效 TLS 参数', async (protocol) => {
    mockGetProtocols.mockResolvedValue([{ ...protocols[0], protocol }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    const params = protocol === 'ss' ? { plugin: 'v2ray-plugin', 'v2ray-plugin-opts': { tls: true } } : { tls: true }
    vm.openEdit({ ...node, protocol, protocol_json: params })
    expect(vm.form.protocol_json).toEqual(params)
    wrapper.unmount()
  })

  it('存在未应用 JSON 草稿时阻止保存', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      handleJsonDirty: (payload: { path: string; dirty: boolean }) => void
      save: () => Promise<void>
      form: { name: string; host: string; port: number }
    }
    vm.openCreate()
    vm.form.name = 'new-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.handleJsonDirty({ path: 'plugin-opts', dirty: true })
    await vm.save()

    expect(mockCreateNode).not.toHaveBeenCalled()
    expect(Notify.warning).toHaveBeenCalledWith(expect.stringContaining('未应用的 JSON'))
    expect(wrapper.findComponent(NodeCheckPanel).props('blockedReason')).toContain('未应用的 JSON')
    wrapper.unmount()
  })

  it('存在未应用自定义值或列表项草稿时阻止保存和目标检查', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.name = 'new-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.handleControlDraftDirty({ path: 'network', dirty: true })
    await vm.save()

    expect(mockCreateNode).not.toHaveBeenCalled()
    expect(Notify.warning).toHaveBeenCalledWith('存在未应用的自定义值或列表项草稿，请先应用或取消后再保存')
    expect(wrapper.findComponent(NodeCheckPanel).props('blockedReason')).toContain('未应用')
    wrapper.unmount()
  })

  it('新增未知扩展随保存提交 extensions', async () => {
    mockCreateNode.mockResolvedValue(node)
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      commitExtensionDraft: () => void
      save: () => Promise<void>
      form: { name: string; host: string; port: number }
      extensionDraft: { scope: string; targets: string[]; label: string; payload: string }
    }
    vm.openCreate()
    vm.form.name = 'new-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.extensionDraft.scope = 'node'
    vm.extensionDraft.targets = ['clash-yaml', 'sr-subs']
    vm.extensionDraft.label = '测试扩展'
    vm.extensionDraft.payload = '{"unknown":true}'
    vm.commitExtensionDraft()
    await vm.save()

    const payload = mockCreateNode.mock.calls[0][0] as Record<string, any>
    expect(payload.extensions).toEqual([
      { scope: 'node', targets: ['clash-yaml', 'sr-subs'], label: '测试扩展', payload: '{"unknown":true}' },
    ])
    wrapper.unmount()
  })

  it('未知扩展 targets 接受空数组并明确不进入输出产物', async () => {
    mockCreateNode.mockResolvedValue(node)
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      openExtensionAdd: () => void
      commitExtensionDraft: () => void
      save: () => Promise<void>
      form: { name: string; host: string; port: number }
      extensionDraft: { scope: string; targets: string[]; label: string; payload: string }
      checkRequest: { targets: string[] }
    }
    vm.openCreate()
    await nextTick()
    vm.openExtensionAdd()
    vm.form.name = 'empty-target-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.extensionDraft.scope = 'node'
    vm.extensionDraft.targets = []
    vm.extensionDraft.label = '空 targets'
    vm.extensionDraft.payload = '{"unknown":true}'
    vm.openExtensionAdd()
    await nextTick()
    expect(document.body.textContent).toContain('不会进入任何输出产物')
    expect(document.body.textContent).toContain('留空表示未关联任何目标')
    vm.extensionDraft.targets = []
    vm.extensionDraft.label = '空 targets'
    vm.extensionDraft.payload = '{"unknown":true}'
    vm.commitExtensionDraft()
    expect(vm.checkRequest.targets).toEqual(['clash-yaml', 'sr-subs', 'generic-subs'])
    await vm.save()
    const payload = mockCreateNode.mock.calls[0][0] as Record<string, any>
    expect(payload.extensions).toEqual([
      { scope: 'node', targets: [], label: '空 targets', payload: '{"unknown":true}' },
    ])
    wrapper.unmount()
  })

  it('未知扩展非法 targets 被前端阻断且不进入保存草稿', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      commitExtensionDraft: () => void
      extensionOps: Array<{ op: string }>
      extensionDraft: { scope: string; targets: string[]; label: string; payload: string }
    }
    vm.openCreate()
    vm.extensionDraft.scope = 'node'
    vm.extensionDraft.targets = ['sr-conf']
    vm.extensionDraft.label = '非法 target'
    vm.extensionDraft.payload = '{"unknown":true}'
    vm.commitExtensionDraft()
    expect(vm.extensionOps).toEqual([])
    expect(Notify.warning).toHaveBeenCalledWith(expect.stringContaining('sr-conf'))
    wrapper.unmount()
  })

  it('编辑节点清除未知扩展时提交 clear extension_ops', async () => {
    const editNode = {
      ...node,
      extensions: [{ id: 'ext-1', scope: 'node', targets: ['clash-yaml'], label: '旧扩展', configured: true }],
    }
    mockUpdateNode.mockResolvedValue(editNode)
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openEdit: (target: any) => void
      removeExtension: (ext: { id: string; scope: string; targets: string[]; label: string }) => void
      save: () => Promise<void>
      form: { protocol_json: Record<string, unknown> }
    }
    vm.openEdit(editNode)
    vm.removeExtension({ id: 'ext-1', scope: 'node', targets: ['clash-yaml'], label: '旧扩展' })
    await vm.save()

    expect(mockUpdateNode).toHaveBeenCalled()
    const payload = mockUpdateNode.mock.calls[0][1] as Record<string, any>
    expect(payload.extension_ops).toEqual([{ op: 'clear', id: 'ext-1' }])
    wrapper.unmount()
  })

  it('SS 按插件显示独立对象，递归字段不混排', async () => {
    mockGetProtocols.mockResolvedValue([{
      protocol: 'ss',
      label: 'Shadowsocks',
      form_schema: [
        { name: 'cipher', type: 'text', required: true, label: '加密方式', group: 'connection' },
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
        {
          name: 'plugin', type: 'select', required: false, label: '插件', group: 'connection', default: '',
          option_items: [{ value: '', label: '不使用插件' }, { value: 'obfs', label: 'obfs' }, { value: 'v2ray-plugin', label: 'v2ray-plugin' }],
        },
        {
          name: 'obfs-opts', type: 'object', required: false, label: 'obfs 参数', group: 'connection',
          object_kind: 'fields', allow_unknown: false, when: { plugin: ['obfs'] },
          properties: [
            { name: 'mode', type: 'select', required: false, label: '模式', option_items: [{ value: 'http', label: 'HTTP' }, { value: 'tls', label: 'TLS' }] },
          ],
        },
        {
          name: 'v2ray-plugin-opts', type: 'object', required: false, label: 'v2ray-plugin 参数', group: 'connection',
          object_kind: 'fields', allow_unknown: false, when: { plugin: ['v2ray-plugin'] },
          properties: [{ name: 'tls', type: 'bool', required: false, default: false, label: 'TLS' }],
        },
      ],
      sensitive_fields: ['password'],
      link_mappings: { sr: true, generic: true },
    }])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openCreate: () => void
      form: { protocol: string; protocol_json: Record<string, unknown> }
    }
    vm.openCreate()
    vm.form.protocol = 'ss'
    vm.form.protocol_json = { plugin: 'obfs' }
    await nextTick()

    expect(document.body.textContent).toContain('obfs 参数')
    expect(document.body.textContent).toContain('模式')
    expect(document.body.textContent).not.toContain('v2ray-plugin 参数')
    expect(document.body.textContent).not.toContain('TLS')
    wrapper.unmount()
  })

  it('未知插件入口完全由真实 schema 补集驱动，所有插件切换均清空且不恢复参数', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'ss'
    vm.form.protocol_json = {
      cipher: 'aes-256-gcm', password: 'keep', plugin: 'obfs',
      'plugin-opts': { stale: 'unknown' }, 'obfs-opts': { mode: 'http' },
      'v2ray-plugin-opts': { mode: 'websocket' }, 'shadow-tls-opts': { version: '3' }, 'restls-opts': { version_hint: 'keep' },
    }
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((field) => field.props('field').name === 'plugin-opts')).toBe(false)

    vm.setField('plugin', 'unknown-a')
    await nextTick()
    expect(vm.form.protocol_json).toEqual({ cipher: 'aes-256-gcm', password: 'keep', plugin: 'unknown-a' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((field) => field.props('field').name === 'plugin-opts')).toBe(true)
    vm.setField('plugin-opts', { flag: '', host: 'cdn.example.com' })
    vm.handleFieldValidity({ path: 'plugin-opts.host', valid: false })
    vm.handleJsonDirty({ path: 'plugin-opts', dirty: true })

    vm.setField('plugin', 'unknown-b')
    expect(vm.form.protocol_json).toEqual({ cipher: 'aes-256-gcm', password: 'keep', plugin: 'unknown-b' })
    expect(vm.invalidProtocolPaths.size).toBe(0)
    expect(vm.unappliedJsonPaths.size).toBe(0)
    vm.setField('plugin', 'unknown-a')
    expect(vm.form.protocol_json).toEqual({ cipher: 'aes-256-gcm', password: 'keep', plugin: 'unknown-a' })
    vm.setField('plugin-opts', { mode: 'again' })
    vm.setField('plugin', 'v2ray-plugin')
    expect(vm.form.protocol_json).toEqual({ cipher: 'aes-256-gcm', password: 'keep', plugin: 'v2ray-plugin' })
    vm.setField('plugin', '')
    expect(vm.form.protocol_json).toEqual({ cipher: 'aes-256-gcm', password: 'keep', plugin: '' })
    expect(vm.resetScopesArray()).toContain('plugin')
    wrapper.unmount()
  })

  it('未知插件普通字符串参数可创建、按响应重开，且敏感命名不进入凭据 UI', async () => {
    const saved = {
      ...node,
      name: 'custom-node',
      protocol_json: {
        cipher: 'aes-256-gcm', password: '', plugin: 'custom-plugin',
        'plugin-opts': { flag: '', special: ':;=\\', password: 'ordinary', token: 'plain', secret: 'visible' },
      },
      current_state: { security: 'none', plugin: 'custom-plugin' },
      saved_sensitive_paths: ['password'],
    }
    mockCreateNode.mockResolvedValue(saved)
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.name = saved.name
    vm.form.host = saved.host
    vm.form.port = saved.port
    vm.form.protocol = 'ss'
    vm.form.protocol_json = { ...saved.protocol_json, password: 'main-secret' }
    await vm.save()
    expect(mockCreateNode.mock.calls[0][0].protocol_json['plugin-opts']).toEqual(saved.protocol_json['plugin-opts'])

    vm.openEdit(saved)
    await nextTick()
    expect(vm.form.protocol_json['plugin-opts']).toEqual(saved.protocol_json['plugin-opts'])
    const mapEditor = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'plugin-opts')!
    expect(mapEditor.exists()).toBe(true)
    const rows = mapEditor.findAll('.protocol-map-entry')
    expect(rows).toHaveLength(5)
    for (const row of rows) {
      expect(row.classes()).toContain('grid-cols-1')
      expect(row.classes()).toContain('md:grid-cols-[minmax(140px,0.7fr)_minmax(180px,1fr)_auto]')
      expect(row.findAll('.min-w-0')).toHaveLength(2)
      expect(row.find('button').text().replace(/\s/g, '')).toContain('删除')
      expect(row.find('input[type="password"]').exists()).toBe(false)
      expect(row.text()).not.toContain('已保存')
      expect(row.text()).not.toContain('待替换')
      expect(row.text()).not.toContain('已清除')
    }

    const originalNameInput = mapEditor.findAll('input[aria-label="参数名"]')[0].element
    ;(originalNameInput as HTMLInputElement).focus()
    for (const name of ['f', 'fl', 'flag-next']) {
      await mapEditor.findAll('input[aria-label="参数名"]')[0].setValue(name)
      await nextTick()
      expect(mapEditor.findAll('input[aria-label="参数名"]')[0].element).toBe(originalNameInput)
      expect(document.activeElement).toBe(originalNameInput)
    }
    expect(vm.form.protocol_json['plugin-opts']).toEqual({
      'flag-next': '', special: ':;=\\', password: 'ordinary', token: 'plain', secret: 'visible',
    })
    wrapper.unmount()
  })

  it('未知插件参数名错误阻止保存并定位字段，删除错误行后恢复', async () => {
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.name = 'invalid-map-name'
    vm.form.host = 'example.com'
    vm.form.port = 8388
    vm.form.protocol = 'ss'
    vm.form.protocol_json = { cipher: 'aes-256-gcm', password: 'secret', plugin: 'custom-plugin', 'plugin-opts': { mode: 'custom' } }
    await nextTick()
    const mapEditor = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'plugin-opts')!
    await mapEditor.find('input[aria-label="参数名"]').setValue('')
    await vm.save()
    expect(mockCreateNode).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(mapEditor.find('input[aria-label="参数名"]').element)
    await mapEditor.find('.protocol-map-entry button').trigger('click')
    await vm.save()
    expect(mockCreateNode).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('WireGuard peers 高级 JSON 应用后保留内部身份且保存不再被阻断', async () => {
    const firstID = '11111111-1111-4111-8111-111111111111'
    const secondID = '22222222-2222-4222-8222-222222222222'
    mockGetProtocols.mockResolvedValue([{
      protocol: 'wireguard',
      label: 'WireGuard',
      form_schema: [
        { name: 'private-key', type: 'password', required: true, label: '私钥', group: 'auth' },
        { name: 'public-key', type: 'text', required: true, label: '公钥', group: 'auth' },
        {
          name: 'peers', type: 'object', required: false, label: 'Peer 列表', group: 'connection',
          object_kind: 'list', item_id_field: '_credential_id', allow_unknown: false,
          properties: [
            { name: 'server', type: 'text', required: false, label: '服务器' },
            { name: 'pre-shared-key', type: 'password', required: false, label: '预共享密钥' },
          ],
        },
      ],
      sensitive_fields: ['private-key', 'peers[].pre-shared-key'],
      link_mappings: { sr: true, generic: true },
    }])
    const wireguardNode = {
      ...node,
      protocol: 'wireguard',
      protocol_json: {
        'private-key': '',
        'public-key': 'wg-public',
        peers: [
          { _credential_id: firstID, server: 'peer-a', 'pre-shared-key': '' },
          { _credential_id: secondID, server: 'peer-b', 'pre-shared-key': '' },
        ],
      },
      current_state: { security: 'none' },
      saved_sensitive_paths: [`peers[${firstID}].pre-shared-key`, `peers[${secondID}].pre-shared-key`],
    }
    mockListNodes.mockResolvedValue([wireguardNode])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit(wireguardNode)
    await nextTick()
    const peers = wrapper.findAllComponents(ProtocolFieldEditor).find((field) => field.props('field').name === 'peers')!
    await peers.findAll('button').find((button) => button.text() === '高级 JSON')!.trigger('click')
    const reordered = [
      { _credential_id: secondID, server: 'peer-b', 'pre-shared-key': '' },
      { _credential_id: firstID, server: 'peer-a', 'pre-shared-key': '' },
    ]
    await peers.find('textarea').setValue(JSON.stringify(reordered))
    await peers.findAll('button').find((button) => button.text().replace(/\s/g, '') === '应用')!.trigger('click')
    expect(vm.invalidProtocolPaths.size).toBe(0)
    expect(vm.unappliedJsonPaths.size).toBe(0)
    expect(vm.form.protocol_json.peers).toEqual(reordered)

    mockUpdateNode.mockResolvedValue({ ...wireguardNode, edit_revision: 4 })
    await vm.save()
    expect(mockUpdateNode).toHaveBeenCalledTimes(1)
    expect(mockUpdateNode.mock.calls[0][1].protocol_json.peers).toEqual(reordered)
    wrapper.unmount()
  })

  const httpProtocol = {
    protocol: 'http',
    label: 'HTTP',
    form_schema: [
      { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['none', 'basic'], default: 'none' },
      { name: 'username', type: 'text', required: false, label: '用户名', group: 'auth', when: { selectors: { auth_mode: ['basic'] } }, required_when: { selectors: { auth_mode: ['basic'] } }, reset_on: ['selector.auth_mode'] },
      { name: 'password', type: 'password', required: false, label: '密码', group: 'auth', when: { selectors: { auth_mode: ['basic'] } }, required_when: { selectors: { auth_mode: ['basic'] } }, reset_on: ['selector.auth_mode'] },
      { name: 'tls', type: 'bool', default: false, label: 'TLS', section: 'switches', feature: { name: 'tls' }, reset_on: ['feature.tls'] },
      { name: 'sni', type: 'text', required: false, label: 'SNI', group: 'connection', when: { features: ['tls'] }, reset_on: ['feature.tls'] },
      { name: 'headers', type: 'object', required: false, label: '请求头', group: 'advanced', object_kind: 'map', map_value_type: 'string', allow_unknown: true },
    ],
    selectors: [{ name: 'auth_mode', values: ['none', 'basic'], default: 'none' }],
    sensitive_fields: ['password'],
    link_mappings: { sr: true, generic: true },
  }

  it('HTTP state_only selector 写入 current_state.selectors 并随切换清空凭据', async () => {
    mockGetProtocols.mockResolvedValue([httpProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'http'
    await nextTick()

    expect(vm.currentState.selectors).toEqual({ auth_mode: 'none' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'username')).toBe(false)
    expect(vm.form.protocol_json['auth-mode']).toBeUndefined()

    const modeField = httpProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'basic')
    await nextTick()
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'basic' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'username')).toBe(true)
    expect(vm.form.protocol_json['auth-mode']).toBeUndefined()

    vm.setField('username', 'u')
    vm.setField('password', 'p')
    await nextTick()
    vm.setFieldModelValue(modeField, 'none')
    await nextTick()
    expect(vm.form.protocol_json.username).toBeUndefined()
    expect(vm.form.protocol_json.password).toBeUndefined()
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'none' })

    vm.form.name = 'http-node'
    mockCreateNode.mockResolvedValue({})
    await vm.save()
    expect(mockCreateNode.mock.calls[0][0].current_state.selectors).toEqual({ auth_mode: 'none' })
    wrapper.unmount()
  })

  it('编辑 HTTP 节点时从 current_state.selectors 回填 state_only 选择', async () => {
    mockGetProtocols.mockResolvedValue([httpProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit({
      ...node,
      protocol: 'http',
      port: 8080,
      protocol_json: { username: 'u', password: '', tls: true, sni: 's.example.com' },
      current_state: { security: 'tls', features: ['tls'], selectors: { auth_mode: 'basic' } },
    })
    await nextTick()
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'basic' })
    const mode = wrapper.findAllComponents(ProtocolFieldEditor).find((item) => item.props('field').name === 'auth-mode')
    expect(mode?.props('modelValue')).toBe('basic')
    wrapper.unmount()
  })

  it('SOCKS5 复用认证/TLS 条件字段且 UDP 独立于认证切换', async () => {
    const socks5Protocol = {
      ...httpProtocol,
      protocol: 'socks5',
      label: 'SOCKS5',
      form_schema: [
        ...httpProtocol.form_schema.filter((field) => field.name !== 'sni' && field.name !== 'headers'),
        { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
      ],
    }
    mockGetProtocols.mockResolvedValue([socks5Protocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'socks5'
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'sni')).toBe(false)

    const modeField = socks5Protocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'basic')
    await nextTick()
    vm.setField('username', 'u')
    vm.setField('password', 'p')
    vm.setField('udp', false)
    await nextTick()
    vm.setFieldModelValue(modeField, 'none')
    await nextTick()
    expect(vm.form.protocol_json.username).toBeUndefined()
    expect(vm.form.protocol_json.udp).toBe(false)
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')
    wrapper.unmount()
  })

  it('SSH 认证分支切换清空另一组凭据并保留结构化 Host Key 列表', async () => {
    const sshProtocol = {
      protocol: 'ssh',
      label: 'SSH',
      form_schema: [
        { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['password', 'private_key'], default: 'password' },
        { name: 'username', type: 'text', required: true, label: '用户名', group: 'auth' },
        { name: 'password', type: 'password', required: false, label: '密码', group: 'auth', when: { selectors: { auth_mode: ['password'] } }, required_when: { selectors: { auth_mode: ['password'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'private-key', type: 'secret-multiline', required: false, label: '私钥', group: 'auth', when: { selectors: { auth_mode: ['private_key'] } }, required_when: { selectors: { auth_mode: ['private_key'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'private-key-passphrase', type: 'password', required: false, label: '私钥口令', group: 'auth', when: { selectors: { auth_mode: ['private_key'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'host-key', type: 'text-list', required: false, label: 'Host Key', group: 'connection' },
        { name: 'host-key-algorithms', type: 'text-list', required: false, label: 'Host Key 算法', group: 'connection' },
      ],
      selectors: [{ name: 'auth_mode', values: ['password', 'private_key'], default: 'password' }],
      sensitive_fields: ['password', 'private-key', 'private-key-passphrase'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([sshProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'ssh'
    await nextTick()

    // 默认 password 分支只显示密码，不显示私钥。
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'password' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'password')).toBe(true)
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'private-key')).toBe(false)

    vm.setField('username', 'u')
    vm.setField('password', 'pw')
    await nextTick()

    // A→B：切到 private_key 清空密码并显示私钥字段。
    const modeField = sshProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'private_key')
    await nextTick()
    expect(vm.form.protocol_json.password).toBeUndefined()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'private-key')).toBe(true)
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')
    expect(vm.form.protocol_json['auth-mode']).toBeUndefined()
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'private_key' })

    // Host Key 以结构化列表提交，不进入 state_only。
    vm.setField('host-key', ['ssh-ed25519 AAAA test'])
    vm.setField('host-key-algorithms', ['ssh-ed25519'])
    await nextTick()
    expect(vm.form.protocol_json['host-key']).toEqual(['ssh-ed25519 AAAA test'])

    // 编辑已保存节点时从 current_state.selectors 回填私钥分支。
    vm.openEdit({
      ...node,
      protocol: 'ssh',
      port: 22,
      protocol_json: { username: 'u', 'private-key': '', 'host-key': ['ssh-ed25519 AAAA test'] },
      current_state: { selectors: { auth_mode: 'private_key' } },
    })
    await nextTick()
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'private_key' })
    const mode = wrapper.findAllComponents(ProtocolFieldEditor).find((item) => item.props('field').name === 'auth-mode')
    expect(mode?.props('modelValue')).toBe('private_key')
    wrapper.unmount()
  })

  it('TrustTunnel 复用分支互斥且 quic 关闭清空 QUIC 调优字段', async () => {
    const trusttunnelProtocol = {
      protocol: 'trusttunnel',
      label: 'TrustTunnel',
      form_schema: [
        { name: 'reuse-mode', type: 'select', required: false, label: '连接复用', group: 'connection', state_only: true, selector_name: 'reuse_mode', options: ['none', 'connections', 'streams'], default: 'none' },
        { name: 'username', type: 'text', required: false, label: '用户名', group: 'auth' },
        { name: 'password', type: 'password', required: false, label: '密码', group: 'auth' },
        { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
        { name: 'health-check', type: 'bool', default: false, label: '健康检查', section: 'switches' },
        { name: 'quic', type: 'bool', default: false, label: 'QUIC (HTTP/3)', group: 'connection', feature: { name: 'quic' } },
        { name: 'congestion-controller', type: 'select', required: false, label: '拥塞控制器', group: 'advanced', options: ['', 'cubic', 'new_reno', 'bbr_meta_v1', 'bbr_meta_v2', 'bbr'], when: { features: ['quic'] }, reset_on: ['feature.quic'] },
        { name: 'cwnd', type: 'number', required: false, label: '拥塞窗口', group: 'advanced', when: { features: ['quic'] }, reset_on: ['feature.quic'] },
        { name: 'bbr-profile', type: 'text', required: false, label: 'BBR Profile', group: 'advanced', when: { features: ['quic'] }, reset_on: ['feature.quic'] },
        { name: 'max-connections', type: 'number', required: false, label: '最大连接数', group: 'connection', when: { selectors: { reuse_mode: ['connections'] } }, required_when: { selectors: { reuse_mode: ['connections'] } }, reset_on: ['selector.reuse_mode'] },
        { name: 'min-streams', type: 'number', required: false, label: '最小流数', group: 'connection', when: { selectors: { reuse_mode: ['connections'] } }, reset_on: ['selector.reuse_mode'] },
        { name: 'max-streams', type: 'number', required: false, label: '最大流数', group: 'connection', when: { selectors: { reuse_mode: ['streams'] } }, required_when: { selectors: { reuse_mode: ['streams'] } }, reset_on: ['selector.reuse_mode'] },
      ],
      selectors: [{ name: 'reuse_mode', values: ['none', 'connections', 'streams'], default: 'none' }],
      sensitive_fields: ['password'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([trusttunnelProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'trusttunnel'
    await nextTick()

    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    // none 分支不显示任何复用数字，selector 本身不进入 protocol_json。
    expect(vm.currentState.selectors).toEqual({ reuse_mode: 'none' })
    expect(fieldNames()).not.toContain('max-connections')
    expect(fieldNames()).not.toContain('max-streams')

    // A：connections 分支只显示 max-connections／min-streams。
    const modeField = trusttunnelProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'connections')
    vm.setField('max-connections', 8)
    vm.setField('min-streams', 5)
    await nextTick()
    expect(fieldNames()).toContain('max-connections')
    expect(fieldNames()).toContain('min-streams')
    expect(fieldNames()).not.toContain('max-streams')

    // A→B：切到 streams 清空旧复用数字，切回也不恢复。
    vm.setFieldModelValue(modeField, 'streams')
    await nextTick()
    expect(vm.form.protocol_json['max-connections']).toBeUndefined()
    expect(vm.form.protocol_json['min-streams']).toBeUndefined()
    expect(fieldNames()).toContain('max-streams')
    expect(vm.resetScopesArray()).toContain('selector.reuse_mode')
    expect(vm.form.protocol_json['reuse-mode']).toBeUndefined()
    expect(vm.currentState.selectors).toEqual({ reuse_mode: 'streams' })

    // quic 开启后可编辑调优字段，关闭必须清空。
    vm.setField('quic', true)
    vm.setField('cwnd', 64)
    vm.setField('congestion-controller', 'bbr')
    await nextTick()
    expect(fieldNames()).toContain('cwnd')
    vm.setField('quic', false)
    await nextTick()
    expect(vm.form.protocol_json.cwnd).toBeUndefined()
    expect(vm.form.protocol_json['congestion-controller']).toBeUndefined()
    expect(fieldNames()).not.toContain('cwnd')
    wrapper.unmount()
  })

  it('OpenVPN 认证分支只清空不再活动的凭据并保留仍活动的共享凭据', async () => {
    const openvpnProtocol = {
      protocol: 'openvpn',
      label: 'OpenVPN',
      form_schema: [
        { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['userpass', 'cert', 'cert_userpass'], default: 'userpass' },
        { name: 'tls-key-mode', type: 'select', required: false, label: 'TLS Key 模式', group: 'auth', state_only: true, selector_name: 'tls_key_mode', options: ['none', 'tls_auth', 'tls_crypt', 'tls_crypt_v2'], default: 'none' },
        { name: 'ca', type: 'multiline', required: true, label: 'CA 证书', section: 'security' },
        { name: 'username', type: 'text', required: false, label: '用户名', group: 'auth', when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, required_when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, clear_when_inactive: true },
        { name: 'password', type: 'password', required: false, label: '密码', group: 'auth', when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, required_when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, clear_when_inactive: true },
        { name: 'cert', type: 'multiline', required: false, label: '客户端证书', group: 'auth', when: { selectors: { auth_mode: ['cert', 'cert_userpass'] } }, required_when: { selectors: { auth_mode: ['cert', 'cert_userpass'] } }, clear_when_inactive: true },
        { name: 'key', type: 'secret-multiline', required: false, label: '客户端私钥', group: 'auth', when: { selectors: { auth_mode: ['cert', 'cert_userpass'] } }, required_when: { selectors: { auth_mode: ['cert', 'cert_userpass'] } }, clear_when_inactive: true },
        { name: 'tls-auth', type: 'secret-multiline', required: false, label: 'TLS Auth Key', group: 'auth', when: { selectors: { tls_key_mode: ['tls_auth'] } }, required_when: { selectors: { tls_key_mode: ['tls_auth'] } }, reset_on: ['selector.tls_key_mode'] },
        { name: 'key-direction', type: 'select', required: false, label: 'Key Direction', group: 'auth', options: ['', '0', '1'], when: { selectors: { tls_key_mode: ['tls_auth'] } }, clear_when_inactive: true },
      ],
      selectors: [
        { name: 'auth_mode', values: ['userpass', 'cert', 'cert_userpass'], default: 'userpass' },
        { name: 'tls_key_mode', values: ['none', 'tls_auth', 'tls_crypt', 'tls_crypt_v2'], default: 'none' },
      ],
      sensitive_fields: ['password', 'key', 'tls-auth'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([openvpnProtocol])
    mockUpdateNode.mockResolvedValue({})
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit({
      ...node,
      protocol: 'openvpn',
      host: 'vpn.example.com',
      port: 1194,
      protocol_json: { ca: 'ca-pem', cert: 'cert-pem', key: '', username: 'u', password: '' },
      current_state: { selectors: { auth_mode: 'cert_userpass', tls_key_mode: 'none' } },
      saved_sensitive_paths: ['key', 'password'],
    })
    await nextTick()
    const authField = openvpnProtocol.form_schema[0] as FieldSchema
    expect(vm.currentState.selectors).toEqual({ auth_mode: 'cert_userpass', tls_key_mode: 'none' })

    // cert_userpass → cert：只清空 username／password，证书与私钥必须保留。
    vm.setFieldModelValue(authField, 'cert')
    await nextTick()
    expect(vm.form.protocol_json.username).toBeUndefined()
    expect(vm.form.protocol_json.password).toBeUndefined()
    expect(vm.form.protocol_json.cert).toBe('cert-pem')
    expect(vm.form.protocol_json.key).toBe('')
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')
    await vm.save()
    const payload = mockUpdateNode.mock.calls[0][1] as { credential_ops?: { path: string; op: string }[] }
    const clearedPaths = (payload.credential_ops ?? []).map((op) => op.path)
    expect(clearedPaths).toContain('password')
    expect(clearedPaths).not.toContain('key')

    // A→B→A：切回 cert_userpass 不得恢复已清空的 username／password。
    vm.setFieldModelValue(authField, 'cert_userpass')
    await nextTick()
    expect(vm.form.protocol_json.username).toBeUndefined()
    expect(vm.form.protocol_json.password).toBeUndefined()

    // userpass→cert_userpass 不得清空仍活动的 username／password。
    vm.setFieldModelValue(authField, 'userpass')
    await nextTick()
    vm.setField('username', 'kept-user')
    vm.setField('password', 'kept-pass')
    await nextTick()
    vm.setFieldModelValue(authField, 'cert_userpass')
    await nextTick()
    expect(vm.form.protocol_json.username).toBe('kept-user')
    expect(vm.form.protocol_json.password).toBe('kept-pass')
    wrapper.unmount()
  })

  it('OpenVPN 粘贴解析应用只覆盖产出字段并纳入页面级阻断', async () => {
    const openvpnProtocol = {
      protocol: 'openvpn',
      label: 'OpenVPN',
      form_schema: [
        { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['userpass', 'cert', 'cert_userpass'], default: 'userpass' },
        { name: 'ca', type: 'multiline', required: true, label: 'CA 证书', section: 'security' },
        { name: 'proto', type: 'select', required: false, label: '传输协议', group: 'connection', options: ['udp', 'tcp'], default: 'udp' },
        { name: 'username', type: 'text', required: false, label: '用户名', group: 'auth', when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, clear_when_inactive: true },
        { name: 'password', type: 'password', required: false, label: '密码', group: 'auth', when: { selectors: { auth_mode: ['userpass', 'cert_userpass'] } }, clear_when_inactive: true },
      ],
      selectors: [{ name: 'auth_mode', values: ['userpass', 'cert', 'cert_userpass'], default: 'userpass' }],
      sensitive_fields: ['password'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([openvpnProtocol])
    mockParseOpenVPN.mockResolvedValue({
      host: 'vpn.example.com',
      port: 1194,
      protocol_json: { ca: 'parsed-ca', proto: 'tcp' },
      selectors: { auth_mode: 'userpass' },
      field_sources: { remote: 3, ca: 8, proto: 4 },
      diagnostics: [],
    })
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'openvpn'
    await nextTick()
    // 预先存在的草稿字段必须在应用解析结果后保留。
    vm.setField('username', 'kept-user')
    await nextTick()
    vm.openvpnImportOpen = true
    await nextTick()

    const panel = wrapper.findComponent(OpenVPNImportPanel)
    expect(panel.exists()).toBe(true)
    const panelButton = (label: string) => panel.findAll('button').find((button) => button.text().replace(/\s+/g, '') === label)

    // 未应用的原文必须纳入页面级阻断，保存不得发出请求。
    await panel.find('textarea').setValue('client\nremote vpn.example.com 1194\n')
    await nextTick()
    expect(vm.checkBlockedReason).toContain('未应用')
    await vm.save()
    expect(mockCreateNode).not.toHaveBeenCalled()

    // 解析后显式应用。
    await panelButton('解析')!.trigger('click')
    await flushPromises()
    await panelButton('应用解析结果')!.trigger('click')
    await flushPromises()

    expect(vm.form.host).toBe('vpn.example.com')
    expect(vm.form.port).toBe(1194)
    expect(vm.form.protocol_json.ca).toBe('parsed-ca')
    expect(vm.form.protocol_json.proto).toBe('tcp')
    // 解析未产出的既有草稿字段不得被空响应静默删除。
    expect(vm.form.protocol_json.username).toBe('kept-user')
    expect(vm.checkBlockedReason).toBe('')
    expect((panel.find('textarea').element as HTMLTextAreaElement).value).toBe('')

    // 切换协议必须丢弃未应用原文并关闭面板。
    await panel.find('textarea').setValue('client\nremote vpn.example.com 1194\n')
    await nextTick()
    expect(vm.checkBlockedReason).toContain('未应用')
    vm.updateProtocol('ss')
    await nextTick()
    expect(vm.openvpnImportOpen).toBe(false)
    expect(wrapper.findComponent(OpenVPNImportPanel).exists()).toBe(false)
    expect(vm.checkBlockedReason).toBe('')
    wrapper.unmount()
  })

  it('Snell 版本与混淆模式切换按 reset 清空旧分支字段', async () => {
    const snellProtocol = {
      protocol: 'snell',
      label: 'Snell',
      form_schema: [
        { name: 'psk', type: 'password', required: true, label: 'PSK', group: 'auth' },
        { name: 'version', type: 'select', required: false, label: '版本', group: 'connection', selector_name: 'version', options: ['1', '2', '3', '4', '5'], default: '1' },
        { name: 'udp', type: 'bool', default: false, label: 'UDP', section: 'switches', when: { selectors: { version: ['3', '4', '5'] } }, reset_on: ['selector.version'] },
        { name: 'reuse', type: 'bool', default: false, label: '连接复用', group: 'connection', when: { selectors: { version: ['4', '5'] } }, reset_on: ['selector.version'] },
        { name: 'obfs-mode', type: 'select', required: false, label: '混淆模式', group: 'connection', state_only: true, selector_name: 'obfs_mode', options: ['none', 'http', 'tls', 'shadow_tls', 'restls', 'jls'], default: 'none' },
        {
          name: 'obfs-opts', type: 'object', required: false, label: '混淆参数', group: 'connection', object_kind: 'fields', allow_unknown: false,
          when: { selectors: { obfs_mode: ['http', 'tls', 'shadow_tls', 'restls', 'jls'] } }, reset_on: ['selector.obfs_mode'],
          properties: [
            { name: 'host', type: 'text', required: false, label: 'Host', group: 'connection', when: { selectors: { obfs_mode: ['http', 'tls', 'shadow_tls', 'restls', 'jls'] } }, reset_on: ['selector.obfs_mode'] },
            { name: 'password', type: 'password', required: false, label: '密码', group: 'connection', when: { selectors: { obfs_mode: ['shadow_tls', 'restls', 'jls'] } }, required_when: { selectors: { obfs_mode: ['shadow_tls', 'restls', 'jls'] } }, reset_on: ['selector.obfs_mode'] },
            { name: 'version-hint', type: 'text', required: false, label: '版本提示', group: 'connection', when: { selectors: { obfs_mode: ['restls'] } }, reset_on: ['selector.obfs_mode'] },
            { name: 'username', type: 'text', required: false, label: '用户名', group: 'connection', when: { selectors: { obfs_mode: ['jls'] } }, reset_on: ['selector.obfs_mode'] },
          ],
        },
        { name: 'client-fingerprint', type: 'text', required: false, label: '客户端指纹', group: 'connection', when: { selectors: { obfs_mode: ['shadow_tls', 'restls', 'jls'] } }, reset_on: ['selector.obfs_mode'] },
      ],
      selectors: [
        { name: 'version', values: ['1', '2', '3', '4', '5'], default: '1', source_field: 'version' },
        { name: 'obfs_mode', values: ['none', 'http', 'tls', 'shadow_tls', 'restls', 'jls'], default: 'none' },
      ],
      sensitive_fields: ['psk', 'obfs-opts.password'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([snellProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'snell'
    await nextTick()

    // 默认 v1：UDP／reuse 不活动。
    expect(vm.currentState.selectors).toEqual({ version: '1', obfs_mode: 'none' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'udp')).toBe(false)
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'reuse')).toBe(false)

    // 切到 v4：UDP／reuse 出现并可编辑。
    vm.setField('version', '4')
    await nextTick()
    expect(vm.currentState.selectors).toEqual({ version: '4', obfs_mode: 'none' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'reuse')).toBe(true)

    // 切到 shadow_tls：obfs-opts 出现，切回 none 后整体清空。
    const modeField = snellProtocol.form_schema[4] as FieldSchema
    vm.setFieldModelValue(modeField, 'shadow_tls')
    await nextTick()
    vm.setField('obfs-opts', { host: 'bing.com', password: 'p' })
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'obfs-opts')).toBe(true)
    vm.setFieldModelValue(modeField, 'none')
    await nextTick()
    expect(vm.form.protocol_json['obfs-opts']).toBeUndefined()
    expect(vm.resetScopesArray()).toContain('selector.obfs_mode')
    expect(vm.form.protocol_json['obfs-mode']).toBeUndefined()
    wrapper.unmount()
  })

  it('Hysteria 认证分支互斥并在切换时清空另一组凭据', async () => {
    const hysteriaProtocol = {
      protocol: 'hysteria',
      label: 'Hysteria',
      form_schema: [
        { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['none', 'base64', 'string'], default: 'none' },
        { name: 'auth', type: 'password', required: false, label: 'Base64 认证', group: 'auth', when: { selectors: { auth_mode: ['base64'] } }, required_when: { selectors: { auth_mode: ['base64'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'auth-str', type: 'password', required: false, label: '认证字符串', group: 'auth', when: { selectors: { auth_mode: ['string'] } }, required_when: { selectors: { auth_mode: ['string'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'up', type: 'text', required: true, label: '上行带宽', group: 'connection' },
        { name: 'down', type: 'text', required: true, label: '下行带宽', group: 'connection' },
        { name: 'protocol', type: 'select', required: false, label: '传输协议', group: 'connection', options: ['udp', 'wechat-video', 'faketcp'], default: 'udp' },
      ],
      selectors: [{ name: 'auth_mode', values: ['none', 'base64', 'string'], default: 'none' }],
      sensitive_fields: ['auth', 'auth-str'],
      link_mappings: { sr: true, generic: true },
    }
    mockGetProtocols.mockResolvedValue([hysteriaProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'hysteria'
    await nextTick()

    expect(vm.currentState.selectors).toEqual({ auth_mode: 'none' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'auth')).toBe(false)

    const modeField = hysteriaProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'base64')
    await nextTick()
    vm.setField('auth', 'dGVzdC1hdXRo')
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'auth')).toBe(true)
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'auth-str')).toBe(false)

    vm.setFieldModelValue(modeField, 'string')
    await nextTick()
    expect(vm.form.protocol_json.auth).toBeUndefined()
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')
    expect(vm.form.protocol_json['auth-mode']).toBeUndefined()
    wrapper.unmount()
  })

  it('Hysteria2 端口模式隐藏顶层端口并按分支清空端口组与混淆字段', async () => {
    const hysteria2Protocol = {
      protocol: 'hysteria2',
      label: 'Hysteria2',
      form_schema: [
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
        { name: 'endpoint-mode', type: 'select', required: false, label: '端口模式', group: 'connection', state_only: true, selector_name: 'endpoint_mode', options: ['single', 'ports'], default: 'single' },
        { name: 'obfs-mode', type: 'select', required: false, label: '混淆模式', group: 'connection', state_only: true, selector_name: 'obfs_mode', options: ['none', 'salamander', 'gecko'], default: 'none' },
        { name: 'ports', type: 'text', required: false, label: '端口组', group: 'connection', when: { selectors: { endpoint_mode: ['ports'] } }, required_when: { selectors: { endpoint_mode: ['ports'] } }, reset_on: ['selector.endpoint_mode'] },
        { name: 'hop-interval', type: 'text', required: false, label: 'Hop 间隔', group: 'connection', when: { selectors: { endpoint_mode: ['ports'] } }, reset_on: ['selector.endpoint_mode'] },
        { name: 'obfs-password', type: 'password', required: false, label: '混淆密码', group: 'connection', when: { selectors: { obfs_mode: ['salamander', 'gecko'] } }, required_when: { selectors: { obfs_mode: ['salamander', 'gecko'] } }, reset_on: ['selector.obfs_mode'] },
      ],
      selectors: [
        { name: 'endpoint_mode', values: ['single', 'ports'], default: 'single' },
        { name: 'obfs_mode', values: ['none', 'salamander', 'gecko'], default: 'none' },
      ],
      endpoint_policies: [
        { when: { selectors: { endpoint_mode: ['single'] } }, host_mode: 'required', port_mode: 'required', emit_host: true, emit_port: true },
        { when: { selectors: { endpoint_mode: ['ports'] } }, host_mode: 'required', port_mode: 'hidden', emit_host: true, emit_port: false },
      ],
      sensitive_fields: ['password', 'obfs-password'],
      link_mappings: { sr: true, generic: true },
    }
    mockGetProtocols.mockResolvedValue([hysteria2Protocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'hysteria2'
    await nextTick()

    // single 模式：显示端口，不显示端口组。
    expect(vm.currentState.selectors).toEqual({ endpoint_mode: 'single', obfs_mode: 'none' })
    expect(vm.showPortField).toBe(true)
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'ports')).toBe(false)

    // 切到 ports：隐藏顶层端口，端口组出现。
    const endpointField = hysteria2Protocol.form_schema[1] as FieldSchema
    vm.setFieldModelValue(endpointField, 'ports')
    await nextTick()
    expect(vm.showPortField).toBe(false)
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'ports')).toBe(true)
    vm.setField('ports', '1000-2000')
    await nextTick()

    // 切回 single：端口组被清空，顶层端口恢复。
    vm.setFieldModelValue(endpointField, 'single')
    await nextTick()
    expect(vm.form.protocol_json.ports).toBeUndefined()
    expect(vm.showPortField).toBe(true)
    expect(vm.resetScopesArray()).toContain('selector.endpoint_mode')

    // 混淆分支：salamander 出现密码字段，切回 none 清空。
    const obfsField = hysteria2Protocol.form_schema[2] as FieldSchema
    vm.setFieldModelValue(obfsField, 'salamander')
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'obfs-password')).toBe(true)
    vm.setField('obfs-password', 'p')
    await nextTick()
    vm.setFieldModelValue(obfsField, 'none')
    await nextTick()
    expect(vm.form.protocol_json['obfs-password']).toBeUndefined()
    wrapper.unmount()
  })

  it('TUIC v4/v5 凭据互斥且 UOT 版本随开关清空', async () => {
    const tuicProtocol = {
      protocol: 'tuic',
      label: 'TUIC',
      form_schema: [
        { name: 'auth-mode', type: 'select', required: false, label: '认证方式', group: 'auth', state_only: true, selector_name: 'auth_mode', options: ['v4', 'v5'], default: 'v5' },
        { name: 'token', type: 'password', required: false, label: 'Token', group: 'auth', when: { selectors: { auth_mode: ['v4'] } }, required_when: { selectors: { auth_mode: ['v4'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'uuid', type: 'password', required: false, label: 'UUID', group: 'auth', when: { selectors: { auth_mode: ['v5'] } }, required_when: { selectors: { auth_mode: ['v5'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'password', type: 'password', required: false, label: '密码', group: 'auth', when: { selectors: { auth_mode: ['v5'] } }, required_when: { selectors: { auth_mode: ['v5'] } }, reset_on: ['selector.auth_mode'] },
        { name: 'udp-over-stream', type: 'bool', default: false, label: 'UDP over Stream', section: 'switches', feature: { name: 'udp-over-stream' }, reset_on: ['feature.udp-over-stream'] },
        { name: 'udp-over-stream-version', type: 'select', required: false, label: 'UDP over Stream 版本', group: 'connection', options: ['1', '2'], default: '1', when: { features: ['udp-over-stream'] }, reset_on: ['feature.udp-over-stream'] },
      ],
      selectors: [{ name: 'auth_mode', values: ['v4', 'v5'], default: 'v5' }],
      sensitive_fields: ['token', 'uuid', 'password'],
      link_mappings: { sr: true, generic: true },
    }
    mockGetProtocols.mockResolvedValue([tuicProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'tuic'
    await nextTick()

    expect(vm.currentState.selectors).toEqual({ auth_mode: 'v5' })
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'token')).toBe(false)
    vm.setField('uuid', '11111111-2222-3333-4444-555555555555')
    vm.setField('password', 'pw')
    await nextTick()

    // 切到 v4：uuid/password 被清空，token 出现。
    const modeField = tuicProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'v4')
    await nextTick()
    expect(vm.form.protocol_json.uuid).toBeUndefined()
    expect(vm.form.protocol_json.password).toBeUndefined()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'token')).toBe(true)
    expect(vm.resetScopesArray()).toContain('selector.auth_mode')

    // UOT：开启后版本出现，关闭后版本清空。
    vm.setField('udp-over-stream', true)
    await nextTick()
    expect(wrapper.findAllComponents(ProtocolFieldEditor).some((item) => item.props('field').name === 'udp-over-stream-version')).toBe(true)
    vm.setField('udp-over-stream-version', '2')
    await nextTick()
    vm.setField('udp-over-stream', false)
    await nextTick()
    expect(vm.form.protocol_json['udp-over-stream-version']).toBeUndefined()
    wrapper.unmount()
  })


  // Build32 Step 11：WireGuard 单 Peer／多 Peer 切换、endpoint policy、reserved 与 Peer 稳定身份。
  it('WireGuard 单/多 Peer 切换隐藏 endpoint、清空对侧字段并保留 reserved 三字节编辑', async () => {
    const wgProtocol = {
      protocol: 'wireguard',
      label: 'WireGuard',
      form_schema: [
        { name: 'peer-mode', type: 'select', required: false, label: 'Peer 模式', group: 'connection', state_only: true, selector_name: 'peer_mode', options: ['single', 'peers'], default: 'single' },
        { name: 'private-key', type: 'secret-multiline', required: true, label: '私钥', group: 'auth' },
        { name: 'public-key', type: 'text', required: false, label: '公钥', group: 'auth', when: { selectors: { peer_mode: ['single'] } }, required_when: { selectors: { peer_mode: ['single'] } }, reset_on: ['selector.peer_mode'] },
        { name: 'reserved', type: 'byte-sequence', required: false, label: '保留字节', group: 'advanced', when: { selectors: { peer_mode: ['single'] } }, reset_on: ['selector.peer_mode'] },
        { name: 'allowed-ips', type: 'text-list', required: false, label: 'Allowed IPs', group: 'advanced', when: { selectors: { peer_mode: ['single'] } }, reset_on: ['selector.peer_mode'] },
        {
          name: 'peers', type: 'object', required: false, label: 'Peer 列表', group: 'connection',
          object_kind: 'list', item_id_field: '_credential_id', allow_unknown: false,
          when: { selectors: { peer_mode: ['peers'] } }, required_when: { selectors: { peer_mode: ['peers'] } }, reset_on: ['selector.peer_mode'],
          properties: [
            { name: 'server', type: 'text', required: true, label: '服务器' },
            { name: 'port', type: 'number', required: true, label: '端口' },
            { name: 'public-key', type: 'text', required: true, label: '公钥' },
            { name: 'pre-shared-key', type: 'password', required: false, label: '预共享密钥' },
            { name: 'reserved', type: 'byte-sequence', required: false, label: '保留字节' },
            { name: 'allowed-ips', type: 'text-list', required: true, label: 'Allowed IPs' },
          ],
        },
        { name: 'ip', type: 'text', required: false, label: 'IP', group: 'advanced' },
      ],
      selectors: [{ name: 'peer_mode', values: ['single', 'peers'], default: 'single' }],
      endpoint_policies: [
        { when: { selectors: { peer_mode: ['single'] } }, host_mode: 'required', port_mode: 'required', emit_host: true, emit_port: true },
        { when: { selectors: { peer_mode: ['peers'] } }, host_mode: 'hidden', port_mode: 'hidden', emit_host: false, emit_port: false },
      ],
      sensitive_fields: ['private-key', 'pre-shared-key', 'peers[].pre-shared-key'],
      link_mappings: { sr: true, generic: true },
    }
    mockGetProtocols.mockResolvedValue([wgProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'wireguard'
    vm.form.host = 'example.com'
    vm.form.port = 51820
    await nextTick()

    // single：显示 endpoint 与顶层 Peer 字段，不显示 peers 列表。
    expect(vm.currentState.selectors).toEqual({ peer_mode: 'single' })
    expect(vm.showHostField).toBe(true)
    expect(vm.showPortField).toBe(true)
    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    expect(fieldNames()).toContain('public-key')
    expect(fieldNames()).not.toContain('peers')

    // 顶层 reserved：三整数控件可编辑并写入规范数组。
    vm.setField('reserved', [1, 2, 3])
    await nextTick()
    expect(vm.form.protocol_json.reserved).toEqual([1, 2, 3])

    // 切到 peers：隐藏 endpoint 并清空顶层 Peer 字段。
    vm.setFieldModelValue(wgProtocol.form_schema[0] as FieldSchema, 'peers')
    await nextTick()
    expect(vm.showHostField).toBe(false)
    expect(vm.showPortField).toBe(false)
    expect(vm.form.host).toBe('')
    expect(vm.form.port).toBe(0)
    expect(vm.form.protocol_json['public-key']).toBeUndefined()
    expect(vm.form.protocol_json.reserved).toBeUndefined()
    expect(fieldNames()).toContain('peers')
    expect(vm.resetScopesArray()).toContain('selector.peer_mode')

    // 切回 single：peers 列表清空，顶层字段恢复且不复活旧值。
    vm.setFieldModelValue(wgProtocol.form_schema[0] as FieldSchema, 'single')
    await nextTick()
    expect(vm.form.protocol_json.peers).toBeUndefined()
    expect(vm.showPortField).toBe(true)
    expect(vm.form.protocol_json.reserved).toBeUndefined()
    wrapper.unmount()
  })


  // Build32 Step 12：Mieru 单端口／端口段切换、隐藏端口与完整枚举。
  it('Mieru 端口段模式隐藏顶层端口并按 selector 清空 port-range', async () => {
    const mieruProtocol = {
      protocol: 'mieru',
      label: 'Mieru',
      form_schema: [
        { name: 'endpoint-mode', type: 'select', required: false, label: '端口模式', group: 'connection', state_only: true, selector_name: 'endpoint_mode', options: ['single', 'range'], default: 'single' },
        { name: 'port-range', type: 'text', required: false, label: '端口范围', group: 'connection', when: { selectors: { endpoint_mode: ['range'] } }, required_when: { selectors: { endpoint_mode: ['range'] } }, reset_on: ['selector.endpoint_mode'] },
        { name: 'username', type: 'text', required: true, label: '用户名', group: 'auth' },
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
        { name: 'transport', type: 'select', required: false, label: '传输', group: 'connection', options: ['TCP', 'UDP'], default: 'TCP' },
        { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
        { name: 'multiplexing', type: 'select', required: false, label: '多路复用', group: 'connection', options: ['', 'MULTIPLEXING_OFF', 'MULTIPLEXING_LOW', 'MULTIPLEXING_MIDDLE', 'MULTIPLEXING_HIGH'], default: '' },
        { name: 'handshake-mode', type: 'select', required: false, label: '握手模式', group: 'connection', options: ['', 'HANDSHAKE_STANDARD', 'HANDSHAKE_NO_WAIT'], default: '' },
        { name: 'traffic-pattern', type: 'text', required: false, label: '流量特征', group: 'advanced' },
      ],
      selectors: [{ name: 'endpoint_mode', values: ['single', 'range'], default: 'single' }],
      endpoint_policies: [
        { when: { selectors: { endpoint_mode: ['single'] } }, host_mode: 'required', port_mode: 'required', emit_host: true, emit_port: true },
        { when: { selectors: { endpoint_mode: ['range'] } }, host_mode: 'required', port_mode: 'hidden', emit_host: true, emit_port: false },
      ],
      sensitive_fields: ['password'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([mieruProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'mieru'
    vm.form.port = 8964
    await nextTick()

    // 完整枚举来自后端 schema，旧值 LOW/MIDDLE/HIGH 不再出现。
    expect(vm.currentSchema().form_schema.find((field: FieldSchema) => field.name === 'multiplexing').options)
      .toEqual(['', 'MULTIPLEXING_OFF', 'MULTIPLEXING_LOW', 'MULTIPLEXING_MIDDLE', 'MULTIPLEXING_HIGH'])
    expect(vm.currentState.selectors).toEqual({ endpoint_mode: 'single' })
    expect(vm.showPortField).toBe(true)
    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    expect(fieldNames()).not.toContain('port-range')

    // 切到 range：隐藏顶层端口，port-range 出现并规范化 port=0。
    vm.setFieldModelValue(mieruProtocol.form_schema[0] as FieldSchema, 'range')
    await nextTick()
    expect(vm.showPortField).toBe(false)
    expect(vm.form.port).toBe(0)
    expect(fieldNames()).toContain('port-range')
    vm.setField('port-range', '1000-2000')
    await nextTick()
    expect(vm.form.protocol_json['port-range']).toBe('1000-2000')

    // 切回 single：port-range 清空且不恢复，端口恢复显示。
    vm.setFieldModelValue(mieruProtocol.form_schema[0] as FieldSchema, 'single')
    await nextTick()
    expect(vm.form.protocol_json['port-range']).toBeUndefined()
    expect(vm.showPortField).toBe(true)
    expect(vm.resetScopesArray()).toContain('selector.endpoint_mode')

    // traffic-pattern 是普通文本字段（非 secret），错误定位由后端返回。
    vm.setField('traffic-pattern', '!!!bad!!!')
    await nextTick()
    expect(vm.form.protocol_json['traffic-pattern']).toBe('!!!bad!!!')
    wrapper.unmount()
  })


  // Build32 Step 13：MASQUE 三种网络模式、h3-l4proxy 强制关闭 UDP 与 QUIC 调优清空。
  it('MASQUE 三种网络模式切换、h3-l4proxy 关闭 UDP 并清空 QUIC 调优', async () => {
    const masqueProtocol = {
      protocol: 'masque',
      label: 'MASQUE',
      form_schema: [
        { name: 'network-mode', type: 'select', required: false, label: '网络模式', group: 'connection', state_only: true, selector_name: 'network_mode', options: ['quic', 'h2', 'h3_l4proxy'], default: 'quic' },
        { name: 'private-key', type: 'secret-multiline', required: true, label: '私钥', group: 'auth' },
        { name: 'public-key', type: 'text', required: true, label: '公钥', group: 'auth' },
        { name: 'ip', type: 'text', required: false, label: 'IP', group: 'advanced' },
        { name: 'uri', type: 'text', required: false, label: '连接 URI', group: 'advanced' },
        { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches', when: { selectors: { network_mode: ['quic', 'h2'] } }, reset_on: ['selector.network_mode'] },
        { name: 'congestion-controller', type: 'select', required: false, label: '拥塞控制器', group: 'advanced', options: ['', 'cubic', 'new_reno', 'bbr_meta_v1', 'bbr_meta_v2', 'bbr'], default: '', when: { selectors: { network_mode: ['quic'] } }, reset_on: ['selector.network_mode'] },
        { name: 'cwnd', type: 'number', required: false, label: '拥塞窗口', group: 'advanced', when: { selectors: { network_mode: ['quic'] } }, reset_on: ['selector.network_mode'] },
        { name: 'skip-cert-verify', type: 'bool', default: false, label: '跳过证书校验', section: 'switches' },
      ],
      selectors: [{ name: 'network_mode', values: ['quic', 'h2', 'h3_l4proxy'], default: 'quic' }],
      sensitive_fields: ['private-key'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([masqueProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'masque'
    vm.form.host = 'example.com'
    vm.form.port = 443
    await nextTick()

    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    expect(vm.currentState.selectors).toEqual({ network_mode: 'quic' })
    expect(fieldNames()).toContain('udp')
    expect(fieldNames()).toContain('congestion-controller')
    vm.setField('udp', true)
    vm.setField('congestion-controller', 'bbr_meta_v2')
    vm.setField('cwnd', 64)
    await nextTick()

    // h2：保留 UDP，清空 QUIC 调优。
    vm.setFieldModelValue(masqueProtocol.form_schema[0] as FieldSchema, 'h2')
    await nextTick()
    expect(fieldNames()).not.toContain('congestion-controller')
    expect(vm.form.protocol_json['congestion-controller']).toBeUndefined()
    expect(vm.form.protocol_json.cwnd).toBeUndefined()

    // h3_l4proxy：UDP 字段隐藏并清空，不得保留 true。
    vm.setFieldModelValue(masqueProtocol.form_schema[0] as FieldSchema, 'h3_l4proxy')
    await nextTick()
    expect(fieldNames()).not.toContain('udp')
    expect(vm.form.protocol_json.udp).toBeUndefined()
    expect(vm.resetScopesArray()).toContain('selector.network_mode')

    // 切回 quic：旧 udp／调优值都不恢复。
    vm.setFieldModelValue(masqueProtocol.form_schema[0] as FieldSchema, 'quic')
    await nextTick()
    expect(vm.form.protocol_json.udp).toBeUndefined()
    expect(vm.form.protocol_json['congestion-controller']).toBeUndefined()
    expect(fieldNames()).toContain('udp')
    wrapper.unmount()
  })


  // Build32 Step 14：Tailscale 无 endpoint、non_empty 依赖与三态 bool。
  it('Tailscale 隐藏 endpoint、按 exit-node 条件显示 LAN access 并保留三态 bool', async () => {
    const tailscaleProtocol = {
      protocol: 'tailscale',
      label: 'Tailscale',
      form_schema: [
        { name: 'hostname', type: 'text', required: false, label: '设备名', group: 'basic' },
        { name: 'auth-key', type: 'password', required: false, label: '认证密钥', group: 'auth' },
        { name: 'control-url', type: 'text', required: false, label: '控制面地址', group: 'connection' },
        { name: 'ephemeral', type: 'bool', default: false, label: '临时节点', section: 'switches' },
        { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
        { name: 'accept-routes', type: 'bool', required: false, label: '接受路由', group: 'switches' },
        { name: 'exit-node', type: 'text', required: false, label: '出口节点', group: 'connection' },
        { name: 'exit-node-allow-lan-access', type: 'bool', required: false, label: '出口节点允许 LAN 访问', group: 'switches', when: { non_empty: ['exit-node'] } },
      ],
      endpoint_policies: [
        { host_mode: 'hidden', port_mode: 'hidden', emit_host: false, emit_port: false },
      ],
      sensitive_fields: ['auth-key'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([tailscaleProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'tailscale'
    await nextTick()

    // 无 endpoint：host／port 不显示，且 request 载荷规范化。
    expect(vm.showHostField).toBe(false)
    expect(vm.showPortField).toBe(false)
    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    // 三态 bool 不进入集中开关区，而在所属分组内以三态控件渲染。
    expect(vm.switchFields.map((item: { path: string }) => item.path)).toEqual(['ephemeral', 'udp'])
    expect(fieldNames()).toContain('accept-routes')

    // exit-node 为空时 LAN access 不显示。
    expect(fieldNames()).not.toContain('exit-node-allow-lan-access')
    vm.setField('exit-node', '100.64.0.1')
    await nextTick()
    expect(fieldNames()).toContain('exit-node-allow-lan-access')

    // 三态：未设置 → 关闭 → 开启，且不写成 false 吞掉未设置。
    const lanField = tailscaleProtocol.form_schema[7] as FieldSchema
    expect(vm.fieldModelValue(lanField)).toBe(undefined)
    vm.setFieldModelValue(lanField, false)
    await nextTick()
    expect(vm.form.protocol_json['exit-node-allow-lan-access']).toBe(false)
    vm.setFieldModelValue(lanField, true)
    await nextTick()
    expect(vm.form.protocol_json['exit-node-allow-lan-access']).toBe(true)
    vm.setFieldModelValue(lanField, undefined)
    await nextTick()
    expect(vm.form.protocol_json['exit-node-allow-lan-access']).toBe(undefined)

    // 清空 exit-node 后 LAN access 立即隐藏。
    vm.setField('exit-node', '')
    await nextTick()
    expect(fieldNames()).not.toContain('exit-node-allow-lan-access')
    wrapper.unmount()
  })


  // Build32 Step 15：AnyTLS 四种安全模式互斥、主密码保留、ECH 与 mTLS。
  it('AnyTLS 三种附加伪装互斥、主密码保留且 ECH 关闭清空子字段', async () => {
    const anytlsProtocol = {
      protocol: 'anytls',
      label: 'AnyTLS',
      form_schema: [
        { name: 'security-mode', type: 'select', required: false, label: '附加安全', group: 'connection', state_only: true, selector_name: 'security_mode', options: ['plain', 'shadow_tls', 'restls', 'jls'], default: 'plain' },
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
        { name: 'sni', type: 'text', required: false, label: 'SNI', group: 'security' },
        { name: 'ech-opts', type: 'object', required: false, label: 'ECH 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, feature: { name: 'ech', toggle: 'enable' }, reset_on: ['feature.ech'], properties: [
          { name: 'enable', type: 'bool', default: false, label: '启用' },
          { name: 'config', type: 'text', required: false, label: '配置', when: { features: ['ech'] }, reset_on: ['feature.ech'] },
          { name: 'query-server-name', type: 'text', required: false, label: '查询服务器名', when: { features: ['ech'] }, reset_on: ['feature.ech'] },
        ] },
        { name: 'certificate', type: 'multiline', required: false, label: '证书', group: 'security' },
        { name: 'private-key', type: 'secret-multiline', required: false, label: '私钥', group: 'security' },
        { name: 'shadow-tls-opts', type: 'object', required: false, label: 'ShadowTLS 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, when: { selectors: { security_mode: ['shadow_tls'] } }, required_when: { selectors: { security_mode: ['shadow_tls'] } }, reset_on: ['selector.security_mode'], properties: [
          { name: 'password', type: 'password', required: false, label: '密码', when: { selectors: { security_mode: ['shadow_tls'] } }, required_when: { selectors: { security_mode: ['shadow_tls'] } }, reset_on: ['selector.security_mode'] },
          { name: 'version', type: 'select', required: false, label: '版本', options: ['', '1', '2', '3'], default: '', when: { selectors: { security_mode: ['shadow_tls'] } }, reset_on: ['selector.security_mode'] },
        ] },
        { name: 'restls-opts', type: 'object', required: false, label: 'Restls 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, when: { selectors: { security_mode: ['restls'] } }, required_when: { selectors: { security_mode: ['restls'] } }, reset_on: ['selector.security_mode'], properties: [
          { name: 'password', type: 'password', required: false, label: '密码', when: { selectors: { security_mode: ['restls'] } }, required_when: { selectors: { security_mode: ['restls'] } }, reset_on: ['selector.security_mode'] },
          { name: 'version-hint', type: 'select', required: false, label: '版本提示', options: ['', 'tls12', 'tls13'], default: '', when: { selectors: { security_mode: ['restls'] } }, required_when: { selectors: { security_mode: ['restls'] } }, reset_on: ['selector.security_mode'] },
        ] },
        { name: 'jls-opts', type: 'object', required: false, label: 'JLS 参数', group: 'connection', object_kind: 'fields', allow_unknown: false, when: { selectors: { security_mode: ['jls'] } }, required_when: { selectors: { security_mode: ['jls'] } }, reset_on: ['selector.security_mode'], properties: [
          { name: 'username', type: 'text', required: false, label: '用户名', when: { selectors: { security_mode: ['jls'] } }, required_when: { selectors: { security_mode: ['jls'] } }, reset_on: ['selector.security_mode'] },
          { name: 'password', type: 'password', required: false, label: '密码', when: { selectors: { security_mode: ['jls'] } }, required_when: { selectors: { security_mode: ['jls'] } }, reset_on: ['selector.security_mode'] },
        ] },
      ],
      selectors: [{ name: 'security_mode', values: ['plain', 'shadow_tls', 'restls', 'jls'], default: 'plain' }],
      sensitive_fields: ['password', 'private-key', 'shadow-tls-opts.password', 'restls-opts.password', 'jls-opts.password'],
      link_mappings: { sr: true, generic: true },
    }
    mockGetProtocols.mockResolvedValue([anytlsProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'anytls'
    await nextTick()

    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    expect(vm.currentState.selectors).toEqual({ security_mode: 'plain' })
    for (const name of ['shadow-tls-opts', 'restls-opts', 'jls-opts']) {
      expect(fieldNames()).not.toContain(name)
    }

    // 主密码在切换分支时保留（不留空即为待替换；此处仅验证字段始终可编辑且不被清空）。
    vm.setField('password', 'main-secret')
    await nextTick()
    const modeField = anytlsProtocol.form_schema[0] as FieldSchema
    vm.setFieldModelValue(modeField, 'shadow_tls')
    await nextTick()
    expect(vm.form.protocol_json.password).toBe('main-secret')
    expect(fieldNames()).toContain('shadow-tls-opts')
    expect(fieldNames()).not.toContain('restls-opts')
    vm.setField('shadow-tls-opts', { password: 'shadow-secret', version: '3' })
    await nextTick()

    // 切换到 restls：旧伪装对象必须清空且主密码保留。
    vm.setFieldModelValue(modeField, 'restls')
    await nextTick()
    expect(vm.form.protocol_json['shadow-tls-opts']).toBeUndefined()
    expect(fieldNames()).toContain('restls-opts')
    expect(vm.form.protocol_json.password).toBe('main-secret')

    // 切到 jls：restls 清空。
    vm.setFieldModelValue(modeField, 'jls')
    await nextTick()
    expect(vm.form.protocol_json['restls-opts']).toBeUndefined()
    expect(fieldNames()).toContain('jls-opts')
    expect(vm.resetScopesArray()).toContain('selector.security_mode')

    // 回到 plain：三类对象全部消失。
    vm.setFieldModelValue(modeField, 'plain')
    await nextTick()
    for (const name of ['shadow-tls-opts', 'restls-opts', 'jls-opts']) {
      expect(vm.form.protocol_json[name]).toBeUndefined()
      expect(fieldNames()).not.toContain(name)
    }

    // ECH 关闭清空 config／query-server-name。
    vm.setField('ech-opts', { enable: true, config: 'cfg', 'query-server-name': 'q.example.com' })
    await nextTick()
    expect(vm.form.protocol_json['ech-opts'].config).toBe('cfg')
    vm.setField('ech-opts', { enable: false })
    await nextTick()
    expect(vm.form.protocol_json['ech-opts'].config).toBeUndefined()
    expect(vm.form.protocol_json['ech-opts']['query-server-name']).toBeUndefined()
    wrapper.unmount()
  })


  // Build32 Step 16：ShadowQUIC QUIC 版本列表、UOT／0-RTT 独立开关与流控字段。
  it('ShadowQUIC 版本列表保序去重、UOT 与 0-RTT 独立且不改写保存结果', async () => {
    const shadowquicProtocol = {
      protocol: 'shadowquic',
      label: 'ShadowQUIC',
      form_schema: [
        { name: 'username', type: 'text', required: true, label: '用户名', group: 'auth' },
        { name: 'password', type: 'password', required: true, label: '密码', group: 'auth' },
        { name: 'sni', type: 'text', required: false, label: 'SNI', group: 'security' },
        { name: 'alpn', type: 'text-list', required: false, label: 'ALPN', group: 'security' },
        { name: 'quic-versions', type: 'text-list', required: false, label: 'QUIC 版本', group: 'connection' },
        { name: 'udp-over-stream', type: 'bool', default: false, label: 'UDP over Stream', section: 'switches' },
        { name: 'zero-rtt', type: 'bool', default: false, label: '0-RTT', section: 'switches' },
        { name: 'keep-alive-interval', type: 'number', required: false, label: '保活间隔', group: 'advanced' },
        { name: 'recv-window-conn', type: 'number', required: false, label: '连接接收窗口', group: 'advanced' },
        { name: 'recv-window', type: 'number', required: false, label: '接收窗口', group: 'advanced' },
        { name: 'up', type: 'text', required: false, label: '上行带宽', group: 'advanced' },
      ],
      sensitive_fields: ['password'],
      link_mappings: { sr: false, generic: false },
    }
    mockGetProtocols.mockResolvedValue([shadowquicProtocol])
    const wrapper = mount(NodesView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.protocol = 'shadowquic'
    await nextTick()

    const fieldNames = () => wrapper.findAllComponents(ProtocolFieldEditor).map((item) => item.props('field').name)
    // 不得从通用 TLS helper 误加固定 tag 没有的字段。
    for (const forbidden of ['skip-cert-verify', 'certificate', 'private-key', 'ech-opts', 'udp-over-stream-version']) {
      expect(fieldNames()).not.toContain(forbidden)
    }
    expect(fieldNames()).toContain('quic-versions')

    // 版本列表保序去重。
    vm.setField('quic-versions', ['v2', 'v1', 'v2'])
    await nextTick()
    expect(vm.form.protocol_json['quic-versions']).toEqual(['v2', 'v1', 'v2'])

    // UOT 与 0-RTT 是独立开关，且不产生附加版本字段。
    vm.setField('udp-over-stream', true)
    vm.setField('zero-rtt', true)
    await nextTick()
    expect(vm.form.protocol_json['udp-over-stream']).toBe(true)
    expect(vm.form.protocol_json['zero-rtt']).toBe(true)
    expect(vm.form.protocol_json['udp-over-stream-version']).toBeUndefined()

    // 关闭 UOT 不影响 0-RTT 与版本列表。
    vm.setField('udp-over-stream', false)
    await nextTick()
    expect(vm.form.protocol_json['udp-over-stream']).toBe(false)
    expect(vm.form.protocol_json['zero-rtt']).toBe(true)
    expect(vm.form.protocol_json['quic-versions']).toEqual(['v2', 'v1', 'v2'])

    // 流控字段保持“未设置”与 0 的区别。
    expect(vm.form.protocol_json['recv-window']).toBeUndefined()
    vm.setField('recv-window', 0)
    await nextTick()
    expect(vm.form.protocol_json['recv-window']).toBe(0)
    wrapper.unmount()
  })

})
