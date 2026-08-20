// proxy-groups-view.spec.ts：代理组管理页前端单测（Build5 Step5 item7 / N3）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('@/api/proxyGroup', () => ({
  listProxyGroups: vi.fn(),
  createProxyGroup: vi.fn(),
  updateProxyGroup: vi.fn(),
  deleteProxyGroup: vi.fn(),
  togglePresetGroup: vi.fn(),
}))

vi.mock('@/api/node', () => ({
  listNodes: vi.fn(),
}))

vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import ProxyGroupsView from '@/views/admin/ProxyGroupsView.vue'
import { listProxyGroups } from '@/api/proxyGroup'
import { listNodes } from '@/api/node'
import { Notify } from '@/components/Notify'

const mockListGroups = listProxyGroups as unknown as ReturnType<typeof vi.fn>
const mockListNodes = listNodes as unknown as ReturnType<typeof vi.fn>
const mockNotifyWarning = Notify.warning as unknown as ReturnType<typeof vi.fn>
const mockNotifyError = Notify.error as unknown as ReturnType<typeof vi.fn>

const groups = [
  { id: 1, name: 'A', type: 'custom' as const, enabled: true, definition: { type: 'select' as const, nodes: [], groups: ['B'] } },
  { id: 2, name: 'B', type: 'custom' as const, enabled: true, definition: { type: 'select' as const, nodes: [], groups: ['A'] } },
]
const manualNodes = [
  { id: 1, name: 'n1', render_name: 'n1', source: 'manual' as const, host: '1.2.3.4', port: 443, protocol: 'vless', protocol_json: {}, is_public: false, enabled: true, allocatable: true, missing: false },
]
const xrayNodes = [
  { id: 2, name: 'x1', render_name: 'x1', source: 'xray' as const, host: '5.6.7.8', port: 443, protocol: 'vless', protocol_json: {}, is_public: true, enabled: true, allocatable: true, missing: false },
]

describe('ProxyGroupsView 代理组管理页', () => {
  beforeEach(() => {
    mockListGroups.mockReset()
    mockListNodes.mockReset()
    mockNotifyWarning.mockReset()
    mockNotifyError.mockReset()
    mockListGroups.mockResolvedValue(groups)
    mockListNodes.mockImplementation((source?: string) =>
      Promise.resolve(source === 'xray' ? xrayNodes : manualNodes),
    )
  })

  it('选择子组形成环时即时提示', async () => {
    const wrapper = mount(ProxyGroupsView)
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openEdit: (g: (typeof groups)[number]) => void
      onGroupRefsChange: (v: any) => void
    }
    vm.openEdit(groups[0])
    vm.onGroupRefsChange(['B'])
    expect(mockNotifyWarning).toHaveBeenCalled()
    expect(String(mockNotifyWarning.mock.calls[0][0])).toContain('代理组存在环')
    wrapper.unmount()
  })

  it('移动端节点引用降级为按钮排序，不渲染拖拽手柄', async () => {
    const wrapper = mount(ProxyGroupsView, { attachTo: document.body })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      isMobile: boolean
      openCreate: () => void
      form: { node_names: string[] }
    }
    vm.isMobile = true
    vm.openCreate()
    vm.form.node_names = ['n1']
    await nextTick()
    expect(document.body.querySelector('[draggable="true"]')).toBeNull()
    expect(document.body.textContent).toMatch(/上\s*移/)
    expect(document.body.textContent).toMatch(/下\s*移/)
    wrapper.unmount()
  })
})
