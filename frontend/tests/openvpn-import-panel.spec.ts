// openvpn-import-panel.spec.ts：Build32 Step 19 `.ovpn` 粘贴解析面板单测。
// 覆盖显式解析／应用／取消、诊断渲染、敏感字段脱敏、原文不进持久化，以及草稿阻断信号。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('@/api/node', () => ({ parseOpenVPN: vi.fn() }))

vi.mock('@/api/request', () => {
  // 与真实 ApiError 一致的四参签名：details 是第 4 个参数（第 3 个是 429 的 Retry-After）。
  class ApiError extends Error {
    status: number
    retryAfter?: string
    details?: unknown
    constructor(status: number, message: string, retryAfter?: string, details?: unknown) {
      super(message)
      this.status = status
      this.retryAfter = retryAfter
      this.details = details
    }
  }
  return { ApiError }
})

import OpenVPNImportPanel from '@/components/OpenVPNImportPanel.vue'
import { parseOpenVPN } from '@/api/node'
import { ApiError } from '@/api/request'

const mockParseOpenVPN = parseOpenVPN as unknown as ReturnType<typeof vi.fn>

const sampleResult = {
  host: 'vpn.example.com',
  port: 1194,
  protocol_json: { ca: 'ca-pem-body', key: 'private-key-body', proto: 'udp' },
  selectors: { auth_mode: 'cert', tls_key_mode: 'none' },
  field_sources: { remote: 3, ca: 8, key: 12, proto: 4 },
  diagnostics: [
    { severity: 'warn', code: 'ovpn_unsupported_directive', line: 5, message: '指令 nobind 已忽略' },
  ],
}

function mountPanel(open = true) {
  return mount(OpenVPNImportPanel, {
    props: { open, sensitivePaths: ['password', 'key', 'tls-auth'] },
  })
}

// Ant Design 会在两个中文字符之间插入空格，按钮文案比较前先去掉全部空白。
function buttonByLabel(wrapper: ReturnType<typeof mountPanel>, label: string) {
  return wrapper.findAll('button').find((button) => button.text().replace(/\s+/g, '') === label)
}

describe('OpenVPNImportPanel', () => {
  beforeEach(() => {
    mockParseOpenVPN.mockReset()
  })

  it('解析成功展示脱敏结构与来源行号，敏感字段不显示取值', async () => {
    mockParseOpenVPN.mockResolvedValue(sampleResult)
    const wrapper = mountPanel()
    await wrapper.find('textarea').setValue('client\nremote vpn.example.com 1194\n')
    await buttonByLabel(wrapper, '解析')!.trigger('click')
    await flushPromises()

    expect(mockParseOpenVPN).toHaveBeenCalledWith('client\nremote vpn.example.com 1194\n')
    const text = wrapper.text()
    expect(text).toContain('ca')
    expect(text).toContain('行 8')
    expect(text).toContain('ovpn_unsupported_directive')
    expect(text).toContain('行 5')
    // 敏感字段只提示已解析，不展示取值。
    expect(text).not.toContain('private-key-body')
    expect(text).toContain('敏感值不展示')
    wrapper.unmount()
  })

  it('阻断响应按 error_code 与诊断展示，不渲染原文', async () => {
    mockParseOpenVPN.mockRejectedValue(new ApiError(400, '`.ovpn` 解析被阻断', undefined, {
      error_code: 'ovpn_dangerous_directive',
      diagnostics: [{ severity: 'error', code: 'ovpn_dangerous_directive', line: 7, message: '指令 up 不会被执行' }],
    }))
    const wrapper = mountPanel()
    await wrapper.find('textarea').setValue('up /etc/openvpn/up.sh\n')
    await buttonByLabel(wrapper, '解析')!.trigger('click')
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('ovpn_dangerous_directive')
    expect(text).toContain('行 7')
    expect(text).not.toContain('up.sh')
    wrapper.unmount()
  })

  it('应用后只发出解析产出的字段并清空原文与草稿阻断', async () => {
    mockParseOpenVPN.mockResolvedValue(sampleResult)
    const wrapper = mountPanel()
    await wrapper.find('textarea').setValue('client\n')
    await buttonByLabel(wrapper, '解析')!.trigger('click')
    await flushPromises()

    await buttonByLabel(wrapper, '应用解析结果')!.trigger('click')
    await flushPromises()

    const applied = wrapper.emitted('apply')
    expect(applied).toBeTruthy()
    expect(applied![0][0]).toEqual({
      host: 'vpn.example.com',
      port: 1194,
      protocol_json: sampleResult.protocol_json,
      selectors: sampleResult.selectors,
    })
    // 应用后原文与结果都被丢弃，阻断解除。
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
    const dirtyEvents = wrapper.emitted('draft-dirty-change') ?? []
    expect(dirtyEvents[dirtyEvents.length - 1][0]).toEqual({ path: 'openvpn-import', dirty: false })
    wrapper.unmount()
  })

  it('取消清空原文并解除草稿阻断', async () => {
    const wrapper = mountPanel()
    await wrapper.find('textarea').setValue('client\nremote vpn.example.com 1194\n')
    await flushPromises()
    const dirtyEvents = wrapper.emitted('draft-dirty-change') ?? []
    expect(dirtyEvents.some((event) => (event[0] as { dirty: boolean }).dirty)).toBe(true)

    await buttonByLabel(wrapper, '取消')!.trigger('click')
    await flushPromises()
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
    const after = wrapper.emitted('draft-dirty-change') ?? []
    expect(after[after.length - 1][0]).toEqual({ path: 'openvpn-import', dirty: false })
    wrapper.unmount()
  })

  it('关闭面板丢弃原文与解析结果', async () => {
    mockParseOpenVPN.mockResolvedValue(sampleResult)
    const wrapper = mountPanel(true)
    await wrapper.find('textarea').setValue('client\n')
    await buttonByLabel(wrapper, '解析')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('已解析结构（脱敏）')

    await wrapper.setProps({ open: false })
    await flushPromises()
    expect(wrapper.text()).not.toContain('已解析结构（脱敏）')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
    wrapper.unmount()
  })
})
