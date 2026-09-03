// nodes-view.spec.ts：节点管理页前端单测（Build5 Step5 item7 / N3）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('@/api/node', () => ({
  listNodes: vi.fn(),
  getProtocols: vi.fn(),
  createNode: vi.fn(),
  updateNode: vi.fn(),
  deleteNode: vi.fn(),
  toggleNode: vi.fn(),
  setNodeDisplayName: vi.fn(),
  importNodes: vi.fn(),
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
import { listNodes, getProtocols, createNode, updateNode } from '@/api/node'
import { ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'

const mockListNodes = listNodes as unknown as ReturnType<typeof vi.fn>
const mockGetProtocols = getProtocols as unknown as ReturnType<typeof vi.fn>
const mockCreateNode = createNode as unknown as ReturnType<typeof vi.fn>
const mockUpdateNode = updateNode as unknown as ReturnType<typeof vi.fn>

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
}

const protocols = [
  {
    protocol: 'ss',
    label: 'Shadowsocks',
    form_schema: [
      { name: 'cipher', type: 'text', required: true, label: '加密方式', section: 'transport' },
      { name: 'password', type: 'password', required: true, label: '密码', section: 'auth' },
      { name: 'udp', type: 'bool', default: true, label: 'UDP', section: 'switches' },
      { name: 'routing-mark', type: 'number', required: false, label: '路由标记', section: 'advanced' },
      {
        name: 'plugin-opts', type: 'object', required: false, label: '插件参数', section: 'transport',
        object_kind: 'fields', allow_unknown: true,
        properties: [{ name: 'host', type: 'text', required: false, label: 'Host' }],
      },
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
    ;(Notify.warning as unknown as ReturnType<typeof vi.fn>).mockClear()
    mockListNodes.mockResolvedValue([node])
    mockGetProtocols.mockResolvedValue(protocols)
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
          object_kind: 'fields', allow_unknown: true, reset_on: ['network'],
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
    expect(Notify.warning).toHaveBeenCalled()
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

  it('编辑 TLS 节点时从 current_state 回填 security', async () => {
    const editNode = {
      ...node,
      protocol: 'vless',
      protocol_json: { uuid: 'u', tls: true },
      current_state: { network: 'tcp', security: 'tls' },
    }
    mockGetProtocols.mockResolvedValue([{
      protocol: 'vless',
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
    const vm = wrapper.vm as unknown as {
      openEdit: (target: any) => void
      form: { protocol_json: Record<string, unknown> }
    }
    vm.openEdit(editNode)
    expect(vm.form.protocol_json.security).toBe('tls')
    expect(vm.form.protocol_json.tls).toBe(true)
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
    expect(Notify.warning).toHaveBeenCalled()
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
      extensionDraft: { scope: string; targets: string; label: string; payload: string }
    }
    vm.openCreate()
    vm.form.name = 'new-node'
    vm.form.host = 'example.com'
    vm.form.port = 443
    vm.extensionDraft.scope = 'node'
    vm.extensionDraft.targets = 'clash-yaml, sr-subs'
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
})
