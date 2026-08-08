// api/custom.ts：自定义订阅接口封装（入口在 Build3 用户管理；本 Build 预留接口与版本路由）
import { http } from './request'

export interface CustomItem {
  id: number
  slug: string
  user_id: number
  platform_id: number
  platform_name?: string
  current_version: number
}

export const upsertCustom = (userId: number, payload: FormData) =>
  http.post<any, CustomItem>(`/admin/users/${userId}/custom`, payload)
export const upsertCustomText = (userId: number, payload: { platform_id: number; text: string }) =>
  http.post<any, CustomItem>(`/admin/users/${userId}/custom?mode=text`, payload)
export const deleteCustom = (userId: number, platformId: number) =>
  http.delete(`/admin/users/${userId}/custom/${platformId}`)
