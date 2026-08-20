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
import { listNodes, getProtocols } from '@/api/node'

const mockListNodes = listNodes as unknown as ReturnType<typeof vi.fn>
const mockGetProtocols = getProtocols as unknown as ReturnType<typeof vi.fn>

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
}

const protocols = [
  {
    protocol: 'ss',
    label: 'Shadowsocks',
    form_schema: [
      { name: 'cipher', type: 'text', required: true, label: '加密方式' },
      { name: 'password', type: 'password', required: true, label: '密码' },
      { name: 'udp', type: 'bool', default: true, label: 'UDP' },
    ],
    sensitive_fields: ['password'],
    link_mappings: { sr: true, generic: true },
  },
]

describe('NodesView 节点管理页', () => {
  beforeEach(() => {
    mockListNodes.mockReset()
    mockGetProtocols.mockReset()
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
    expect(document.body.querySelector('input[placeholder="留空 = 保留原凭据"]')).not.toBeNull()
    wrapper.unmount()
  })
})
