// sse.ts：最小 SSE 帧解析器（R28-07I）。fetch + ReadableStream 分块读取后，
// 按空行分帧；data 多行以 \n 连接；注释帧和 CRLF 均按规范处理；跨 chunk 半帧缓冲。
export interface SSEMessage {
  event: string
  data: string
}

export function createSSEParser(onMessage: (message: SSEMessage) => void) {
  let buffer = ''

  function dispatch(rawFrame: string) {
    const lines = rawFrame.split('\n')
    let event = 'message'
    const dataLines: string[] = []
    let sawData = false
    for (const line of lines) {
      if (line === '' || line.startsWith(':')) continue
      const colon = line.indexOf(':')
      const field = colon === -1 ? line : line.slice(0, colon)
      let value = colon === -1 ? '' : line.slice(colon + 1)
      if (value.startsWith(' ')) value = value.slice(1)
      if (field === 'event') event = value
      else if (field === 'data') {
        dataLines.push(value)
        sawData = true
      }
    }
    if (sawData) onMessage({ event, data: dataLines.join('\n') })
  }

  function processBuffer(final: boolean) {
    // 逐帧按空行切分；CRLF 归一化为 LF。
    let normalized = buffer.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
    let index: number
    while ((index = normalized.indexOf('\n\n')) !== -1) {
      const frame = normalized.slice(0, index)
      normalized = normalized.slice(index + 2)
      dispatch(frame)
    }
    if (final && normalized.trim() !== '') {
      dispatch(normalized)
      normalized = ''
    }
    buffer = normalized
  }

  return {
    feed(chunk: string) {
      buffer += chunk
      processBuffer(false)
    },
    flush() {
      processBuffer(true)
    },
  }
}
