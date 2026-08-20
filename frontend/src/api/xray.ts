// api/xray.ts：Xray 实例、独立账号与对账接口（Build7 Step3）
import { http } from './request'

export interface XrayInstance {
  id: number
  name: string
  slug: string
  api_addr: string
  api_tag: string
  enabled: boolean
  last_collect_at?: string | null
  collect_status?: string
  collect_error?: string
}

export interface AddedNode {
  node_id: number
  tag: string
  name: string
}

export interface DetectResult {
  added: number
  updated: number
  missing: number
  skipped: { tag: string; reason: string }[]
  added_nodes: AddedNode[]
}

export interface ExtPushTarget {
  instance_id: number
  inbound_tag: string
  node_id?: number
  name?: string
  display_name?: string | null
  render_name?: string
}

export interface ExtAccount {
  id: number
  name: string
  email: string
  quota?: number | null
  quota_exceeded: boolean
  push_targets?: ExtPushTarget[]
  used_bytes?: number
}

export interface ExtCredentials {
  uuid: string
  proxy_secret: string
}

export interface ReconcileItem {
  email: string
  source: 'user' | 'ext'
  user_id?: number
  ext_account_id?: number
  instance_id: number
  inbound_tag: string
  node_id: number
  name?: string
  render_name?: string
}

export interface ReconcileResult {
  to_push: ReconcileItem[]
  orphans: ReconcileItem[]
  ext_orphans: ReconcileItem[]
  credential_mismatches: ReconcileItem[]
}

export interface AdminTask {
  id: string
  kind: string
  status: 'running' | 'succeeded' | 'failed'
  result?: unknown
  error?: string
}

export const listInstances = () =>
  http.get<any, { list: XrayInstance[]; total: number }>('/admin/xray/instances').then((d) => d.list)
export const createInstance = (data: { name: string; api_addr: string; api_tag?: string; enabled?: boolean }) =>
  http.post<any, XrayInstance>('/admin/xray/instances', data)
export const updateInstance = (id: number, data: { name: string; api_addr: string; api_tag?: string; enabled?: boolean }) =>
  http.put<any, XrayInstance>(`/admin/xray/instances/${id}`, data)
export const deleteInstance = (id: number) =>
  http.delete<any, { task_id: string }>(`/admin/xray/instances/${id}`)
export const testConnection = (apiAddr: string, timeout = 120000) =>
  http.post<any, { ok: boolean }>('/admin/xray/instances/test', { api_addr: apiAddr }, { timeout })
export const detectNodes = (id: number, timeout = 120000) =>
  http.post<any, DetectResult>(`/admin/xray/instances/${id}/detect`, {}, { timeout })
export const runInit = () => http.post<any, { task_id: string }>('/admin/xray/init', {})
export const getUserSync = (id: number) => http.get<any, { list: unknown[] }>(`/admin/xray/users/${id}/sync`)
export const retryUserSync = (id: number, timeout = 120000) =>
  http.post<any, Record<string, number>>(`/admin/xray/users/${id}/retry`, {}, { timeout })
export const resetQuota = (id: number, timeout = 120000) =>
  http.post(`/admin/xray/users/${id}/reset-quota`, {}, { timeout })
export const getInstanceStats = (id: number) => http.get<any, XrayInstance>(`/admin/xray/instances/${id}/stats`)
export const reconcile = (id: number, timeout = 120000) =>
  http.get<any, ReconcileResult>(`/admin/xray/instances/${id}/reconcile`, { timeout })
export const pushRepair = (id: number) => http.post<any, { task_id: string }>(`/admin/xray/instances/${id}/reconcile/push`, {})
export const cleanOrphans = (id: number, emails: string[]) =>
  http.post<any, { task_id: string }>(`/admin/xray/instances/${id}/reconcile/clean`, { emails })
export const repairCredentials = (id: number, items: ReconcileItem[]) =>
  http.post<any, { task_id: string }>(`/admin/xray/instances/${id}/reconcile/credentials`, { items })
export const pushOne = (id: number, item: ReconcileItem, timeout = 120000) =>
  http.post(`/admin/xray/instances/${id}/reconcile/push-one`, item, { timeout })
export const repairCredentialsOne = (id: number, item: ReconcileItem, timeout = 120000) =>
  http.post(`/admin/xray/instances/${id}/reconcile/credentials-one`, item, { timeout })

export const listExtAccounts = () =>
  http.get<any, { list: ExtAccount[]; total: number }>('/admin/xray/ext').then((d) => d.list)
export const createExtAccount = (data: {
  name: string
  credential_mode: 'generate' | 'manual'
  uuid?: string
  proxy_secret?: string
  quota?: number | null
  push_targets: ExtPushTarget[]
}, timeout = 120000) =>
  http.post<any, { account: ExtAccount; credentials?: ExtCredentials }>('/admin/xray/ext', data, { timeout })
export const updateExtAccount = (id: number, data: {
  name: string
  uuid?: string
  proxy_secret?: string
  quota?: number | null
  push_targets: ExtPushTarget[]
}, timeout = 120000) =>
  http.put<any, ExtAccount>(`/admin/xray/ext/${id}`, data, { timeout })
export const deleteExtAccount = (id: number, timeout = 120000) =>
  http.delete(`/admin/xray/ext/${id}`, { timeout })
export const retryExtSync = (id: number, timeout = 120000) =>
  http.post<any, Record<string, number>>(`/admin/xray/ext/${id}/retry`, {}, { timeout })
export const getExtCredentials = (id: number) =>
  http.get<any, ExtCredentials>(`/admin/xray/ext/${id}/credentials`)
export const resetExtQuota = (id: number, timeout = 120000) =>
  http.post(`/admin/xray/ext/${id}/reset-quota`, {}, { timeout })
