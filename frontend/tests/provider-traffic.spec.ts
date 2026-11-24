// provider-traffic.spec.ts：服务器剩余流量首页卡片与管理设置组件定向回归（Build41 Step 5）。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import dayjs from 'dayjs'
import { ApiError } from '@/api/request'

vi.mock('@/api/providerTraffic', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/providerTraffic')>()
  return {
    ...actual,
    getHomeProviderTraffic: vi.fn(),
    getProviderTrafficSettings: vi.fn(),
    saveProviderTrafficEnabled: vi.fn(),
    saveProviderTrafficBinding: vi.fn(),
    deleteProviderTrafficBinding: vi.fn(),
    refreshProviderTraffic: vi.fn(),
    saveProviderTrafficHost: vi.fn(),
    saveProviderTrafficVPSAccount: vi.fn(),
  }
})

import ProviderTrafficCard from '@/components/ProviderTrafficCard.vue'
import ProviderTrafficSettings from '@/components/settings/ProviderTrafficSettings.vue'
import ConfirmModal from '@/components/ConfirmModal.vue'
import { useSystemStore } from '@/stores/system'
import {
  getHomeProviderTraffic,
  getProviderTrafficSettings,
  saveProviderTrafficEnabled,
  saveProviderTrafficBinding,
  deleteProviderTrafficBinding,
  refreshProviderTraffic,
  saveProviderTrafficHost,
  saveProviderTrafficVPSAccount,
  type Provider,
  type ProviderTrafficBinding,
  type ProviderTrafficHome,
  type ProviderTrafficResult,
  type ProviderTrafficSettings as ProviderTrafficSettingsData,
} from '@/api/providerTraffic'

const mockHome = getHomeProviderTraffic as unknown as ReturnType<typeof vi.fn>
const mockSettings = getProviderTrafficSettings as unknown as ReturnType<typeof vi.fn>
const mockSaveEnabled = saveProviderTrafficEnabled as unknown as ReturnType<typeof vi.fn>
const mockSaveBinding = saveProviderTrafficBinding as unknown as ReturnType<typeof vi.fn>
const mockDeleteBinding = deleteProviderTrafficBinding as unknown as ReturnType<typeof vi.fn>
const mockRefresh = refreshProviderTraffic as unknown as ReturnType<typeof vi.fn>

enableAutoUnmount(afterEach)

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

const SAMPLED_AT = '2026-11-24T02:15:00Z'

function binding(provider: Provider, patch: Partial<ProviderTrafficBinding> = {}): ProviderTrafficBinding {
  return {
    provider,
    enabled: false,
    instance_id: null,
    revision: null,
    credential_configured: false,
    result: null,
    ...patch,
  }
}

function settingsData(patch: Partial<ProviderTrafficSettingsData> = {}): ProviderTrafficSettingsData {
  return {
    available: true,
    enabled: false,
    providers: [binding('kiwivm'), binding('vps')],
    ...patch,
  }
}

function okResult(provider: Provider, remaining: number, unit: string, sampledAt = SAMPLED_AT): ProviderTrafficResult {
  return { provider, status: 'ok', remaining, unit, sampled_at: sampledAt }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function findButton(scope: { findAll: (selector: string) => DOMWrapper<Element>[] }, text: string) {
  return scope.findAll('button').find((button) => button.text().replace(/\s/g, '') === text)
}

function saveButton(scope: { findAll: (selector: string) => DOMWrapper<Element>[] }) {
  return findButton(scope, '保存')
}

function refreshButton(scope: { findAll: (selector: string) => DOMWrapper<Element>[] }) {
  return findButton(scope, '立即采集')
}

function deleteButton(scope: { findAll: (selector: string) => DOMWrapper<Element>[] }) {
  return findButton(scope, '解除绑定')
}

async function mountSettings(data = settingsData()) {
  mockSettings.mockResolvedValueOnce(data)
  const wrapper = mount(ProviderTrafficSettings)
  await flushPromises()
  return wrapper
}

function providerRow(wrapper: ReturnType<typeof mount>, provider: Provider) {
  return wrapper.find(`[data-provider="${provider}"]`)
}

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  vi.clearAllMocks()
  mockHome.mockResolvedValue({ list: [], total: 0 })
  mockSettings.mockResolvedValue(settingsData())
  mockSaveEnabled.mockResolvedValue(settingsData())
  mockSaveBinding.mockResolvedValue(settingsData())
  mockDeleteBinding.mockResolvedValue(settingsData())
  mockRefresh.mockResolvedValue(okResult('kiwivm', 1, 'GiB'))
})

