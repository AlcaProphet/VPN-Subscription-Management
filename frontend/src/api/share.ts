// api/share.ts：分享订阅接口封装
import { http } from './request'

export interface ShareItem {
  id: number
  slug: string
  name: string
  token_status: 'active' | 'revoked'
  token: string // 仅 active 时返回
  current_version: number
  created_at: string | null // UTC RFC3339；空值 null（R07-04）
}

export const listShares = () =>
  http.get<any, { list: ShareItem[]; total: number }>('/admin/shares').then((d) => d.list)
export const createShare = (payload: FormData) => http.post<any, ShareItem>('/admin/shares', payload)
export const renameShare = (id: number, name: string) => http.put(`/admin/shares/${id}`, { name })
export const deleteShare = (id: number) => http.delete(`/admin/shares/${id}`)
export const refreshShareToken = (id: number) => http.post<any, { token: string }>(`/admin/shares/${id}/token/refresh`)
export const revokeShareToken = (id: number) => http.post(`/admin/shares/${id}/token/revoke`)
