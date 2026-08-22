// api/pool.ts：规则素材池域接口（Design2-UI §9.1）
import { http } from './request'

export interface PoolItem {
  id: number
  name: string
  urls: string[]
  entry_count: number
  last_synced_at?: string | null
  sync_status: string // '' / running / succeeded / failed / partial
  sync_error: string
  auto_sync: boolean
  sync_time: string
}

export interface PoolEntryItem {
  id: number
  pool_id: number
  rule_type: string
  match_value: string
  source: 'url' | 'manual'
  sort_order: number
}

export interface PerURLResult {
  url: string
  ok: boolean
  added: number
  removed: number
  skipped: number
  skip_reasons?: string[]
  error: string
}

export interface SyncTaskItem {
  task_id: number
  pool_id: number
  status: 'running' | 'succeeded' | 'failed' | 'partial'
  per_url: PerURLResult[]
  error: string
  started_at?: string | null
  finished_at?: string | null
}

export const listPools = () =>
  http.get<any, { list: PoolItem[]; total: number }>('/admin/pools').then((d) => d.list)
export const createPool = (data: { name: string; urls: string[]; auto_sync: boolean; sync_time: string }) =>
  http.post<any, PoolItem>('/admin/pools', data)
export const updatePool = (id: number, data: { name: string; urls: string[]; auto_sync: boolean; sync_time: string }) =>
  http.put(`/admin/pools/${id}`, data)
export const deletePool = (id: number) => http.delete(`/admin/pools/${id}`)

export const listEntries = (poolId: number, page = 1, pageSize = 20) =>
  http.get<any, { list: PoolEntryItem[]; total: number }>(`/admin/pools/${poolId}/entries`, {
    params: { page, page_size: pageSize },
  })
export const createEntry = (poolId: number, data: { rule_type: string; match_value: string }) =>
  http.post<any, PoolEntryItem>(`/admin/pools/${poolId}/entries`, data)
export const updateEntry = (poolId: number, entryId: number, data: { rule_type: string; match_value: string }) =>
  http.put(`/admin/pools/${poolId}/entries/${entryId}`, data)
export const deleteEntry = (poolId: number, entryId: number) =>
  http.delete(`/admin/pools/${poolId}/entries/${entryId}`)

export const submitSync = (poolId: number) =>
  http.post<any, { task_id: number }>(`/admin/pools/${poolId}/sync`)
export const cancelSync = (poolId: number, taskId: number) =>
  http.post<any, { task_id: number; status: string }>(`/admin/pools/${poolId}/sync/tasks/${taskId}/cancel`)
export const getSyncStatus = (poolId: number) =>
  http.get<any, SyncTaskItem>(`/admin/pools/${poolId}/sync/status`)
export const listSyncTasks = (poolId: number, page = 1, pageSize = 20) =>
  http.get<any, { list: SyncTaskItem[]; total: number }>(`/admin/pools/${poolId}/sync/tasks`, {
    params: { page, page_size: pageSize },
  })
