// sse-parser.spec.ts：fetch/ReadableStream SSE 帧解析边界（R28-07I Step 17）。
import { describe, expect, it, vi } from 'vitest'
import { createSSEParser, type SSEMessage } from '@/utils/sse'
import { openLogStream } from '@/api/log'

describe('SSE 帧解析', () => {
  it('标准 data 帧、多行 data、注释与空行', () => {
    const out: SSEMessage[] = []
    const parser = createSSEParser((m) => out.push(m))
    parser.feed('data: first\n\n: heartbeat\ndata: line1\ndata: line2\n\n')
    expect(out).toEqual([
      { event: 'message', data: 'first' },
      { event: 'message', data: 'line1\nline2' },
    ])
  })

  it('跨 chunk 半帧缓冲与 CRLF', () => {
    const out: SSEMessage[] = []
    const parser = createSSEParser((m) => out.push(m))
    parser.feed('event: log\r\ndata: {"mes')
    parser.feed('sage":"hello"}\r\n\r\ndata: next\n\n')
    expect(out).toEqual([
      { event: 'log', data: '{"message":"hello"}' },
      { event: 'message', data: 'next' },
    ])
  })

  it('flush 处理末尾无空行的完整帧，且无 data 的事件不派发', () => {
    const out: SSEMessage[] = []
    const parser = createSSEParser((m) => out.push(m))
    parser.feed('event: ping\ndata: tail')
    parser.flush()
    parser.feed('event: ignored')
    parser.flush()
    expect(out).toEqual([{ event: 'ping', data: 'tail' }])
  })

  it('无冒号 data 与带空格 data 均按规范取值', () => {
    const out: SSEMessage[] = []
    const parser = createSSEParser((m) => out.push(m))
    parser.feed('data:value\n\ndata:  value2\n\n')
    expect(out).toEqual([
      { event: 'message', data: 'value' },
      { event: 'message', data: ' value2' },
    ])
  })
})

describe('openLogStream 请求协议', () => {
  it('使用 Bearer 头且 URL 不携带查询 Token', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200 })
    vi.stubGlobal('fetch', fetchMock)
    localStorage.setItem('token', 'session-token')
    const signal = new AbortController().signal
    await openLogStream(signal)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/admin/logs/stream')
    expect(url).not.toContain('token=')
    expect(init.headers.Authorization).toBe('Bearer session-token')
    expect(init.signal).toBe(signal)
    vi.unstubAllGlobals()
    localStorage.clear()
  })
})

