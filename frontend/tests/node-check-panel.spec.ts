// node-check-panel.spec.ts：Build19 Step 5 目标检查 UI 测试。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

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
import { ApiError } from '@/api/request'

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
      diagnostics: null,
    },
    'sr-subs': {
      status: 'skip',
      preview: null,
      diagnostics: [
        { severity: 'error', code: 'core_semantic_unexpressible', target: 'sr-subs', field_path: 'network', message: '无法表达', evidence: 'cvr' },
      ],
    },
    'generic-subs': { status: 'warn', preview: null, diagnostics: [] },
  },
}

describe('NodeCheckPanel', () => {
  beforeEach(() => {
    mockCheckNode.mockReset()
    mockCheckNode.mockResolvedValue(response)
  })

  it('携带当前草稿检查，默认显示汇总，展开后才显示诊断和脱敏产物入口', async () => {
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    expect(button).toBeTruthy()
    expect(wrapper.find('details').exists()).toBe(false)
    await button!.trigger('click')
    await flushPromises()

    expect(mockCheckNode).toHaveBeenCalledWith(request)
    const details = wrapper.get('details')
    const summary = details.get('summary')
    expect(details.element.open).toBe(false)
    expect(summary.text()).toBe('检查结果（3 个目标） · 通过 1 · 警告 1 · 错误 0 · 跳过 1')
    expect(wrapper.get('[role="status"]').text()).toBe(summary.text())
    await summary.trigger('click')
    expect(details.element.open).toBe(true)
    expect(wrapper.text()).toContain('clash-yaml')
    expect(wrapper.text()).toContain('未发现诊断')
    expect(wrapper.text()).toContain('core_semantic_unexpressible')
    expect(details.get('details').element.open).toBe(false)
    await summary.trigger('click')
    expect(details.element.open).toBe(false)
    wrapper.unmount()
  })

  it('草稿变化后旧检查结果立即失效', async () => {
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    await button!.trigger('click')
    await flushPromises()
    await wrapper.get('details > summary').trigger('click')
    expect(wrapper.get('details').element.open).toBe(true)

    await wrapper.setProps({ request: { ...request, protocol_json: { network: 'ws' } } })
    expect(wrapper.find('details').exists()).toBe(false)

    await button!.trigger('click')
    await flushPromises()
    expect(wrapper.get('details').element.open).toBe(false)
    wrapper.unmount()
  })

  it('重新检查立即移除已展开的上一轮结果，新结果默认折叠', async () => {
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.get('button')
    await button.trigger('click')
    await flushPromises()
    await wrapper.get('details > summary').trigger('click')
    expect(wrapper.get('details').element.open).toBe(true)

    let resolveRetry!: (value: unknown) => void
    mockCheckNode.mockImplementationOnce(() => new Promise((resolve) => { resolveRetry = resolve }))
    await button.trigger('click')
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('[role="status"]').text()).toBe('正在检查…')

    resolveRetry(response)
    await flushPromises()
    expect(wrapper.get('details').element.open).toBe(false)
    wrapper.unmount()
  })

  it('修改草稿立即退出加载，迟到响应不会改变下一次检查的加载状态或结果', async () => {
    let resolveOld!: (value: unknown) => void
    let resolveNew!: (value: unknown) => void
    mockCheckNode.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    mockCheckNode.mockImplementationOnce(() => new Promise((resolve) => { resolveNew = resolve }))
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.get('button')
    await button.trigger('click')
    expect(wrapper.get('[role="status"]').text()).toBe('正在检查…')

    const updatedRequest = { ...request, protocol_json: { network: 'grpc' } }
    await wrapper.setProps({ request: updatedRequest })
    expect(wrapper.get('[role="status"]').text()).toBe('')
    await button.trigger('click')
    expect(mockCheckNode).toHaveBeenLastCalledWith(updatedRequest)
    resolveOld(response)
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('正在检查…')
    expect(wrapper.find('details').exists()).toBe(false)

    resolveNew(response)
    await flushPromises()
    expect(wrapper.get('details').element.open).toBe(false)
    wrapper.unmount()
  })

  it.each([
    { error: new Error('连接失败'), message: '连接失败', conflict: false },
    { error: new ApiError(409, '冲突'), message: '节点已被其他编辑更新，请重新加载后重试', conflict: true },
  ])('在结果折叠区外显示请求失败：$message', async ({ error, message, conflict }) => {
    mockCheckNode.mockRejectedValueOnce(error)
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain(message)
    expect(!!wrapper.emitted('conflict')).toBe(conflict)
    wrapper.unmount()
  })

  it('存在未应用控件草稿时禁用检查并可由用户定位草稿', async () => {
    const blockedReason = '存在未应用的自定义值或列表项草稿，请先应用或取消后再检查'
    const wrapper = mount(NodeCheckPanel, { props: { request, blockedReason } })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))!
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain(blockedReason)
    await button.trigger('click')
    expect(mockCheckNode).not.toHaveBeenCalled()
    const locate = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('定位草稿'))!
    await locate.trigger('click')
    expect(wrapper.emitted('locate')).toHaveLength(1)
  })

  it.each([false, true])('预览背景使用设计 Token（dark=%s）', async (dark) => {
    document.documentElement.classList.toggle('dark', dark)
    const wrapper = mount(NodeCheckPanel, { props: { request }, attachTo: document.body })
    const button = wrapper.findAll('button').find((b) => b.text().replace(/\s/g, '').includes('检查当前节点'))
    await button!.trigger('click')
    await flushPromises()
    const pre = wrapper.find('pre')
    expect(pre.exists()).toBe(true)
    expect(pre.classes()).toContain('bg-surface-subtle')
    expect(pre.classes()).not.toContain('bg-gray-50')
    document.documentElement.classList.remove('dark')
    wrapper.unmount()
  })

})
