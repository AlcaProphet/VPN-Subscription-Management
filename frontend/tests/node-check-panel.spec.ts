// node-check-panel.spec.ts：Build19 Step 5 目标检查 UI 测试。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('@/api/node', () => ({
  checkNode: vi.fn(),
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

import NodeCheckPanel from '@/components/NodeCheckPanel.vue'
import { checkNode } from '@/api/node'

const mockCheckNode = checkNode as unknown as ReturnType<typeof vi.fn>

const request = {
  protocol: 'vless',
  host: 'example.com',
  port: 443,
  protocol_json: { network: 'tcp' },
  current_state: { network: 'tcp', security: 'tls' },
  targets: ['clash-yaml', 'sr-subs', 'generic-subs'],
}

const response = {
  check_id: 'chk-1',
  check_version: 1,
  targets: {
    'clash-yaml': {
      status: 'ok',
      preview: 'proxies:\n  - name: demo',
      diagnostics: [],
    },
    'sr-subs': {
      status: 'skip',
      preview: null,
      diagnostics: [
        { severity: 'error', code: 'core_semantic_unexpressible', target: 'sr-subs', field_path: 'network', message: '无法表达', evidence: 'cvr' },
      ],
    },
  },
}

describe('NodeCheckPanel', () => {
  beforeEach(() => {
    mockCheckNode.mockReset()
    mockCheckNode.mockResolvedValue(response)
  })

  it('点击检查时携带当前草稿请求并展示目标状态', async () => {
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    expect(button).toBeTruthy()
    await button!.trigger('click')
    await flushPromises()

    expect(mockCheckNode).toHaveBeenCalledWith(request)
    expect(wrapper.text()).toContain('clash-yaml')
    expect(wrapper.text()).toContain('core_semantic_unexpressible')
    wrapper.unmount()
  })

  it('草稿变化后旧检查结果立即失效', async () => {
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    await button!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('clash-yaml')

    await wrapper.setProps({ request: { ...request, protocol_json: { network: 'ws' } } })
    await nextTick()

    expect(wrapper.text()).not.toContain('clash-yaml')
    wrapper.unmount()
  })

  it('迟到旧响应不会覆盖新草稿状态', async () => {
    let resolveOld!: (value: unknown) => void
    mockCheckNode.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    await button!.trigger('click')

    await wrapper.setProps({ request: { ...request, protocol_json: { network: 'grpc' } } })
    resolveOld(response)
    await flushPromises()

    expect(wrapper.text()).not.toContain('clash-yaml')
    wrapper.unmount()
  })
})
