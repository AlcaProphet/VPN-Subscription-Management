// api/user.ts：管理员用户管理接口（Build3 Step 1）
import { http } from './request'

export interface CustomSubItem {
  id: number
  platform_id: number
  platform_name: string
}

export interface AdminUser {
  id: number
  username: string
  email: string
  role: 'admin' | 'user'
  group_id: number
  group_name: string
  source: 'oidc' | 'local' | 'selfreg'
  status: 'pending' | 'active' | 'disabled'
  has_password: boolean
  has_oidc_binding: boolean
  custom_subs: CustomSubItem[]
  // 高级模式字段
  used_bytes?: number
  effective_quota?: number | null
  quota_override?: number | null
  quota_exceeded?: boolean
  sync_status?: string
  sync_error?: string
}

// 分页列表保留 {list,total} 包裹（调用方取 list/total），与全量列表 api 的 .then(d => d.list) 解包约定区分（R02-01）
export const listUsers = (q: { page: number; size: number; keyword: string }) =>
  http.get<any, { list: AdminUser[]; total: number }>('/admin/users', { params: q })
export const createUser = (data: { username: string; email: string; password: string }) =>
  http.post('/admin/users', data)
export const updateUser = (id: number, data: { group_id?: number; email?: string }) =>
  http.put(`/admin/users/${id}`, data)
export const changeRole = (id: number, role: string) => http.put(`/admin/users/${id}/role`, { role })
export const revokeTokens = (id: number) => http.post(`/admin/users/${id}/tokens/revoke`)
export const resetPassword = (id: number, data: { mode: 'send_email' | 'direct' }) =>
  http.post<any, { password?: string }>(`/admin/users/${id}/password/reset`, data)
export const clearOidc = (id: number) => http.delete<any, { has_password: boolean }>(`/admin/users/${id}/oidc`)
export const setStatus = (id: number, disabled: boolean) => http.put(`/admin/users/${id}/status`, { disabled })
export const deleteUser = (id: number) => http.delete(`/admin/users/${id}`)
export const sendPasswordLinks = () =>
  http.post<any, { sent: number; skipped_pending: number; skipped_disabled: number; skipped_no_email: number }>(
    '/admin/users/send_password_links',
  )
export const setUserQuota = (id: number, data: { quota_override?: number | null }) =>
  http.put(`/admin/users/${id}/quota`, data)
