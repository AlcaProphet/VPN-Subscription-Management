// logs-view.spec.ts：实时日志页 fetch/ReadableStream 协议边界（R28-07I Step 17）。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const mocks = vi.hoisted(() => ({
  openLogStream: vi.fn(),
  notifyError: vi.fn(),
  notifyWarning: vi.fn(),
  queryMailLogs: vi.fn(),
  clearMailLogs: vi.fn(),
}))

vi.mock('@/api/log', () => ({
  queryAccessLogs: vi.fn().mockResolvedValue({ list: [], total: 0 }),
  clearAccessLogs: vi.fn(),
  openLogStream: mocks.openLogStream,
  queryMailLogs: mocks.queryMailLogs,
  clearMailLogs: mocks.clearMailLogs,
}))

vi.mock('@/components/Notify', () => ({
  Notify: { error: mocks.notifyError, warning: mocks.notifyWarning, success: vi.fn() },
}))

import LogsView from '@/views/admin/LogsView.vue'

afterEach(() => {
  vi.useRealTimers()
})

function sseResponse(body: ReadableStream<Uint8Array>, status = 200): Response {
  return new Response(body, { status, headers: { 'Content-Type': 'text/event-stream' } })
}

async function openStreamTab(wrapper: ReturnType<typeof mount>) {
  const tab = wrapper.findAll('.ant-tabs-tab').find((node) => node.text().includes('实时日志流'))
  expect(tab).toBeTruthy()
  await tab!.trigger('click')
  await flushPromises()
}

async function openMailTab(wrapper: ReturnType<typeof mount>) {
  const tab = wrapper.findAll('.ant-tabs-tab').find((node) => node.text().includes('邮件发送日志'))
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
    mocks.queryMailLogs.mockReset()
    mocks.queryMailLogs.mockResolvedValue({ list: [], total: 0 })
    mocks.clearMailLogs.mockReset()
    vi.useRealTimers()
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

describe('LogsView 邮件发送日志', () => {
  it('页签展示掩码记录与 SMTP 已接受状态', async () => {
    mocks.queryMailLogs.mockResolvedValue({
      list: [{
        id: 1,
        created_at: '2026-09-17T00:00:00Z',
        started_at: '2026-09-17T00:00:00Z',
        finished_at: '2026-09-17T00:00:01Z',
        kind: 'password_reset',
        source: 'public_forgot',
        user_id: 1,
        recipient_masked: 'k***@example.com',
        status: 'accepted',
        failure_stage: null,
        queue_duration_ms: 1,
        send_duration_ms: 2,
      }],
      total: 1,
    })
    const wrapper = mount(LogsView)
    await flushPromises()
    await openMailTab(wrapper)
    expect(wrapper.text()).toContain('密码重置')
    expect(wrapper.text()).toContain('k***@example.com')
    expect(wrapper.text()).toContain('SMTP 已接受')
    expect(wrapper.text()).toContain('SMTP 已接受仅表示发件服务器接受邮件')
    wrapper.unmount()
  })

  it('5 秒轮询单飞且切出页签后停止', async () => {
    let tick: (() => void) | null = null
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval').mockImplementation(((fn: () => void, ms?: number) => {
      if (ms === 5000) tick = fn
      return 1 as unknown as ReturnType<typeof setInterval>
    }) as any)
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval').mockImplementation(() => {})

    let resolvePending!: (value: any) => void
    const pending = new Promise<any>((resolve) => { resolvePending = resolve })
    mocks.queryMailLogs.mockReturnValue(pending)
    const wrapper = mount(LogsView)
    await flushPromises()
    await openMailTab(wrapper)
    expect(tick).toBeTypeOf('function')
    const before = mocks.queryMailLogs.mock.calls.length
    expect(before).toBeGreaterThanOrEqual(1)

    tick!() // 前一次仍在请求中：本轮必须跳过
    await flushPromises()
    expect(mocks.queryMailLogs.mock.calls.length).toBe(before)

    resolvePending({ list: [], total: 0 })
    await flushPromises()
    tick!()
    await flushPromises()
    expect(mocks.queryMailLogs.mock.calls.length).toBeGreaterThan(before)

    const accessTab = wrapper.findAll('.ant-tabs-tab').find((node) => node.text().includes('访问日志'))
    await accessTab!.trigger('click')
    await flushPromises()
    expect(clearIntervalSpy).toHaveBeenCalled()
    wrapper.unmount()

    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })
})
