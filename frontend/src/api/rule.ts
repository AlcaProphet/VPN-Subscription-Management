// api/rule.ts：规则接口封装（管理端 + 用户端）
import { http } from './request'

export interface RuleItem {
  id: number
  slug: string
  name: string
  client_type: string
  schemes: string[]
  token: string // 全局共享 Token（复制链接下载用）
  current_version: number
  is_home_default: boolean
  created_at: string
  refreshed_at: string | null // UTC RFC3339；无 Token 时 null
}

export const listAdminRules = () =>
  http.get<any, { list: RuleItem[]; total: number }>('/admin/rules').then((d) => d.list)
// 双模式：FormData=文件上传；JSON=文本创建（必须带 ?mode=text）；
// text 可选（留空=创建空规则实体，供 SR 分流规则装配目标）
export const createRule = (
  payload: FormData | { name: string; client_type: string; schemes: string[]; text?: string },
) => {
  const isText = !(payload instanceof FormData)
  return http.post<any, RuleItem>(`/admin/rules${isText ? '?mode=text' : ''}`, payload)
}
export const renameRule = (id: number, name: string) => http.put(`/admin/rules/${id}`, { name })
export const setHomeDefault = (id: number, isDefault: boolean) =>
  http.put(`/admin/rules/${id}/home-default`, { is_default: isDefault })
export const deleteRule = (id: number) => http.delete(`/admin/rules/${id}`)
export const refreshRuleToken = (id: number) => http.post<any, { token: string }>(`/admin/rules/${id}/token/refresh`)

// 用户端（仅会话）
export const userRules = () =>
  http.get<any, { list: RuleItem[]; total: number }>('/rules').then((d) => d.list)
export const previewRule = (id: number) => http.get<any, string>(`/rules/${id}/preview`, { responseType: 'text' })
