// api/group.ts：用户组接口封装（Build4：旧订阅选定移除；高级节点字段 Build6/7 追加）
import { http } from './request'

export interface GroupItem {
  id: number
  slug: string
  name: string
  is_default: boolean
  default_quota?: number | null // advanced_mode=off 时后端省略
  node_count: number
  user_count: number
}

export interface GroupNode {
  node_id: number
  node_name: string
  display_name?: string | null
  render_name: string
  sort_order: number
  is_public: boolean
  source: string
}

export interface CandidateNode {
  node_id: number
  name: string
  render_name: string
  is_public: boolean
  in_partial_blueprint: boolean
}

export interface GroupDetail extends GroupItem {
  nodes?: GroupNode[]
  candidate_nodes?: CandidateNode[]
}

export const listGroups = () =>
  http.get<any, { list: GroupItem[]; total: number }>('/admin/groups').then((d) => d.list)
export const getGroup = (id: number) =>
  http.get<any, { group: GroupItem; nodes?: GroupNode[]; candidate_nodes?: CandidateNode[] }>(`/admin/groups/${id}`)
    .then((d) => ({ ...d.group, nodes: d.nodes ?? [], candidate_nodes: d.candidate_nodes ?? [] }) as GroupDetail)
export const createGroup = (name: string) => http.post<any, GroupItem>('/admin/groups', { name })
export const updateGroup = (id: number, data: { name: string }) =>
  http.put(`/admin/groups/${id}`, data)
export const deleteGroup = (id: number) => http.delete(`/admin/groups/${id}`)
export const updateGroupNodes = (id: number, data: { node_ids: number[] }) =>
  http.put(`/admin/groups/${id}/nodes`, data)
export const updateGroupQuota = (id: number, data: { default_quota?: number | null }) =>
  http.put(`/admin/groups/${id}/quota`, data)
