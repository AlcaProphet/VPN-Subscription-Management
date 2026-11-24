// api/providerTraffic.ts：基础模式服务器剩余流量站内接口与固定展示常量（Build41 Step 5）。
// 封装站内流量接口，折半与 Token 续期由后端处理。
import { http } from './request'

export type Provider = 'kiwivm' | 'vps'
export type ProviderTrafficStatus = 'waiting' | 'ok' | 'error'

export interface ProviderTrafficResult {
  provider: Provider
  host_id?: string
  name?: string
  half_remaining?: boolean
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
  hosts?: ProviderTrafficHost[]
  vps_account?: ProviderTrafficAccount
}

export interface ProviderTrafficHost extends ProviderTrafficBinding {
  id: string
  name: string
  half_remaining: boolean
  instance_id: string
  revision: string
}

export interface ProviderTrafficAccount {
  configured: boolean
  revision: string
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

export interface ProviderTrafficHostPayload extends ProviderTrafficBindingPayload {
  name: string
  half_remaining: boolean
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

const hostPath = (provider: Provider, id = '') =>
  `/admin/settings/provider-traffic/hosts/${provider}${id ? `/${encodeURIComponent(id)}` : ''}`

export const saveProviderTrafficHost = (provider: Provider, id: string, payload: ProviderTrafficHostPayload) =>
  id
    ? http.put<any, ProviderTrafficSettings>(hostPath(provider, id), payload)
    : http.post<any, ProviderTrafficSettings>(hostPath(provider), payload)

export const deleteProviderTrafficHost = (provider: Provider, id: string, revision: string) =>
  http.delete<any, ProviderTrafficSettings>(hostPath(provider, id), { data: { revision } })

export const refreshProviderTrafficHost = (provider: Provider, id: string, revision: string) =>
  http.post<any, ProviderTrafficResult>(`${hostPath(provider, id)}/refresh`, { revision }, {
    timeout: PROVIDER_TRAFFIC_REFRESH_TIMEOUT_MS,
  })

const vpsAccountPath = '/admin/settings/provider-traffic/accounts/vps'

export const saveProviderTrafficVPSAccount = (revision: string, credential: string) =>
  http.put<any, ProviderTrafficSettings>(vpsAccountPath, { revision, credential })

export const deleteProviderTrafficVPSAccount = (revision: string) =>
  http.delete<any, ProviderTrafficSettings>(vpsAccountPath, { data: { revision } })
