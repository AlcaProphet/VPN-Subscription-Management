// api/rule.ts：规则接口封装（管理端 + 用户端）
import { http } from './request'

export interface RuleItem {
  id: number
  slug: string
  name: string
  client_type: string
  schemes: string[]
  token: string // 全局共享 Token（一键导入用）
  current_version: number
  created_at: string
  refreshed_at: string | null // UTC RFC3339；无 Token 时 null（R07-04）
}

export const listAdminRules = () =>
  http.get<any, { list: RuleItem[]; total: number }>('/admin/rules').then((d) => d.list)
export const createRule = (payload: FormData) => http.post<any, RuleItem>('/admin/rules', payload)
export const renameRule = (id: number, name: string) => http.put(`/admin/rules/${id}`, { name })
export const deleteRule = (id: number) => http.delete(`/admin/rules/${id}`)
export const refreshRuleToken = (id: number) => http.post<any, { token: string }>(`/admin/rules/${id}/token/refresh`)

// 用户端（仅会话）
export const userRules = () =>
  http.get<any, { list: RuleItem[]; total: number }>('/rules').then((d) => d.list)
export const previewRule = (id: number) => http.get<any, string>(`/rules/${id}/preview`, { responseType: 'text' })
