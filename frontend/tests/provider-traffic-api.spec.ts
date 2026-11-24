// provider-traffic-api.spec.ts：服务器剩余流量 API 封装的方法、请求体、超时与 200 平台错误语义（Build41 Step 5）。
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { http } from '@/api/request'
import {
  deleteProviderTrafficBinding,
  getHomeProviderTraffic,
  getProviderTrafficSettings,
  refreshProviderTraffic,
  saveProviderTrafficBinding,
  saveProviderTrafficEnabled,
} from '@/api/providerTraffic'

interface AdapterCall {
  method?: string
  url?: string
  data?: unknown
  timeout?: number
}

const settingsFixture = {
  available: true,
  enabled: false,
  providers: [],
}

function installAdapter(payload: unknown): AdapterCall[] {
  const calls: AdapterCall[] = []
  http.defaults.adapter = (async (config: any) => {
    calls.push(config)
    return {
      data: { code: 0, data: payload },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }) as any
  return calls
}

function jsonBody(call: AdapterCall): Record<string, unknown> {
  return JSON.parse(String(call.data)) as Record<string, unknown>
}

describe('providerTraffic API 封装', () => {
  beforeEach(() => {
    localStorage.clear()
    delete http.defaults.adapter
  })

  afterEach(() => {
    delete http.defaults.adapter
  })

  it('首页读取使用 GET、默认 15 秒超时并解包 list/total', async () => {
    const calls = installAdapter({ list: [{ provider: 'kiwivm', status: 'waiting' }], total: 1 })
    const data = await getHomeProviderTraffic()
    expect(calls).toHaveLength(1)
    expect(calls[0].method).toBe('get')
    expect(calls[0].url).toBe('/home/provider-traffic')
    expect(calls[0].timeout).toBe(15000)
    expect(data.list).toHaveLength(1)
    expect(data.total).toBe(1)
  })

  it('设置读取与总开关保存只提交 enabled', async () => {
    let calls = installAdapter(settingsFixture)
    await getProviderTrafficSettings()
    expect(calls[0].method).toBe('get')
    expect(calls[0].url).toBe('/admin/settings/provider-traffic')

    calls = installAdapter(settingsFixture)
    await saveProviderTrafficEnabled(true)
    expect(calls[0].method).toBe('put')
    expect(calls[0].url).toBe('/admin/settings/provider-traffic')
    expect(jsonBody(calls[0])).toEqual({ enabled: true })
    expect(calls[0].timeout).toBe(15000)
  })

  it('单家保存使用 PUT，首次绑定 revision:null，同实例省略 credential', async () => {
    let calls = installAdapter(settingsFixture)
    await saveProviderTrafficBinding('kiwivm', {
      enabled: true,
      instance_id: '1001',
      revision: null,
      credential: 'synthetic-key',
    })
    expect(calls[0].method).toBe('put')
    expect(calls[0].url).toBe('/admin/settings/provider-traffic/kiwivm')
    expect(jsonBody(calls[0])).toEqual({
      enabled: true,
      instance_id: '1001',
      revision: null,
      credential: 'synthetic-key',
    })

    calls = installAdapter(settingsFixture)
    await saveProviderTrafficBinding('vps', {
      enabled: true,
      instance_id: '2002',
      revision: 'rev-2',
    })
    expect(jsonBody(calls[0])).toEqual({
      enabled: true,
      instance_id: '2002',
      revision: 'rev-2',
    })
    expect(jsonBody(calls[0])).not.toHaveProperty('credential')
  })

  it('解绑使用 DELETE 并携带 JSON revision 请求体', async () => {
    const calls = installAdapter(settingsFixture)
    await deleteProviderTrafficBinding('kiwivm', 'revision-1')
    expect(calls[0].method).toBe('delete')
    expect(calls[0].url).toBe('/admin/settings/provider-traffic/kiwivm')
    expect(jsonBody(calls[0])).toEqual({ revision: 'revision-1' })
  })

  it('手动采集提交指定绑定修订，使用独立的 25 秒超时', async () => {
    const calls = installAdapter({
      provider: 'vps',
      status: 'error',
      message: '服务商凭据被拒绝',
      hint: '请重新填写对应凭据',
    })
    const result = await refreshProviderTraffic('vps', 'rev-vps')
    expect(calls[0].method).toBe('post')
    expect(calls[0].url).toBe('/admin/settings/provider-traffic/vps/refresh')
    expect(calls[0].timeout).toBe(25000)
    expect(jsonBody(calls[0])).toEqual({ revision: 'rev-vps' })
    expect(http.defaults.timeout).toBe(15000)
    expect(result).toEqual({
      provider: 'vps',
      status: 'error',
      message: '服务商凭据被拒绝',
      hint: '请重新填写对应凭据',
    })
  })

  it('HTTP 200 的平台 error 结果保留登录凭据，不触发站内退出', async () => {
    localStorage.setItem('token', 'session-token')
    installAdapter({
      provider: 'kiwivm',
      status: 'error',
      message: '服务商凭据被拒绝',
      hint: '请重新填写对应凭据',
    })
    const result = await refreshProviderTraffic('kiwivm', 'rev-kiwivm')
    expect(result.status).toBe('error')
    expect(result.message).toBe('服务商凭据被拒绝')
    expect(localStorage.getItem('token')).toBe('session-token')
  })
})