describe('ProviderTrafficCard 首页展示', () => {
  it('独立显示同平台的各台主机以及折半结果', async () => {
    mockHome.mockResolvedValueOnce({ list: [
      { ...okResult('kiwivm', 50.75, 'GiB'), host_id: 'host-1', name: 'JP01', half_remaining: true },
      { provider: 'kiwivm', host_id: 'host-2', name: 'JP02', status: 'error', message: '无法连接服务商' },
    ], total: 2 })
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(wrapper.text()).toContain('JP01')
    expect(wrapper.text()).toContain('50.75')
    expect(wrapper.text()).toContain('双向流量计算，显示 50% 余量')
    expect(wrapper.text()).toContain('JP02')
    expect(wrapper.text()).toContain('无法连接服务商')
  })

  it('默认关闭或空列表时隐藏整个卡片', async () => {
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(wrapper.find('#provider-traffic-home').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('服务器剩余流量')
    expect(mockHome).toHaveBeenCalledTimes(1)
  })

  it('waiting 只显示等待采集', async () => {
    mockHome.mockResolvedValue({ list: [{ provider: 'kiwivm', status: 'waiting' }], total: 1 })
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('服务器剩余流量')
    expect(text).toContain('搬瓦工')
    expect(text).toContain('等待采集')
    expect(text).not.toContain('采集失败')
  })

  it('成功结果保留两位、使用后端单位并本地化采集时间，合法零显示 0.00', async () => {
    mockHome.mockResolvedValue({ list: [okResult('kiwivm', 0, 'GiB')], total: 1 })
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('0.00')
    expect(text).toContain('GiB')
    expect(text).toContain(`采集于 ${dayjs(SAMPLED_AT).format('MM-DD HH:mm')}`)
  })

  it('双家展示时一家失败不遮掉另一家结果', async () => {
    mockHome.mockResolvedValue({
      list: [
        okResult('kiwivm', 180.41, 'GiB'),
        { provider: 'vps', status: 'error', message: '无法连接服务商' },
      ],
      total: 2,
    })
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('180.41')
    expect(text).toContain('GiB')
    expect(text).toContain('采集失败')
    expect(text).toContain('无法连接服务商')
  })

  it('成功转失败撤下旧余量与旧时间，之后成功恢复新数值', async () => {
    vi.useFakeTimers()
    mockHome.mockResolvedValue({ list: [okResult('kiwivm', 180.41, 'GiB')], total: 1 })
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(wrapper.text()).toContain('180.41')

    mockHome.mockResolvedValueOnce({
      list: [{ provider: 'kiwivm', status: 'error', message: '服务商限流' }],
      total: 1,
    })
    await vi.advanceTimersByTimeAsync(60000)
    await flushPromises()
    expect(wrapper.text()).toContain('采集失败')
    expect(wrapper.text()).toContain('服务商限流')
    expect(wrapper.text()).not.toContain('180.41')

    const recoveredAt = '2026-11-24T03:20:00Z'
    mockHome.mockResolvedValueOnce({ list: [okResult('kiwivm', 0, 'GiB', recoveredAt)], total: 1 })
    await vi.advanceTimersByTimeAsync(60000)
    await flushPromises()
    expect(wrapper.text()).toContain('0.00')
    expect(wrapper.text()).toContain(`采集于 ${dayjs(recoveredAt).format('MM-DD HH:mm')}`)
    expect(wrapper.text()).not.toContain('采集失败')
  })

  it('首页接口自身失败只显示服务器余量读取失败，下一次成功后恢复', async () => {
    vi.useFakeTimers()
    mockHome.mockRejectedValueOnce(new Error('network down'))
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(wrapper.text()).toContain('服务器剩余流量')
    expect(wrapper.text()).toContain('服务器余量读取失败')

    mockHome.mockResolvedValueOnce({ list: [okResult('vps', 5.5, 'GB')], total: 1 })
    await vi.advanceTimersByTimeAsync(60000)
    await flushPromises()
    expect(wrapper.text()).not.toContain('服务器余量读取失败')
    expect(wrapper.text()).toContain('5.50')
  })

  it('每分钟轮询，离页清理定时器', async () => {
    vi.useFakeTimers()
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(mockHome).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(60000)
    expect(mockHome).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(60000)
    expect(mockHome).toHaveBeenCalledTimes(2)
  })

  it('迟到的旧响应不覆盖较新的结果', async () => {
    vi.useFakeTimers()
    const first = deferred<ProviderTrafficHome>()
    const second = deferred<ProviderTrafficHome>()
    mockHome.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(mockHome).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(60000)
    expect(mockHome).toHaveBeenCalledTimes(2)

    second.resolve({ list: [okResult('vps', 5, 'GB')], total: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('5.00')

    first.resolve({ list: [okResult('kiwivm', 99, 'GiB')], total: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('5.00')
    expect(wrapper.text()).not.toContain('99.00')
  })

  it('卸载后的迟到响应不影响新挂载页面', async () => {
    const oldPending = deferred<ProviderTrafficHome>()
    mockHome.mockReturnValueOnce(oldPending.promise)
    const oldWrapper = mount(ProviderTrafficCard)
    await flushPromises()
    oldWrapper.unmount()

    mockHome.mockResolvedValueOnce({ list: [okResult('vps', 7, 'GB')], total: 1 })
    const newWrapper = mount(ProviderTrafficCard)
    await flushPromises()
    expect(newWrapper.text()).toContain('7.00')

    oldPending.resolve({ list: [okResult('kiwivm', 11, 'GiB')], total: 1 })
    await flushPromises()
    expect(newWrapper.text()).toContain('7.00')
    expect(newWrapper.text()).not.toContain('11.00')
  })
})

describe('ProviderTrafficSettings 管理设置', () => {
  it('按主机添加搬瓦工，并为 V.PS 账号下的主机单独保存折半开关', async () => {
    const base = settingsData({ hosts: [], vps_account: { configured: false, revision: '' } })
    const kiwiHost = {
      ...binding('kiwivm'), id: 'host-1', name: 'JP01', instance_id: '1001', revision: 'r1',
      enabled: true, half_remaining: false, credential_configured: true,
    }
    const kiwiSaved = { ...base, hosts: [kiwiHost] }
    vi.mocked(saveProviderTrafficHost).mockResolvedValueOnce(kiwiSaved)
    const wrapper = await mountSettings(base)
    const addButtons = () => wrapper.findAll('button').filter((button) => button.text() === '添加主机')
    expect(addButtons()[1].attributes('disabled')).toBeDefined()
    await addButtons()[0].trigger('click')
    const kiwi = providerRow(wrapper, 'kiwivm')
    await kiwi.get('input[aria-label="主机名称"]').setValue('JP01')
    await kiwi.get('input[aria-label="搬瓦工 实例 ID"]').setValue('1001')
    await kiwi.get('input[aria-label="搬瓦工 凭据"]').setValue('synthetic-kiwi-key')
    await saveButton(kiwi)!.trigger('click')
    await flushPromises()
    expect(saveProviderTrafficHost).toHaveBeenCalledWith('kiwivm', '', {
      name: 'JP01', instance_id: '1001', revision: null, enabled: true,
      half_remaining: false, credential: 'synthetic-kiwi-key',
    })
    expect(wrapper.get('[data-host="host-1"]').text()).toContain('JP01')

    const accountSaved = { ...kiwiSaved, vps_account: { configured: true, revision: 'account-r1' } }
    vi.mocked(saveProviderTrafficVPSAccount).mockResolvedValueOnce(accountSaved)
    await wrapper.get('input[aria-label="V.PS 账号 Refresh Token"]').setValue('synthetic-refresh')
    await findButton(wrapper, '保存账号')!.trigger('click')
    await flushPromises()
    expect(saveProviderTrafficVPSAccount).toHaveBeenCalledWith('', 'synthetic-refresh')
    expect((wrapper.get('input[aria-label="V.PS 账号 Refresh Token"]').element as HTMLInputElement).value).toBe('')

    const vpsHost = { ...kiwiHost, provider: 'vps' as const, id: 'host-2', name: 'VPS01', instance_id: '2001', half_remaining: true }
    vi.mocked(saveProviderTrafficHost).mockResolvedValueOnce({ ...accountSaved, hosts: [kiwiHost, vpsHost] })
    await addButtons()[1].trigger('click')
    const vps = providerRow(wrapper, 'vps')
    expect(vps.find('input[type="password"]').exists()).toBe(false)
    await vps.get('input[aria-label="主机名称"]').setValue('VPS01')
    await vps.get('input[aria-label="V.PS 实例 ID"]').setValue('2001')
    await vps.get('[aria-label="剩余流量折半显示"]').trigger('click')
    await saveButton(vps)!.trigger('click')
    await flushPromises()
    expect(saveProviderTrafficHost).toHaveBeenLastCalledWith('vps', '', {
      name: 'VPS01', instance_id: '2001', revision: null, enabled: true, half_remaining: true,
    })
    expect(wrapper.get('[data-host="host-2"]').get('[aria-label="剩余流量折半显示"]').attributes('aria-checked')).toBe('true')
  })

  const originalBinding = binding('kiwivm', {
    enabled: true, instance_id: '1001', revision: 'rev-1', credential_configured: true,
  })
  const replacementBinding = binding('kiwivm', {
    enabled: true, instance_id: '2002', revision: 'rev-2', credential_configured: true,
  })
  const originalSettings = settingsData({
    enabled: true, providers: [originalBinding, binding('vps')],
  })
  const replacementSettings = settingsData({
    enabled: true, providers: [replacementBinding, binding('vps')],
  })

  it('高级模式只显示不可用提示，不暴露表单', async () => {
    const wrapper = await mountSettings({ available: false, enabled: false, providers: [] })
    expect(wrapper.text()).toContain('服务器剩余流量')
    expect(wrapper.text()).toContain('仅在基础模式下可用')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(saveButton(wrapper)).toBeUndefined()
    expect(wrapper.findAll('button[role="switch"]')).toHaveLength(0)
  })

  it('默认关闭时两家固定顺序展示，解除绑定与立即采集均不可用', async () => {
    const wrapper = await mountSettings()
    const rows = wrapper.findAll('.provider-traffic-row')
    expect(rows).toHaveLength(2)
    expect(rows[0].attributes('data-provider')).toBe('kiwivm')
    expect(rows[0].text()).toContain('搬瓦工')
    expect(rows[1].attributes('data-provider')).toBe('vps')
    expect(rows[1].text()).toContain('V.PS')
    expect(rows[0].text()).toContain('未绑定')
    expect(refreshButton(rows[0])?.attributes('disabled')).toBeDefined()
    expect(deleteButton(rows[0])?.attributes('disabled')).toBeDefined()
  })

  it('总开关独立保存，不调用任何单家保存接口', async () => {
    const wrapper = await mountSettings()
    const total = wrapper.find('.provider-traffic-total')
    await total.find('button[role="switch"]').trigger('click')
    await saveButton(total)!.trigger('click')
    await flushPromises()
    expect(mockSaveEnabled).toHaveBeenCalledWith(true)
    expect(mockSaveBinding).not.toHaveBeenCalled()
    expect(mockDeleteBinding).not.toHaveBeenCalled()
  })

  it('总开关关闭时仍可保存单家绑定，解除绑定也可用', async () => {
    const fixture = settingsData({
      enabled: false,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: okResult('kiwivm', 180.41, 'GiB'),
        }),
        binding('vps'),
      ],
    })
    mockSaveBinding.mockResolvedValue(fixture)
    const wrapper = await mountSettings(fixture)
    const row = providerRow(wrapper, 'kiwivm')
    expect(refreshButton(row)?.attributes('disabled')).toBeDefined()
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).toHaveBeenCalledWith('kiwivm', {
      enabled: true,
      instance_id: '1001',
      revision: 'rev-1',
    })
    expect(deleteButton(row)?.attributes('disabled')).toBeUndefined()
  })

  it('首次绑定提交 revision:null 与凭据，成功后清空输入并显示保存标记', async () => {
    mockSaveBinding.mockResolvedValue(settingsData({
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-new',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    }))
    const wrapper = await mountSettings()
    const row = providerRow(wrapper, 'kiwivm')
    await row.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1001')
    await row.find('input[aria-label="搬瓦工 凭据"]').setValue('synthetic-key')
    await row.find('button[role="switch"]').trigger('click')
    await saveButton(row)!.trigger('click')
    await flushPromises()

    expect(mockSaveBinding).toHaveBeenCalledWith('kiwivm', {
      enabled: true,
      instance_id: '1001',
      revision: null,
      credential: 'synthetic-key',
    })
    expect((wrapper.find('input[aria-label="搬瓦工 凭据"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('已保存')
  })

  it('同一实例留空按省略 credential 提交，保留已保存凭据', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    })
    mockSaveBinding.mockResolvedValue(fixture)
    const wrapper = await mountSettings(fixture)
    const row = providerRow(wrapper, 'kiwivm')
    expect(row.text()).toContain('已保存')
    expect((row.find('input[aria-label="搬瓦工 凭据"]').element as HTMLInputElement).value).toBe('')
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).toHaveBeenCalledWith('kiwivm', {
      enabled: true,
      instance_id: '1001',
      revision: 'rev-1',
    })
    const payload = mockSaveBinding.mock.calls[0][1] as Record<string, unknown>
    expect(payload).not.toHaveProperty('credential')
  })

  it('首次绑定未填写凭据时被拦截', async () => {
    const wrapper = await mountSettings()
    const row = providerRow(wrapper, 'kiwivm')
    await row.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1001')
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).not.toHaveBeenCalled()
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('首次绑定必须填写凭据')
  })

  it('更换实例留空被拦截，不发送空串或 null 代替保留', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    })
    const wrapper = await mountSettings(fixture)
    const row = providerRow(wrapper, 'kiwivm')
    await row.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1002')
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).not.toHaveBeenCalled()
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('更换实例必须填写新凭据')
  })

  it('实例 ID 只接受规范正十进制数字', async () => {
    const wrapper = await mountSettings()
    const row = providerRow(wrapper, 'kiwivm')
    await row.find('input[aria-label="搬瓦工 实例 ID"]').setValue('0012')
    await row.find('input[aria-label="搬瓦工 凭据"]').setValue('synthetic-key')
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).not.toHaveBeenCalled()
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('实例 ID 必须为正十进制数字')
  })

  it('保存旧修订冲突提示刷新，不自动重放写操作', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    })
    mockSaveBinding.mockRejectedValueOnce(new ApiError(409, '绑定修订冲突'))
    const wrapper = await mountSettings(fixture)
    await saveButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('绑定已在其他页面更改，请刷新页面后重试')
  })

  it('解除绑定经确认后携带当前已保存修订，响应后隐藏结果', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: okResult('kiwivm', 180.41, 'GiB'),
        }),
        binding('vps'),
      ],
    })
    mockDeleteBinding.mockResolvedValueOnce(settingsData({
      enabled: true,
      providers: [binding('kiwivm'), binding('vps')],
    }))
    const wrapper = await mountSettings(fixture)
    expect(wrapper.find('[data-provider="kiwivm"]').text()).toContain('180.41')
    await deleteButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    const modal = wrapper.findComponent(ConfirmModal)
    expect(modal.props('open')).toBe(true)
    expect(String(modal.props('content'))).toContain('搬瓦工')
    expect(String(modal.props('content'))).toContain('1001')
    modal.vm.$emit('confirm')
    await flushPromises()
    expect(mockDeleteBinding).toHaveBeenCalledWith('kiwivm', 'rev-1')
    const row = wrapper.find('[data-provider="kiwivm"]')
    expect(row.text()).toContain('未绑定')
    expect(row.text()).not.toContain('180.41')
  })

  it('解除绑定旧修订冲突提示且保留原结果', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: okResult('kiwivm', 180.41, 'GiB'),
        }),
        binding('vps'),
      ],
    })
    mockDeleteBinding.mockRejectedValueOnce(new ApiError(409, '绑定修订冲突'))
    const wrapper = await mountSettings(fixture)
    await deleteButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    wrapper.findComponent(ConfirmModal).vm.$emit('confirm')
    await flushPromises()
    expect(mockDeleteBinding).toHaveBeenCalledTimes(1)
    const row = wrapper.find('[data-provider="kiwivm"]')
    expect(row.text()).toContain('绑定已在其他页面更改')
    expect(row.text()).toContain('180.41')
  })

  it('切回基础模式等待重读，旧确认不能删除新绑定', async () => {
    const wrapper = await mountSettings(originalSettings)
    await deleteButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    const modal = wrapper.findComponent(ConfirmModal)
    expect(String(modal.props('content'))).toContain('1001')

    const system = useSystemStore()
    system.status = { advanced_mode: true } as SystemStatus
    await flushPromises()
    expect(modal.props('open')).toBe(false)
    const pending = deferred<ProviderTrafficSettingsData>()
    mockSettings.mockReturnValueOnce(pending.promise)
    system.status = { advanced_mode: false } as SystemStatus
    await flushPromises()
    expect(wrapper.text()).toContain('加载中')
    expect(deleteButton(wrapper)).toBeUndefined()
    modal.vm.$emit('confirm')
    pending.resolve(replacementSettings)
    await flushPromises()
    modal.vm.$emit('confirm')
    expect(mockDeleteBinding).not.toHaveBeenCalled()

    await deleteButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    expect(String(modal.props('content'))).toContain('2002')
    modal.vm.$emit('confirm')
    await flushPromises()
    expect(mockDeleteBinding).toHaveBeenCalledExactlyOnceWith('kiwivm', 'rev-2')
  })

  it('立即采集仅在已保存绑定、双方启用且无草稿时可用', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps', { enabled: false }),
      ],
    })
    const wrapper = await mountSettings(fixture)
    const kiwi = providerRow(wrapper, 'kiwivm')
    expect(refreshButton(kiwi)?.attributes('disabled')).toBeUndefined()

    await kiwi.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1002')
    expect(refreshButton(wrapper.find('[data-provider="kiwivm"]'))?.attributes('disabled')).toBeDefined()
  })

  it('总开关关闭、平台停用或未绑定时禁用立即采集', async () => {
    const totalOff = await mountSettings(settingsData({
      enabled: false,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    }))
    expect(refreshButton(providerRow(totalOff, 'kiwivm'))?.attributes('disabled')).toBeDefined()
    totalOff.unmount()

    const platformOff = await mountSettings(settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: false,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    }))
    expect(refreshButton(providerRow(platformOff, 'kiwivm'))?.attributes('disabled')).toBeDefined()
    platformOff.unmount()

    const unbound = await mountSettings(settingsData({ enabled: true }))
    expect(refreshButton(providerRow(unbound, 'kiwivm'))?.attributes('disabled')).toBeDefined()
  })

  it('手动采集只用已保存绑定，慢响应中不重复发送，并显示管理员 hint', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: okResult('kiwivm', 180.41, 'GiB'),
        }),
        binding('vps'),
      ],
    })
    const pending = deferred<ProviderTrafficResult>()
    mockRefresh.mockReturnValueOnce(pending.promise)
    const wrapper = await mountSettings(fixture)
    const row = providerRow(wrapper, 'kiwivm')
    await refreshButton(row)!.trigger('click')
    await flushPromises()
    expect(mockRefresh).toHaveBeenCalledTimes(1)
    expect(mockRefresh).toHaveBeenCalledWith('kiwivm', 'rev-1')
    const loadingButton = refreshButton(providerRow(wrapper, 'kiwivm'))!
    expect(loadingButton.attributes('disabled')).toBeDefined()
    await loadingButton.trigger('click')
    await flushPromises()
    expect(mockRefresh).toHaveBeenCalledTimes(1)

    pending.resolve({
      provider: 'kiwivm',
      status: 'error',
      message: '服务商凭据被拒绝',
      hint: '请重新填写对应凭据',
    })
    await flushPromises()
    const text = providerRow(wrapper, 'kiwivm').text()
    expect(text).toContain('采集失败')
    expect(text).toContain('服务商凭据被拒绝')
    expect(text).toContain('请重新填写对应凭据')
    expect(text).not.toContain('180.41')
  })

  it('站内 409 只显示请求错误，不用错误结果覆盖已成功余量', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: okResult('kiwivm', 180.41, 'GiB'),
        }),
        binding('vps'),
      ],
    })
    mockRefresh.mockRejectedValueOnce(new ApiError(409, '正在采集，请稍后再试'))
    const wrapper = await mountSettings(fixture)
    await refreshButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    await flushPromises()
    expect(mockRefresh).toHaveBeenCalledTimes(1)
    const row = providerRow(wrapper, 'kiwivm')
    expect(row.text()).toContain('正在采集，请稍后再试')
    expect(row.text()).toContain('180.41')
  })

  it('页面的绑定修订过期时，手动采集显示冲突且不重试', async () => {
    mockRefresh.mockRejectedValueOnce(new ApiError(409, '绑定修订冲突'))
    const wrapper = await mountSettings(originalSettings)
    const row = providerRow(wrapper, 'kiwivm')
    await refreshButton(row)!.trigger('click')
    await flushPromises()
    expect(mockRefresh).toHaveBeenCalledExactlyOnceWith('kiwivm', 'rev-1')
    expect(row.text()).toContain('绑定已在其他页面更改')
    expect(row.text()).not.toContain('采集失败')
  })

  it('手动采集的迟到响应不覆盖已重新读取的新绑定结果', async () => {
    const pending = deferred<ProviderTrafficResult>()
    mockRefresh.mockReturnValueOnce(pending.promise)
    mockSaveEnabled.mockResolvedValueOnce(settingsData({
      enabled: true,
      providers: [{ ...replacementBinding, result: okResult('kiwivm', 200.25, 'GiB') }, binding('vps')],
    }))
    const wrapper = await mountSettings(originalSettings)
    await refreshButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    await saveButton(wrapper.find('.provider-traffic-total'))!.trigger('click')
    await flushPromises()
    pending.resolve(okResult('kiwivm', 100.25, 'GiB'))
    await flushPromises()
    const row = providerRow(wrapper, 'kiwivm')
    expect(row.text()).toContain('200.25')
    expect(row.text()).not.toContain('100.25')
  })

  it('保存单家不覆盖总开关未保存草稿，总开关保存不覆盖单家未保存草稿', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    })
    mockSaveBinding.mockResolvedValueOnce(fixture)
    mockSaveEnabled.mockResolvedValueOnce(fixture)
    const wrapper = await mountSettings(fixture)

    const total = wrapper.find('.provider-traffic-total')
    await total.find('button[role="switch"]').trigger('click')
    expect(total.find('button[role="switch"]').attributes('aria-checked')).toBe('false')

    await saveButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.provider-traffic-total button[role="switch"]').attributes('aria-checked')).toBe('false')

    await providerRow(wrapper, 'kiwivm').find('input[aria-label="搬瓦工 实例 ID"]').setValue('9999')
    await wrapper.find('.provider-traffic-total button[role="switch"]').trigger('click')
    await saveButton(wrapper.find('.provider-traffic-total'))!.trigger('click')
    await flushPromises()
    expect(mockSaveEnabled).toHaveBeenCalledTimes(1)
    expect((wrapper.find('input[aria-label="搬瓦工 实例 ID"]').element as HTMLInputElement).value).toBe('9999')
  })

  it.each([
    { name: '改绑', initial: originalBinding, updated: replacementBinding },
    { name: '同实例换证', initial: originalBinding, updated: { ...replacementBinding, instance_id: '1001' } },
    { name: '其他页面先完成首次绑定', initial: binding('kiwivm'), updated: originalBinding },
  ])('$name后保留草稿并阻止旧草稿提交，允许显式采用最新绑定', async ({ initial, updated }) => {
    const wrapper = await mountSettings(settingsData({
      enabled: true, providers: [initial, binding('vps')],
    }))
    const row = providerRow(wrapper, 'kiwivm')
    await row.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1001')
    await row.find('input[aria-label="搬瓦工 凭据"]').setValue('synthetic-draft-key')
    const after = settingsData({ enabled: false, providers: [updated, binding('vps')] })
    mockSaveEnabled.mockResolvedValueOnce(after)
    const total = wrapper.find('.provider-traffic-total')
    await total.find('button[role="switch"]').trigger('click')
    await saveButton(total)!.trigger('click')
    await flushPromises()
    expect(row.find<HTMLInputElement>('input[aria-label="搬瓦工 凭据"]').element.value).toBe('synthetic-draft-key')
    expect(wrapper.emitted('dirty-change')?.slice(-1)).toEqual([[true]])
    expect(saveButton(row)?.attributes('disabled')).toBeDefined()
    expect(refreshButton(row)?.attributes('disabled')).toBeDefined()
    await saveButton(row)!.trigger('click')
    expect(mockSaveBinding).not.toHaveBeenCalled()

    await findButton(row, '使用最新绑定')!.trigger('click')
    expect(row.find<HTMLInputElement>('input[aria-label="搬瓦工 实例 ID"]').element.value).toBe(updated.instance_id)
    expect(row.find<HTMLInputElement>('input[aria-label="搬瓦工 凭据"]').element.value).toBe('')
    expect(wrapper.emitted('dirty-change')?.slice(-1)).toEqual([[false]])
    mockSaveBinding.mockResolvedValueOnce(after)
    await saveButton(row)!.trigger('click')
    await flushPromises()
    expect(mockSaveBinding).toHaveBeenCalledExactlyOnceWith('kiwivm', {
      enabled: true, instance_id: updated.instance_id, revision: updated.revision,
    })
  })

  it('dirty-change 随草稿与保存结果变化，供父页未保存提示使用', async () => {
    const fixture = settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    })
    mockSaveBinding.mockResolvedValueOnce(fixture)
    const wrapper = await mountSettings(fixture)
    const initialEvents = wrapper.emitted('dirty-change')!
    expect(initialEvents[initialEvents.length - 1]).toEqual([false])
    await wrapper.find('input[aria-label="搬瓦工 实例 ID"]').setValue('1002')
    await flushPromises()
    const dirtyEvents = wrapper.emitted('dirty-change')!
    expect(dirtyEvents[dirtyEvents.length - 1]).toEqual([true])
    await wrapper.find('input[aria-label="搬瓦工 凭据"]').setValue('synthetic-key')
    await saveButton(providerRow(wrapper, 'kiwivm'))!.trigger('click')
    await flushPromises()
    const savedEvents = wrapper.emitted('dirty-change')!
    expect(savedEvents[savedEvents.length - 1]).toEqual([false])
  })

  it('等待采集与管理员结果错误 hint 均可显示', async () => {
    const waiting = await mountSettings(settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
        }),
        binding('vps'),
      ],
    }))
    expect(providerRow(waiting, 'kiwivm').text()).toContain('等待采集')
    waiting.unmount()

    const failed = await mountSettings(settingsData({
      enabled: true,
      providers: [
        binding('kiwivm', {
          enabled: true,
          instance_id: '1001',
          revision: 'rev-1',
          credential_configured: true,
          result: {
            provider: 'kiwivm',
            status: 'error',
            message: '计费倍率待核对',
            hint: '请核对面板与官方口径',
          },
        }),
        binding('vps'),
      ],
    }))
    const text = providerRow(failed, 'kiwivm').text()
    expect(text).toContain('采集失败')
    expect(text).toContain('计费倍率待核对')
    expect(text).toContain('请核对面板与官方口径')
  })
})
