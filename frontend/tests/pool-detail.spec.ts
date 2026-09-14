// pool-detail.spec.ts：素材池详情同步历史分页与卸载取消轮询（R12-05）
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PoolDetail from '@/views/admin/assembly/PoolDetail.vue'

vi.mock('@/api/pool', () => ({
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  submitSync: vi.fn(),
  getSyncStatus: vi.fn(),
  listSyncTasks: vi.fn(),
  clearSyncTasks: vi.fn(),
  listSourceStatuses: vi.fn(),
  activatePending: vi.fn(),
  discardPending: vi.fn(),
}))

vi.mock('@/api/rulespec', () => ({
  listCapabilityMeta: vi.fn().mockResolvedValue({ legacy: [], capabilities: [] }),
}))

vi.mock('@/api/request', () => {
  class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  }
  return { pollTask: vi.fn(), ApiError }
})

vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), detail: vi.fn() },
}))

import {
  activatePending, createEntry, discardPending, listEntries, listSourceStatuses, listSyncTasks,
  clearSyncTasks, updateEntry,
} from '@/api/pool'
import { listCapabilityMeta } from '@/api/rulespec'
import { pollTask } from '@/api/request'
import { Notify } from '@/components/Notify'

const mockListEntries = listEntries as unknown as ReturnType<typeof vi.fn>
const mockListTasks = listSyncTasks as unknown as ReturnType<typeof vi.fn>
const mockPollTask = pollTask as unknown as ReturnType<typeof vi.fn>
const mockCreateEntry = createEntry as unknown as ReturnType<typeof vi.fn>
const mockUpdateEntry = updateEntry as unknown as ReturnType<typeof vi.fn>
const mockListCapabilityMeta = listCapabilityMeta as unknown as ReturnType<typeof vi.fn>
const mockClearSyncTasks = clearSyncTasks as unknown as ReturnType<typeof vi.fn>
const mockListSourceStatuses = listSourceStatuses as unknown as ReturnType<typeof vi.fn>
const mockActivatePending = activatePending as unknown as ReturnType<typeof vi.fn>
const mockDiscardPending = discardPending as unknown as ReturnType<typeof vi.fn>
const mockNotifyError = Notify.error as unknown as ReturnType<typeof vi.fn>

const pool = {
  id: 1, name: '苹果域名', urls: ['https://example.com/rules.txt'], entry_count: 2,
  last_synced_at: '2026-08-19T10:00:00Z', sync_status: 'succeeded', sync_error: '',
  auto_sync: true, sync_time: '04:00',
}
const entry = {
  id: 1, pool_id: 1, rule_type: 'DOMAIN-SUFFIX', match_value: 'example.com', source: 'manual' as const, sort_order: 1,
}
const urlEntry = {
  id: 2, pool_id: 1, rule_type: 'DOMAIN-SUFFIX', match_value: 'url.example.com', source: 'url' as const, sort_order: 100000,
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((res) => { resolve = res })
  return { promise, resolve }
}

function makeStats(overrides: Record<string, unknown> = {}) {
  return {
    schema_version: 1,
    source_mode: 'auto',
    detection: { evidence_codes: [], recognition_required_percent: 100 },
    rule_counts: [],
    unclassified_rejected: 0,
    comparison: null,
    decision: null,
    ...overrides,
  }
}

function makeSnapshot(overrides: Record<string, unknown> = {}) {
  return {
    id: 1, source_id: 1, format: 'typed-rule-text', profile: 'shadowrocket', status: 'active',
    input: 1, recognized: 1, accepted: 1, excluded: 0, rejected: 0, duplicates: 0,
    diagnostics: [] as Array<Record<string, unknown>>, stats: makeStats(), error: '',
    activated_at: null, created_at: '2026-09-10T08:00:00Z',
    ...overrides,
  }
}

function makeStatus(overrides: Record<string, unknown> = {}) {
  return {
    source_id: 1, display_url: 'https://example.com/rules.txt?token=***', source_mode: 'auto',
    never_synced: false, latest_attempt: null, active: null, pending: null, latest_failed: null,
    ...overrides,
  }
}

function mountPoolDetail(options: { pool?: typeof pool; statuses?: Array<Record<string, unknown>>; attach?: boolean } = {}) {
  if (options.statuses) mockListSourceStatuses.mockResolvedValue(options.statuses)
  return mount(PoolDetail, {
    props: { pool: options.pool ?? pool },
    ...(options.attach ? { attachTo: document.body } : {}),
  })
}

