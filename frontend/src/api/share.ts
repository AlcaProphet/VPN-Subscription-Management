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
// 双模式：FormData=文件上传；JSON=文本创建（必须带 ?mode=text，后端按查询参数区分）
export const createShare = (payload: FormData | { name: string; text: string }) => {
  const isText = !(payload instanceof FormData)
  return http.post<any, ShareItem>(`/admin/shares${isText ? '?mode=text' : ''}`, payload)
}
export const renameShare = (id: number, name: string) => http.put(`/admin/shares/${id}`, { name })
export const deleteShare = (id: number) => http.delete(`/admin/shares/${id}`)
export const refreshShareToken = (id: number) => http.post<any, { token: string }>(`/admin/shares/${id}/token/refresh`)
export const revokeShareToken = (id: number) => http.post(`/admin/shares/${id}/token/revoke`)
