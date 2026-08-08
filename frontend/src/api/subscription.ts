// api/subscription.ts：订阅池接口封装
import { http } from './request'

export interface GroupBrief {
  id: number
  name: string
}

export interface SubscriptionItem {
  id: number
  slug: string
  name: string
  platform_id: number
  current_version: number
  groups: GroupBrief[]
  selected_by: number // 被多少组选定中（Step 3 接通）
}

export interface PlatformSubs {
  platform_id: number
  platform_name: string
  subscriptions: SubscriptionItem[]
}

export const listSubscriptions = () =>
  http.get<any, { list: PlatformSubs[]; total: number }>('/admin/subscriptions').then((d) => d.list)
export const getSubscription = (id: number) => http.get<any, SubscriptionItem>(`/admin/subscriptions/${id}`)
export const createSubscription = (data: { platform_id: number; name: string; slug: string; group_ids: number[] }) =>
  http.post<any, SubscriptionItem>('/admin/subscriptions', data)
export const updateSubscription = (id: number, data: { name: string; group_ids: number[] }) =>
  http.put(`/admin/subscriptions/${id}`, data)
export const deleteSubscription = (id: number) => http.delete(`/admin/subscriptions/${id}`)
export const checkSlug = (slug: string, type?: string, id?: number) =>
  http.get<any, { available: boolean }>('/admin/slug/check', { params: { slug, type, id } })
