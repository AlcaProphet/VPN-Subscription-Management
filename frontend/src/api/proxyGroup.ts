// api/proxyGroup.ts：代理组管理接口（Design2-UI §9.1）
import { http } from './request'

export interface ProxyGroupDefinition {
  type: 'select' | 'url-test' | 'fallback' | 'load-balance' | 'relay'
  groups: string[]
  use?: string[]
  url?: string
  'expected-status'?: string
  interval?: number
  timeout?: number
  'max-failed-times'?: number
  lazy?: boolean
  'disable-udp'?: boolean
  'interface-name'?: string
  'routing-mark'?: number
  filter?: string
  'exclude-filter'?: string
  'exclude-type'?: string
  'include-all'?: boolean
  'include-all-proxies'?: boolean
  'include-all-providers'?: boolean
  hidden?: boolean
  icon?: string
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
  group_type: ProxyGroupDefinition['type']
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
