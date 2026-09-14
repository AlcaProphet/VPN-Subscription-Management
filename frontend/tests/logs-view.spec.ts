// logs-view.spec.ts：实时日志页 fetch/ReadableStream 协议边界（R28-07I Step 17）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const mocks = vi.hoisted(() => ({
  openLogStream: vi.fn(),
  notifyError: vi.fn(),
  notifyWarning: vi.fn(),
}))

vi.mock('@/api/log', () => ({
  queryAccessLogs: vi.fn().mockResolvedValue({ list: [], total: 0 }),
  clearAccessLogs: vi.fn(),
  openLogStream: mocks.openLogStream,
}))

vi.mock('@/components/Notify', () => ({
  Notify: { error: mocks.notifyError, warning: mocks.notifyWarning, success: vi.fn() },
}))

import LogsView from '@/views/admin/LogsView.vue'

function sseResponse(body: ReadableStream<Uint8Array>, status = 200): Response {
  return new Response(body, { status, headers: { 'Content-Type': 'text/event-stream' } })
}

async function openStreamTab(wrapper: ReturnType<typeof mount>) {
  const tab = wrapper.findAll('.ant-tabs-tab').find((node) => node.text().includes('实时日志流'))
  expect(tab).toBeTruthy()
  await tab!.trigger('click')
  await flushPromises()
}

describe('LogsView 实时日志流', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.openLogStream.mockReset()
    mocks.notifyError.mockReset()
    mocks.notifyWarning.mockReset()
  })

  it('源码使用 fetch/ReadableStream，不再使用 EventSource 或查询 Token', () => {
    const source = readFileSync(join(process.cwd(), 'src/views/admin/LogsView.vue'), 'utf8')
    expect(source).toContain('openLogStream')
    expect(source).toContain('createSSEParser')
    expect(source).not.toContain('EventSource')
    expect(source).not.toContain('issueStreamToken')
    expect(source).not.toContain('?token=')
  })

  it('401 停止重连并提示会话过期', async () => {
    mocks.openLogStream.mockResolvedValue(new Response('', { status: 401 }))
    const wrapper = mount(LogsView)
    await flushPromises()
    await openStreamTab(wrapper)
    expect(mocks.openLogStream).toHaveBeenCalledTimes(1)
    expect(mocks.notifyError).toHaveBeenCalledWith(expect.stringContaining('会话已过期'))
    wrapper.unmount()
  })

  it('403 停止重连并提示权限不足', async () => {
    mocks.openLogStream.mockResolvedValue(new Response('', { status: 403 }))
    const wrapper = mount(LogsView)
    await flushPromises()
    await openStreamTab(wrapper)
    expect(mocks.notifyError).toHaveBeenCalledWith(expect.stringContaining('权限不足'))
    wrapper.unmount()
  })

  it('ReadableStream 分帧后渲染日志行', async () => {
    const encoder = new TextEncoder()
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode('data: {"time":"2026-09-11T12:00:00Z","level":"info","message":"hello-stream","attrs":""}\n\n'))
        controller.close()
      },
    })
    mocks.openLogStream.mockResolvedValue(sseResponse(stream))
    const wrapper = mount(LogsView)
    await flushPromises()
    await openStreamTab(wrapper)
    await new Promise((resolve) => setTimeout(resolve, 20))
    await flushPromises()
    expect(wrapper.text()).toContain('hello-stream')
    wrapper.unmount()
  })

  it('组件卸载 abort 当前流', async () => {
    const stream = new ReadableStream<Uint8Array>({ start() {} })
    mocks.openLogStream.mockResolvedValue(sseResponse(stream))
    const wrapper = mount(LogsView)
    await flushPromises()
    await openStreamTab(wrapper)
    const signal = mocks.openLogStream.mock.calls[mocks.openLogStream.mock.calls.length - 1][0] as AbortSignal
    expect(signal).toBeInstanceOf(AbortSignal)
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
  })
})
