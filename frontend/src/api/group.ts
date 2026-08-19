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

export const listGroups = () =>
  http.get<any, { list: GroupItem[]; total: number }>('/admin/groups').then((d) => d.list)
export const getGroup = (id: number) =>
  http.get<any, { group: GroupItem }>(`/admin/groups/${id}`).then((d) => d.group)
export const createGroup = (name: string) => http.post<any, GroupItem>('/admin/groups', { name })
export const updateGroup = (id: number, data: { name: string }) =>
  http.put(`/admin/groups/${id}`, data)
export const deleteGroup = (id: number) => http.delete(`/admin/groups/${id}`)
