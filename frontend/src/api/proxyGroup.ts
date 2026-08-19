// api/proxyGroup.ts：代理组管理接口（Design2-UI §9.1）
import { http } from './request'

export interface ProxyGroupDefinition {
  type: 'select' | 'url-test' | 'fallback'
  nodes: string[]
  groups: string[]
}

export interface ProxyGroupItem {
  id: number
  name: string
  type: 'preset' | 'custom'
  preset_key?: string
  enabled: boolean
  definition: ProxyGroupDefinition
}

export interface ProxyGroupForm {
  name: string
  group_type: 'select' | 'url-test' | 'fallback'
  definition: ProxyGroupDefinition
}

export const listProxyGroups = () =>
  http.get<any, { list: ProxyGroupItem[]; total: number }>('/admin/proxy-groups').then((d) => d.list)
export const createProxyGroup = (data: ProxyGroupForm) =>
  http.post<any, ProxyGroupItem>('/admin/proxy-groups', data)
export const updateProxyGroup = (id: number, data: { group_type: ProxyGroupDefinition['type']; definition: ProxyGroupDefinition }) =>
  http.put<any, ProxyGroupItem>(`/admin/proxy-groups/${id}`, data)
export const deleteProxyGroup = (id: number) =>
  http.delete(`/admin/proxy-groups/${id}`)
export const togglePresetGroup = (id: number, enabled: boolean) =>
  http.put<any, ProxyGroupItem>(`/admin/proxy-groups/${id}/preset-toggle`, { enabled })
