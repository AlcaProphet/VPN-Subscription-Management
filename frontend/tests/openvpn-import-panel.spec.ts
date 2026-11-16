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
    attachTo: document.body,
    props: { open, sensitivePaths: ['password', 'key', 'tls-auth'] },
  })
}

// Ant Design 会在两个中文字符之间插入空格，按钮文案比较前先去掉全部空白。
function buttonByLabel(wrapper: ReturnType<typeof mountPanel>, label: string) {
  return wrapper.findAll('button').find((button) => button.text().replace(/\s+/g, '') === label)
}

describe('OpenVPNImportPanel', () => {
  it('阻断诊断定位原文整行，不修改原文、草稿阻断或发出应用', async () => {
    const source = 'remote diag.example.test 1194\nproto udp\nverb 3\nscript-security 2\nup /tmp/qa-never-run'
    mockParseOpenVPN.mockRejectedValue(new ApiError(400, 'blocked', undefined, {
      error_code: 'ovpn_dangerous_directive',
      diagnostics: [{ severity: 'error', code: 'ovpn_dangerous_directive', line: 4, message: '危险指令' }],
    }))
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue(source)
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      const dirtyBefore = wrapper.emitted('draft-dirty-change')?.length
      const jump = wrapper.find('button[aria-label="定位原文第 4 行"]')
      expect(jump.exists()).toBe(true)
      expect(jump.attributes('type')).toBe('button')
      await jump.trigger('click')
      const textarea = wrapper.find('textarea').element
      expect(document.activeElement).toBe(textarea)
      expect([textarea.selectionStart, textarea.selectionEnd]).toEqual([47, 64])
      expect(textarea.value.slice(textarea.selectionStart, textarea.selectionEnd)).toBe('script-security 2')
      expect(textarea.id).toBe(jump.attributes('aria-controls'))
      expect(textarea.value).toBe(source)
      expect(mockParseOpenVPN).toHaveBeenCalledTimes(1)
      expect(wrapper.emitted('apply')).toBeUndefined()
      expect(wrapper.emitted('draft-dirty-change')?.length).toBe(dirtyBefore)
      expect(document.querySelector('[data-openvpn-line-measure]')).toBeNull()
    } finally { wrapper.unmount() }
  })

  it.each([
    ['LF', 'first\nsecond\nlast', 2, 6, 12],
    ['CRLF', 'first\r\nsecond\r\nlast', 2, 6, 12],
    ['首行', 'first\nlast', 1, 0, 5],
    ['Unicode', '# 中文🙂\n\tverb 3', 2, 7, 14],
    ['空行', 'a\n\nb', 2, 2, 2],
    ['末行', 'a\nlast', 2, 2, 6],
    ['末尾换行', 'a\n', 2, 2, 2],
  ] as const)('%s 的来源行号按 textarea 字符偏移定位', async (_name, source, line, start, end) => {
    mockParseOpenVPN.mockResolvedValue({ ...sampleResult, field_sources: { key: line }, diagnostics: [] })
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue(source)
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      const jump = wrapper.find(`button[aria-label="定位原文第 ${line} 行"]`)
      expect(jump.exists()).toBe(true)
      await jump.trigger('click')
      const textarea = wrapper.find('textarea').element
      expect(document.activeElement).toBe(textarea)
      expect([textarea.selectionStart, textarea.selectionEnd]).toEqual([start, end])
      expect(wrapper.text()).not.toContain('private-key-body')
    } finally { wrapper.unmount() }
  })

  it('缺失或无效行号保留提示，不产生错误的定位入口', async () => {
    mockParseOpenVPN.mockResolvedValue({ ...sampleResult, field_sources: {}, diagnostics:
      [undefined, 0, -1, 1.5, 99].map(line => ({ severity: 'warn', code: 'notice', line, message: '诊断仍可读' })),
    })
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue('first\nsecond')
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('诊断仍可读')
      expect(wrapper.findAll('button[aria-controls]')).toHaveLength(0)
    } finally { wrapper.unmount() }
  })

  it('多个面板的来源定位只聚焦当前实例', async () => {
    mockParseOpenVPN.mockResolvedValue({ ...sampleResult, field_sources: { proto: 2 }, diagnostics: [] })
    // 同一个 Vue 应用内的兄弟实例；useId 的唯一性边界是应用，不是独立 mount 的多个应用。
    const root = mount({ components: { OpenVPNImportPanel }, template:
      '<div><OpenVPNImportPanel :open="true" :sensitive-paths="[]" /><OpenVPNImportPanel :open="true" :sensitive-paths="[]" /></div>',
    }, { attachTo: document.body })
    const [first, second] = root.findAllComponents(OpenVPNImportPanel)
    try {
      for (const wrapper of [first, second]) {
        await wrapper.find('textarea').setValue('first\nproto udp')
        await buttonByLabel(wrapper, '解析')!.trigger('click')
        await flushPromises()
      }
      const jump = second.find('button[aria-label="定位原文第 2 行"]')
      expect(jump.exists()).toBe(true)
      await jump.trigger('click')
      expect(document.activeElement).toBe(second.find('textarea').element)
      expect(first.find('textarea').element.id).not.toBe(second.find('textarea').element.id)
    } finally { root.unmount() }
  })

  it('接近 256 KiB 的长原文末行仍能精确选中，重新解析后恢复定位与应用', async () => {
    const source = ('# ' + 'x'.repeat(1020) + '\n').repeat(250) + 'proto udp'
    mockParseOpenVPN.mockResolvedValue({ ...sampleResult, field_sources: { proto: 251 }, diagnostics: [] })
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue(source)
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      const jump = wrapper.find('button[aria-label="定位原文第 251 行"]')
      expect(jump.exists()).toBe(true)
      await jump.trigger('click')
      const textarea = wrapper.find('textarea').element
      expect(textarea.value.slice(textarea.selectionStart, textarea.selectionEnd)).toBe('proto udp')
      await wrapper.find('textarea').setValue('proto tcp')
      expect((wrapper.vm as any).canApply).toBe(false)
      mockParseOpenVPN.mockResolvedValue({ ...sampleResult, field_sources: { proto: 1 }, diagnostics: [] })
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      expect((wrapper.vm as any).canApply).toBe(true)
      expect(wrapper.text()).not.toContain('原文已更改')
      expect(wrapper.find('button[aria-label="定位原文第 1 行"]').exists()).toBe(true)
    } finally { wrapper.unmount() }
  })

  it.each(['修改', '清空', '修改后撤回'] as const)('%s 原文后旧结果过期且保留未处理结果的阻断', async action => {
    mockParseOpenVPN.mockResolvedValue(sampleResult)
    const wrapper = mountPanel()
    try {
      const source = 'first\nproto udp\nverb 3'
      await wrapper.find('textarea').setValue(source)
      await buttonByLabel(wrapper, '解析')!.trigger('click')
      await flushPromises()
      await wrapper.find('textarea').setValue(action === '清空' ? '' : '# changed\n' + source)
      if (action === '修改后撤回') await wrapper.find('textarea').setValue(source)
      expect((wrapper.vm as any).canApply).toBe(false)
      expect(wrapper.text()).toContain('原文已更改')
      expect(wrapper.text()).not.toContain('已解析结构（脱敏）')
      expect(wrapper.findAll('button[aria-controls]')).toHaveLength(0)
      // 处理函数同样守住过期边界，不能只依赖按钮 disabled。
      ;(wrapper.vm as any).apply()
      expect(wrapper.emitted('apply')).toBeUndefined()
      const dirty = wrapper.emitted('draft-dirty-change')!
      expect(dirty[dirty.length - 1][0]).toEqual({ path: 'openvpn-import', dirty: true })
      await buttonByLabel(wrapper, '取消')!.trigger('click')
      expect(wrapper.text()).not.toContain('原文已更改')
    } finally { wrapper.unmount() }
  })

  it.each(['成功', '阻断'] as const)('编辑期间迟到的%s响应不能恢复旧结果或诊断、覆盖新 loading', async outcome => {
    const requests: { resolve: (result: typeof sampleResult) => void; reject: (error: Error) => void }[] = []
    mockParseOpenVPN.mockImplementation(() => new Promise((resolve, reject) => requests.push({ resolve, reject })))
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue('old')
      const old = (wrapper.vm as any).run()
      await wrapper.find('textarea').setValue('new')
      expect((wrapper.vm as any).parsing).toBe(false)
      const current = (wrapper.vm as any).run()
      if (outcome === '成功') requests[0].resolve(sampleResult)
      else requests[0].reject(new ApiError(400, 'old-error', undefined, { diagnostics: [{ code: 'old-diagnostic' }] }))
      await old
      expect((wrapper.vm as any).parsing).toBe(true)
      expect((wrapper.vm as any).canApply).toBe(false)
      expect((wrapper.vm as any).diagnostics).toEqual([])
      expect((wrapper.vm as any).error).toBe('')
      requests[1].resolve({ ...sampleResult, host: 'new.example.test' })
      await current
      expect((wrapper.vm as any).canApply).toBe(true)
      expect((wrapper.vm as any).result.host).toBe('new.example.test')
    } finally { wrapper.unmount() }
  })

  it.each(['成功', '阻断'] as const)('编辑后不再发请求时，迟到的%s响应仍须失效', async outcome => {
    let finish!: (result: typeof sampleResult) => void
    let fail!: (error: Error) => void
    mockParseOpenVPN.mockImplementation(() => new Promise((resolve, reject) => { finish = resolve; fail = reject }))
    const wrapper = mountPanel()
    try {
      await wrapper.find('textarea').setValue('old')
      const request = (wrapper.vm as any).run()
      await wrapper.find('textarea').setValue('new')
      if (outcome === '成功') finish(sampleResult)
      else fail(new ApiError(400, 'old-error', undefined, { diagnostics: [{ code: 'old-diagnostic' }] }))
      await request
      expect(mockParseOpenVPN).toHaveBeenCalledTimes(1)
      expect((wrapper.vm as any).canApply).toBe(false)
      expect((wrapper.vm as any).diagnostics).toEqual([])
      expect((wrapper.vm as any).error).toBe('')
      expect((wrapper.vm as any).parsing).toBe(false)
      expect(wrapper.find('textarea').element.value).toBe('new')
    } finally { wrapper.unmount() }
  })

  it.each(['reset', 'close', 'unmount', 'apply'] as const)('%s 后忽略迟到错误、诊断及结果', async (action) => {
    let reject!: (error: Error) => void
    mockParseOpenVPN.mockImplementation(() => new Promise((_resolve, fail) => { reject = fail }))
    const wrapper = mountPanel()
    await wrapper.find('textarea').setValue('client\n')
    const vm = wrapper.vm as any
    const request = vm.run()
    if (action === 'reset') vm.reset()
    if (action === 'close') await wrapper.setProps({ open: false })
    if (action === 'apply') await wrapper.setProps({ applyVersion: 1 })
    if (action === 'unmount') wrapper.unmount()
    reject(new ApiError(400, 'old-error', undefined, { diagnostics: [{ code: 'old-diagnostic' }] }))
    await request
    expect(vm.error).toBe('')
    expect(vm.diagnostics).toEqual([])
    expect(vm.parsing).toBe(false)
    expect(vm.canApply).toBe(false)
    if (action !== 'unmount') wrapper.unmount()
  })

  it('旧请求结束不能关闭新请求的 loading 或覆盖新结果', async () => {
    const resolve: ((value: typeof sampleResult) => void)[] = []
    mockParseOpenVPN.mockImplementation(() => new Promise(done => resolve.push(done)))
    const wrapper = mountPanel()
    const vm = wrapper.vm as any
    await wrapper.find('textarea').setValue('old')
    const old = vm.run()
    vm.reset()
    await wrapper.find('textarea').setValue('new')
    const current = vm.run()
    resolve[0](sampleResult)
    await old
    expect(vm.parsing).toBe(true)
    expect(vm.canApply).toBe(false)
    resolve[1]({ ...sampleResult, host: 'new.example.test' })
    await current
    expect(vm.parsing).toBe(false)
    expect(vm.result.host).toBe('new.example.test')
    wrapper.unmount()
  })
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

  it('应用请求只发出解析产出，收到页面确认回执后才清空原文与草稿阻断', async () => {
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
    // 页面尚未确认时保留原文与解析结果；取消预检仍可继续编辑。
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('client\n')
    expect(wrapper.text()).toContain('已解析结构')
    await wrapper.setProps({ applyVersion: 1 })
    await flushPromises()
    // 收到应用回执后原文与结果才被丢弃，阻断解除。
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
