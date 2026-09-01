// api/overview.ts：管理员概览的单一汇总接口契约（Build11 Step 5）。
import { http } from './request'
import type { PendingUser } from './approval'
import type { AccessLog } from './log'

export interface OverviewStatus {
  app_mode: string
  advanced_mode: boolean
  emergency: boolean
}

export interface OverviewCounts {
  platforms: number
  subscriptions: number
  nodes: number
  usable_nodes: number
  manual_nodes: number
  xray_nodes: number
  rules: number
  shares: number
  users: number
  pending_users: number
  pools: number
  proxy_groups: number
  xray_instances: number
  ext_accounts: number
}

export interface OverviewChecklistItem {
  key: 'platforms' | 'subscriptions' | 'nodes' | 'version_active' | 'member_check'
  done: boolean
  manual?: boolean
  label: string
  action_path: string
  action_label: string
}

export interface AdminOverview {
  status: OverviewStatus
  counts: OverviewCounts
  checklist: OverviewChecklistItem[]
  recent: {
    pending_users: PendingUser[]
    access_logs: AccessLog[]
  }
}

export const getAdminOverview = () => http.get<any, AdminOverview>('/admin/overview')