describe('PoolDetail', () => {
  beforeEach(() => {
    mockListEntries.mockReset()
    mockListTasks.mockReset()
    mockPollTask.mockReset()
    mockCreateEntry.mockReset()
    mockUpdateEntry.mockReset()
    mockListSourceStatuses.mockReset()
    mockActivatePending.mockReset()
    mockDiscardPending.mockReset()
    mockNotifyError.mockReset()
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    mockListSourceStatuses.mockResolvedValue([])
    mockActivatePending.mockResolvedValue(undefined)
    mockDiscardPending.mockResolvedValue(undefined)
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('同步历史支持分页加载', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 2 })
    mockListTasks
      .mockResolvedValueOnce({
        list: [{
          task_id: 1, pool_id: 1, status: 'succeeded', per_url: [], error: '',
          started_at: '2026-08-19T10:00:00Z', finished_at: '2026-08-19T10:01:00Z',
        }],
        total: 2,
      })
      .mockResolvedValueOnce({
        list: [{
          task_id: 2, pool_id: 1, status: 'failed', per_url: [], error: '拉取失败',
          started_at: '2026-08-19T11:00:00Z', finished_at: '2026-08-19T11:01:00Z',
        }],
        total: 2,
      })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(wrapper.text()).toContain('成功')
    const vm = wrapper.vm as unknown as { taskPage: number }
    vm.taskPage = 2
    await flushPromises()
    expect(mockListTasks).toHaveBeenCalledWith(1, 2, 20)
    expect(wrapper.text()).toContain('失败')
    expect(wrapper.text()).toContain('原因')
  })

  it('URL 同步条目默认不查询，展开后按来源单独加载', async () => {
    mockListEntries
      .mockResolvedValueOnce({ list: [entry], total: 1 })
      .mockResolvedValueOnce({ list: [urlEntry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(mockListEntries).toHaveBeenCalledTimes(1)
    expect(mockListEntries).toHaveBeenLastCalledWith(1, 1, 20, 'manual')
    expect(wrapper.text()).not.toContain('url.example.com')

    await wrapper.get('[data-testid="toggle-url-entries"]').trigger('click')
    await flushPromises()
    expect(mockListEntries).toHaveBeenLastCalledWith(1, 1, 20, 'url')
    expect(wrapper.text()).toContain('url.example.com')
  })

  it('组件卸载时取消正在进行的同步轮询', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    const d = deferred<{ status: string }>()
    const cancel = vi.fn()
    const run = vi.fn(() => d.promise)
    mockPollTask.mockReturnValue({ run, cancel })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { doSync: () => Promise<void> }
    const first = vm.doSync()
    await Promise.resolve()
    wrapper.unmount()
    expect(cancel).toHaveBeenCalled()
    d.resolve({ status: 'succeeded' })
    await first
  })

  it('手动条目新增后向上通知素材池内容变化', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    mockCreateEntry.mockResolvedValue(entry)
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      entryForm: { rule_type: string; match_value: string }
      saveEntry: () => Promise<void>
    }
    vm.entryForm.match_value = 'new.example.com'
    await vm.saveEntry()
    expect(wrapper.emitted('changed')).toEqual([[pool.id]])
  })

  it('素材池规则类型下拉从后端 snake_case 元数据生成', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({ list: [], total: 0 })
    mockListCapabilityMeta.mockResolvedValue({
      legacy: [
        { rule_type: 'DOMAIN', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: false },
        { rule_type: 'DOMAIN-SUFFIX', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: false },
        { rule_type: 'IP-CIDR', scope: 'common', material_pool: true, advanced: true, supports_no_resolve: true },
        { rule_type: 'GEOSITE', scope: 'clash_only', material_pool: false, advanced: true, supports_no_resolve: false },
      ],
      capabilities: [],
    })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { manualRuleTypes: string[] }
    expect(vm.manualRuleTypes).toContain('DOMAIN')
    expect(vm.manualRuleTypes).toContain('DOMAIN-SUFFIX')
    expect(vm.manualRuleTypes).toContain('IP-CIDR')
    expect(vm.manualRuleTypes).not.toContain('GEOSITE')
  })

  it('清理已完成历史调用当前池接口并刷新历史', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks
      .mockResolvedValueOnce({ list: [{ task_id: 1, pool_id: 1, status: 'succeeded', per_url: [], error: '', started_at: '', finished_at: '' }], total: 1 })
      .mockResolvedValueOnce({ list: [], total: 0 })
    mockClearSyncTasks.mockResolvedValue({ cleared: 1 })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    const vm = wrapper.vm as unknown as { clearOpen: boolean; confirmClearTasks: () => Promise<void> }
    vm.clearOpen = true
    await vm.confirmClearTasks()
    expect(mockClearSyncTasks).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('暂无同步任务')
  })

  it('同步历史展示后端实际返回的解析统计而非空计数', async () => {
    mockListEntries.mockResolvedValue({ list: [entry], total: 1 })
    mockListTasks.mockResolvedValue({
      list: [{
        task_id: 1, pool_id: 1, status: 'succeeded',
        per_url: [{ url: 'https://example.com/rules.txt', ok: true, accepted: 112, excluded: 0, rejected: 2, duplicates: 3, pending: false, error: '' }],
        error: '', started_at: '', finished_at: '',
      }],
      total: 1,
    })
    const wrapper = mount(PoolDetail, { props: { pool } })
    await flushPromises()
    expect(wrapper.text()).toContain('接受 112')
    expect(wrapper.text()).toContain('拒绝 2')
    expect(wrapper.text()).toContain('重复 3')
    expect(wrapper.text()).not.toContain('新增 undefined')
  })

  it('初始化时加载来源状态并展示 display_url', async () => {
    const wrapper = mountPoolDetail({ statuses: [makeStatus()] })
    await flushPromises()
    expect(mockListSourceStatuses).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('https://example.com/rules.txt?token=***')
    expect(wrapper.find('[data-testid="source-status-card"]').exists()).toBe(true)
  })

  it('同步完成后重新加载来源状态', async () => {
    mockPollTask.mockReturnValue({
      run: vi.fn().mockResolvedValue({ status: 'succeeded', per_url: [], error: '' }),
      cancel: vi.fn(),
    })
    const wrapper = mountPoolDetail()
    await flushPromises()
    const before = mockListSourceStatuses.mock.calls.length
    const vm = wrapper.vm as unknown as { doSync: () => Promise<void> }
    await vm.doSync()
    await flushPromises()
    expect(mockListSourceStatuses.mock.calls.length).toBeGreaterThan(before)
  })

  it('pending 快照展示激活和丢弃操作', async () => {
    const pending = makeSnapshot({ id: 12, status: 'pending' })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: pending, pending })] })
    await flushPromises()
    expect(wrapper.text()).toContain('待激活快照 #12')
    const buttonTexts = wrapper.findAll('button').map((b) => b.text().replace(/\s+/g, ''))
    expect(buttonTexts).toContain('激活')
    expect(buttonTexts).toContain('丢弃')
  })

  it('激活确认展示旧 active 与新 pending 的输入、接受数、格式、平台和诊断差异', async () => {
    const oldActive = makeSnapshot({
      id: 10, status: 'active', input: 3, accepted: 3, format: 'typed-rule-text', profile: 'shadowrocket',
      diagnostics: [{ line: 1, kind: 'warn', message: '旧诊断', raw: '' }],
    })
    const pending = makeSnapshot({
      id: 12, status: 'pending', input: 5, accepted: 4, format: 'mihomo-domain-yaml', profile: 'clash',
      diagnostics: [{ line: 0, kind: 'warn', message: '新诊断', raw: '' }],
    })
    const wrapper = mountPoolDetail({
      statuses: [makeStatus({ latest_attempt: pending, active: oldActive, pending })], attach: true,
    })
    await flushPromises()
    const vm = wrapper.vm as unknown as { sourceStatuses: Array<Record<string, unknown>>; openActivate: (s: any) => void }
    vm.openActivate(vm.sourceStatuses[0])
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    const text = document.body.textContent ?? ''
    expect(text).toContain('旧 active')
    expect(text).toContain('新 pending')
    expect(text).toContain('输入 3')
    expect(text).toContain('输入 5')
    expect(text).toContain('接受 3')
    expect(text).toContain('接受 4')
    expect(text).toContain('typed-rule-text')
    expect(text).toContain('mihomo-domain-yaml')
    expect(text).toContain('shadowrocket')
    expect(text).toContain('clash')
    expect(text).toContain('旧诊断')
    expect(text).toContain('新诊断')
  })

  it('激活成功后刷新来源状态和素材条目，并展示服务端 activated_at', async () => {
    const pending = makeSnapshot({ id: 12, status: 'pending' })
    const oldActive = makeSnapshot({ id: 10, status: 'active' })
    mockListSourceStatuses
      .mockResolvedValueOnce([makeStatus({ latest_attempt: pending, active: oldActive, pending })])
      .mockResolvedValueOnce([makeStatus({
        latest_attempt: makeSnapshot({ id: 12, status: 'active', activated_at: '2026-09-10T12:00:00Z' }),
        active: makeSnapshot({ id: 12, status: 'active', activated_at: '2026-09-10T12:00:00Z' }),
      })])
    const wrapper = mountPoolDetail()
    await flushPromises()
    const entryCalls = mockListEntries.mock.calls.length
    const vm = wrapper.vm as unknown as {
      sourceStatuses: Array<Record<string, unknown>>
      openActivate: (s: any) => void
      confirmActivate: () => Promise<void>
    }
    vm.openActivate(vm.sourceStatuses[0])
    await vm.confirmActivate()
    await flushPromises()
    expect(mockActivatePending).toHaveBeenCalledWith(1, 1, 12)
    expect(mockListSourceStatuses.mock.calls.length).toBe(2)
    expect(mockListEntries.mock.calls.length).toBeGreaterThan(entryCalls)
    const activated = wrapper.find('[data-testid="active-activated-at"]')
    expect(activated.attributes('data-raw')).toBe('2026-09-10T12:00:00Z')
    expect(wrapper.text()).toContain('2026-09-10')
  })

  it('丢弃成功后刷新来源状态和素材条目', async () => {
    const pending = makeSnapshot({ id: 12, status: 'pending' })
    mockListSourceStatuses
      .mockResolvedValueOnce([makeStatus({ latest_attempt: pending, pending })])
      .mockResolvedValue([makeStatus()])
    const wrapper = mountPoolDetail()
    await flushPromises()
    const entryCalls = mockListEntries.mock.calls.length
    const vm = wrapper.vm as unknown as {
      sourceStatuses: Array<Record<string, unknown>>
      openDiscard: (s: any) => void
      confirmDiscard: () => Promise<void>
    }
    vm.openDiscard(vm.sourceStatuses[0])
    await vm.confirmDiscard()
    await flushPromises()
    expect(mockDiscardPending).toHaveBeenCalledWith(1, 1, 12)
    expect(mockListSourceStatuses.mock.calls.length).toBe(2)
    expect(mockListEntries.mock.calls.length).toBeGreaterThan(entryCalls)
  })

  it('展示 v1 stats 的检测依据、rule_counts、比较差异和初始决策原因', async () => {
    const snap = makeSnapshot({
      status: 'pending',
      stats: makeStats({
        detection: { evidence_codes: ['typed_rule_marker'], recognition_required_percent: 100 },
        rule_counts: [{ family: 'domain', matcher: 'suffix', scope: 'common', accepted: 2, excluded: 1, rejected: 0, duplicates: 1 }],
        unclassified_rejected: 1,
        comparison: {
          previous_active: { snapshot_id: 9, format: 'typed-rule-text', profile: 'shadowrocket', accepted: 5 },
          format_changed: true,
          profile_changed: true,
          accepted_drop_threshold_percent: 70,
          accepted_drop_triggered: true,
        },
        decision: { initial_status: 'pending', reason_codes: ['format_changed', 'accepted_below_threshold'] },
      }),
    })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: snap, pending: snap })] })
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('typed_rule_marker')
    expect(text).toContain('domain')
    expect(text).toContain('suffix')
    expect(text).toContain('common')
    expect(text).toContain('旧 active #9')
    expect(text).toContain('typed-rule-text')
    expect(text).toContain('格式变化')
    expect(text).toContain('平台变化')
    expect(text).toContain('接受量低于阈值')
    expect(text).toContain('70')
  })

  it('version 0 显示历史统计不可用，不把空字段解释为零变化', async () => {
    const snap = makeSnapshot({
      status: 'active',
      stats: {
        schema_version: 0, source_mode: '', detection: null, rule_counts: [],
        unclassified_rejected: 0, comparison: null, decision: null,
      },
    })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: snap, active: snap })] })
    await flushPromises()
    expect(wrapper.text()).toContain('历史统计不可用')
    expect(wrapper.text()).not.toContain('格式变化')
    expect(wrapper.text()).not.toContain('首次同步')
  })

  it('最近失败但旧 active 生效时主状态为 failed 并提示继续使用旧活动快照', async () => {
    const failed = makeSnapshot({
      id: 12, status: 'failed', format: '', profile: '', input: 0, recognized: 0, accepted: 0,
      diagnostics: [{ line: 0, kind: 'error', message: 'HTTP 500', raw: '' }], error: 'HTTP 500',
    })
    const active = makeSnapshot({ id: 10, input: 3, accepted: 3 })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: failed, active, latest_failed: failed })] })
    await flushPromises()
    expect(wrapper.find('[data-testid="source-main-status"]').attributes('data-status')).toBe('failed')
    expect(wrapper.text()).toContain('同步失败，继续使用旧活动快照')
    expect(wrapper.text()).toContain('当前活动快照 #10')
    expect(wrapper.text()).toContain('HTTP 500')
  })

  it('失败后再次同步成功时主状态恢复 active，历史 failed 不永久控制主徽标', async () => {
    const failed = makeSnapshot({ id: 12, status: 'failed', error: 'HTTP 500' })
    const success = makeSnapshot({ id: 13, status: 'active', input: 2, accepted: 2 })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: success, active: success, latest_failed: failed })] })
    await flushPromises()
    expect(wrapper.find('[data-testid="source-main-status"]').attributes('data-status')).toBe('active')
    expect(wrapper.text()).not.toContain('同步失败，继续使用旧活动快照')
    expect(wrapper.text()).toContain('历史失败 #12')
  })

  it('从未同步的来源显示 never_synced 主状态', async () => {
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ never_synced: true })] })
    await flushPromises()
    expect(wrapper.find('[data-testid="source-main-status"]').attributes('data-status')).toBe('never_synced')
    expect(wrapper.text()).toContain('从未同步')
  })

  it('display_url 只用于展示且不进入条目编辑提交', async () => {
    const poolWithOriginal = {
      ...pool,
      urls: ['https://example.com/rules.txt?token=secret'],
      sources: [{
        id: 1, pool_id: 1, kind: 'url' as const, url: 'https://example.com/rules.txt?token=secret',
        source_mode: 'auto' as const, sort_order: 1,
      }],
    }
    mockUpdateEntry.mockResolvedValue(entry)
    const wrapper = mountPoolDetail({
      pool: poolWithOriginal as typeof pool,
      statuses: [makeStatus({ display_url: 'https://example.com/rules.txt?token=***' })],
    })
    await flushPromises()
    expect(wrapper.text()).toContain('https://example.com/rules.txt?token=***')
    expect(wrapper.text()).not.toContain('token=secret')
    const vm = wrapper.vm as unknown as {
      openEditEntry: (e: unknown) => void
      entryForm: { rule_type: string; match_value: string }
      saveEntry: () => Promise<void>
    }
    vm.openEditEntry(entry)
    vm.entryForm.match_value = 'changed.example.com'
    await vm.saveEntry()
    expect(mockUpdateEntry).toHaveBeenCalledWith(1, 1, { rule_type: 'DOMAIN-SUFFIX', match_value: 'changed.example.com' })
    expect(JSON.stringify(mockUpdateEntry.mock.calls[0])).not.toContain('display_url')
    expect(JSON.stringify(mockUpdateEntry.mock.calls[0])).not.toContain('token=secret')
  })

  it('来源状态加载失败时展示错误并保留重试入口', async () => {
    mockListSourceStatuses.mockRejectedValueOnce(new Error('source status failed'))
    const wrapper = mountPoolDetail()
    await flushPromises()
    expect(wrapper.text()).toContain('source status failed')
    expect(wrapper.find('[data-testid="source-status-retry"]').exists()).toBe(true)
    expect(mockNotifyError).toHaveBeenCalledWith('source status failed')
  })

  it('激活失败按现有错误处理展示且不错误刷新成功的来源状态', async () => {
    const pending = makeSnapshot({ id: 12, status: 'pending' })
    mockActivatePending.mockRejectedValueOnce(new Error('activate failed'))
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: pending, pending })] })
    await flushPromises()
    const before = mockListSourceStatuses.mock.calls.length
    const vm = wrapper.vm as unknown as {
      sourceStatuses: Array<Record<string, unknown>>
      openActivate: (s: any) => void
      confirmActivate: () => Promise<void>
    }
    vm.openActivate(vm.sourceStatuses[0])
    await vm.confirmActivate()
    await flushPromises()
    expect(mockNotifyError).toHaveBeenCalledWith('activate failed')
    expect(mockListSourceStatuses.mock.calls.length).toBe(before)
  })

  it('长诊断使用可换行/截断样式，避免卡片横向溢出', async () => {
    const longDiag = makeSnapshot({
      diagnostics: [{
        line: 1, kind: 'error', message: 'x'.repeat(500),
        raw: `https://example.com/very-long?trace=${'a'.repeat(500)}`,
      }],
    })
    const wrapper = mountPoolDetail({ statuses: [makeStatus({ latest_attempt: longDiag })] })
    await flushPromises()
    const diag = wrapper.find('[data-testid="source-diagnostic"]')
    expect(diag.exists()).toBe(true)
    expect(diag.classes()).toContain('break-all')
    expect(diag.classes()).toContain('whitespace-pre-wrap')
  })


})
