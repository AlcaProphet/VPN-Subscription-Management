// api/pool.ts：规则素材池域接口（Design2-UI §9.1）
import { http } from './request'

export type SourceMode = 'clash' | 'shadowrocket' | 'auto'

export interface PoolSourceItem {
  id: number
  pool_id: number
  kind: 'manual' | 'url'
  url?: string
  source_mode: SourceMode
  sort_order: number
  active_snapshot_id?: number
  pending_snapshot_id?: number
}

export interface PoolItem {
  id: number
  name: string
  urls: string[]
  sources?: PoolSourceItem[]
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
  source_id?: number
  ok: boolean
  format?: string
  profile?: string
  accepted?: number
  excluded?: number
  rejected?: number
  duplicates?: number
  pending?: boolean
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

export interface SourceInput {
  url: string
  source_mode: SourceMode
}

export interface RuleCountItem {
  family: string
  matcher: string
  scope: string
  accepted: number
  excluded: number
  rejected: number
  duplicates: number
}

export interface DetectionStats {
  evidence_codes: string[]
  recognition_required_percent?: number | null
}

export interface PreviousActiveSnapshot {
  snapshot_id: number
  format: string
  profile: string
  accepted: number
}

export interface ComparisonStats {
  previous_active?: PreviousActiveSnapshot | null
  format_changed: boolean
  profile_changed: boolean
  accepted_drop_threshold_percent: number
  accepted_drop_triggered: boolean
}

export interface DecisionStats {
  initial_status: string
  reason_codes: string[]
}

export interface SnapshotStats {
  schema_version: number
  source_mode: string
  detection?: DetectionStats | null
  rule_counts: RuleCountItem[]
  unclassified_rejected: number
  comparison?: ComparisonStats | null
  decision?: DecisionStats | null
}

export interface SourceDiagnostic {
  line: number
  kind: string
  message: string
  raw: string
}

export interface SourceSnapshot {
  id: number
  source_id: number
  format: string
  profile: string
  status: string
  input: number
  recognized: number
  accepted: number
  excluded: number
  rejected: number
  duplicates: number
  diagnostics: SourceDiagnostic[]
  stats: SnapshotStats
  error?: string
  activated_at?: string | null
  created_at?: string | null
}

export interface SourceStatusItem {
  source_id: number
  display_url: string
  source_mode: SourceMode
  never_synced: boolean
  latest_attempt?: SourceSnapshot | null
  active?: SourceSnapshot | null
  pending?: SourceSnapshot | null
  latest_failed?: SourceSnapshot | null
}

export const listSourceStatuses = (poolId: number) =>
  http.get<any, { list: SourceStatusItem[]; total: number }>(`/admin/pools/${poolId}/sources/status`).then((d) => d.list)
export const listSourceSnapshots = (poolId: number, sourceId: number, page = 1, pageSize = 20) =>
  http.get<any, { list: SourceSnapshot[]; total: number }>(`/admin/pools/${poolId}/sources/${sourceId}/snapshots`, {
    params: { page, page_size: pageSize },
  })

export const listPools = () =>
  http.get<any, { list: PoolItem[]; total: number }>('/admin/pools').then((d) => d.list)
export const createPool = (data: { name: string; sources: SourceInput[]; auto_sync: boolean; sync_time: string }) =>
  http.post<any, PoolItem>('/admin/pools', data)
export const updatePool = (id: number, data: { name: string; sources: SourceInput[]; auto_sync: boolean; sync_time: string }) =>
  http.put(`/admin/pools/${id}`, data)
export const deletePool = (id: number) => http.delete(`/admin/pools/${id}`)

export const listEntries = (poolId: number, page = 1, pageSize = 20, source?: 'manual' | 'url') =>
  http.get<any, { list: PoolEntryItem[]; total: number }>(`/admin/pools/${poolId}/entries`, {
    params: { page, page_size: pageSize, ...(source ? { source } : {}) },
  })
export const activatePending = (poolId: number, sourceId: number, snapshotId: number) =>
  http.post(`/admin/pools/${poolId}/sources/${sourceId}/pending/${snapshotId}/activate`)
export const discardPending = (poolId: number, sourceId: number, snapshotId: number) =>
  http.delete(`/admin/pools/${poolId}/sources/${sourceId}/pending/${snapshotId}`)

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
export const clearSyncTasks = (poolId: number) =>
  http.delete<any, { cleared: number }>(`/admin/pools/${poolId}/sync/tasks/completed`)
