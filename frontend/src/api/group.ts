// api/group.ts：用户组接口封装
import { http } from './request'

export interface GroupItem {
  id: number
  slug: string
  name: string
  is_default: boolean
  needs_reselect: boolean
  sub_count: number // 关联订阅数
  user_count: number // 组内用户数
}

export interface SelectionItem {
  platform_id: number
  subscription_id: number // 0 = 取消选定
}

export interface GroupDetail extends GroupItem {
  selections: SelectionItem[]
}

export const listGroups = () =>
  http.get<any, { list: GroupItem[]; total: number }>('/admin/groups').then((d) => d.list)
export const getGroup = (id: number) => http.get<any, GroupDetail>(`/admin/groups/${id}`)
export const createGroup = (name: string) => http.post<any, GroupItem>('/admin/groups', { name })
export const updateGroup = (id: number, data: { name: string; sub_ids: number[]; selections: SelectionItem[] }) =>
  http.put(`/admin/groups/${id}`, data)
export const deleteGroup = (id: number) => http.delete(`/admin/groups/${id}`)
export const setSelections = (id: number, selections: SelectionItem[]) =>
  http.put(`/admin/groups/${id}/selections`, { selections })
