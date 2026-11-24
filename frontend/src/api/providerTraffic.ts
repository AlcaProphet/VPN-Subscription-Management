// api/providerTraffic.ts：基础模式服务器剩余流量站内接口与固定展示常量（Build41 Step 5）。
// 仅封装 Step 4 已实现的六项接口；不直接请求服务商，也不在前端计算余量或处理 Token 续期。
import { http } from './request'

export type Provider = 'kiwivm' | 'vps'
export type ProviderTrafficStatus = 'waiting' | 'ok' | 'error'

export interface ProviderTrafficResult {
  provider: Provider
  status: ProviderTrafficStatus
  remaining?: number
  unit?: string
  sampled_at?: string
  message?: string
  hint?: string
}

export interface ProviderTrafficBinding {
  provider: Provider
  enabled: boolean
  instance_id: string | null
  revision: string | null
  credential_configured: boolean
  result: ProviderTrafficResult | null
}

export interface ProviderTrafficSettings {
  available: boolean
  enabled: boolean
  providers: ProviderTrafficBinding[]
}

export interface ProviderTrafficHome {
  list: ProviderTrafficResult[]
  total: number
}

export interface ProviderTrafficBindingPayload {
  enabled: boolean
  instance_id: string
  revision: string | null
  credential?: string
}

// 两家固定展示顺序与名称；后端同样固定返回该顺序。
export const PROVIDER_TRAFFIC_PROVIDERS = ['kiwivm', 'vps'] as const
export const PROVIDER_TRAFFIC_LABELS: Record<Provider, string> = {
  kiwivm: '搬瓦工',
  vps: 'V.PS',
}
// 首页停留时的只读刷新周期（Design7 §四）。
export const PROVIDER_TRAFFIC_POLL_INTERVAL_MS = 60_000
// 仅手动采集使用更长超时，覆盖 V.P.S 最慢的两次上游请求和本地处理；全局默认仍为 15 秒。
export const PROVIDER_TRAFFIC_REFRESH_TIMEOUT_MS = 25_000

export const getHomeProviderTraffic = () =>
  http.get<any, ProviderTrafficHome>('/home/provider-traffic')

export const getProviderTrafficSettings = () =>
  http.get<any, ProviderTrafficSettings>('/admin/settings/provider-traffic')

export const saveProviderTrafficEnabled = (enabled: boolean) =>
  http.put<any, ProviderTrafficSettings>('/admin/settings/provider-traffic', { enabled })

export const saveProviderTrafficBinding = (provider: Provider, payload: ProviderTrafficBindingPayload) =>
  http.put<any, ProviderTrafficSettings>(`/admin/settings/provider-traffic/${provider}`, payload)

// 解绑按冻结合同携带 JSON 请求体；Axios 使用 config.data 发送 DELETE body。
export const deleteProviderTrafficBinding = (provider: Provider, revision: string) =>
  http.delete<any, ProviderTrafficSettings>(`/admin/settings/provider-traffic/${provider}`, {
    data: { revision },
  })

// 仅采集当前已保存的指定绑定修订；HTTP 200 的平台 error 也通过此接口正常返回。
export const refreshProviderTraffic = (provider: Provider, revision: string) =>
  http.post<any, ProviderTrafficResult>(
    `/admin/settings/provider-traffic/${provider}/refresh`,
    { revision },
    { timeout: PROVIDER_TRAFFIC_REFRESH_TIMEOUT_MS },
  )
