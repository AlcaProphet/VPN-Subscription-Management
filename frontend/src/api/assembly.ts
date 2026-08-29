// api/assembly.ts：订阅装配接口（Design2-UI §9.1）
import { http } from './request'
import type { NodeItem } from './node'
import type { ProxyGroupItem } from './proxyGroup'
import type { PoolItem } from './pool'
import type { PlatformItem } from './platform'
import type { RuleItem } from './rule'
import type { SubscriptionItem } from './subscription'

export type TargetSyntax = 'clash-yaml' | 'sr-subs' | 'generic-subs' | 'sr-conf'

export interface PoolSelection {
  pool_id: number
  target: string
}
export interface RuleLine {
  rule_type: string
  match_value: string
  target: string
}
export interface OverlayInput {
  merge_yaml?: string
  rules_yaml?: string
  proxies_yaml?: string
  groups_yaml?: string
}
export interface GenerateInput {
  target_syntax: TargetSyntax
  preview_hash?: string
  platform_id?: number
  rule_id?: number
  rule_name?: string
  fixed_params?: Record<string, unknown>
  node_names: string[]
  group_names: string[]
  group_node_orders?: Record<string, string[]>
  overseas_members: string[]
  fallback_group_members: string[]
  pools: PoolSelection[]
  custom_rules: RuleLine[]
  final_direction?: string
  overlay?: OverlayInput
}
export interface SkipItem {
  kind: string
  name: string
  reason: string
}
export interface PreviewResponse {
  content: string
  preview_hash: string
  skipped: SkipItem[]
  warnings: string[]
  name_changed?: Record<string, string>
}
export interface AssemblyContext {
  nodes: NodeItem[]
  proxy_groups: ProxyGroupItem[]
  pools: PoolItem[]
  platforms: PlatformItem[]
  rules: RuleItem[]
  subscriptions: SubscriptionItem[]
}
export interface BlueprintResponse {
  blueprint: {
    target_syntax: TargetSyntax
    version_no?: number
    platform_id?: number | null
    rule_id?: number | null
    fixed_params: Record<string, unknown>
    overlay?: OverlayInput
    selection: {
      node_names: string[]
      group_names: string[]
      group_node_orders?: Record<string, string[]>
      overseas_members: string[]
      fallback_group_members: string[]
      pools: PoolSelection[]
      final_direction?: string
      overlay?: OverlayInput
    }
    custom_rules: RuleLine[]
    render_plan: Record<string, unknown>
  }
  invalid_refs: Array<{ kind: string; name: string }>
  name_changed?: Record<string, string>
}

export const getAssemblyContext = () =>
  http.get<any, AssemblyContext>('/admin/assembly/context')
export const previewAssembly = (data: GenerateInput) =>
  http.post<any, PreviewResponse>('/admin/assembly/preview', data, { timeout: 120000 })
export const generateAssembly = (data: GenerateInput) =>
  http.post<any, { version_id: number; version_no: number; auto_activated: boolean; rule_id?: number; skipped: SkipItem[]; warnings: string[] }>(
    '/admin/assembly/generate', data, { timeout: 120000 },
  )
export const getBlueprint = (versionId: number) =>
  http.get<any, BlueprintResponse>(`/admin/versions/${versionId}/blueprint`)
