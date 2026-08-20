// api/subscription.ts：订阅地址池接口封装（每平台一份订阅条目）
import { http } from './request'

export type ProductType = 'yaml' | 'subs' | 'generic-subs'

export interface SubscriptionItem {
  id: number
  slug: string
  name: string
  platform_id: number
  product_type: ProductType
  current_version: number
  content_kind: 'blueprint' | 'upload' | '' // Build5 起回填；无激活版本为空
  platform_name?: string
}

export const listSubscriptions = () =>
  http.get<any, { list: SubscriptionItem[]; total: number }>('/admin/subscriptions').then((d) => d.list)
export const getSubscription = (id: number) => http.get<any, SubscriptionItem>(`/admin/subscriptions/${id}`)
export const createSubscription = (data: { platform_id: number; name: string; slug?: string }) =>
  http.post<any, SubscriptionItem>('/admin/subscriptions', data)
export const updateSubscription = (id: number, data: { name: string }) =>
  http.put(`/admin/subscriptions/${id}`, data)
export const deleteSubscription = (id: number) => http.delete(`/admin/subscriptions/${id}`)
// 会话凭据按平台预览当前版本（管理员首页预览形态；无 subscription_id 参数）
export const previewSubscriptionByPlatform = (platformSlug: string) =>
  http.get<any, string>(`/subscriptions/preview?platform=${encodeURIComponent(platformSlug)}`, {
    responseType: 'text',
  })
