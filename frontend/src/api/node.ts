// api/node.ts：节点管理接口（Design2-UI §9.1）
import { http } from './request'

export interface NodeItem {
  id: number
  source: 'manual' | 'xray'
  name: string
  display_name?: string | null
  render_name: string
  instance_id?: number | null
  instance_slug?: string
  tag?: string
  protocol: string
  host: string
  port: number
  protocol_json: Record<string, unknown>
  is_public: boolean
  enabled: boolean
  allocatable: boolean
  missing: boolean
}

export interface FieldSchema {
  name: string
  type: string
  required: boolean
  default?: unknown
  label: string
  help?: string
  options?: string[]
}

export interface LinkMapping {
  sr: boolean
  generic: boolean
  params?: string[]
}

export interface ProtocolInfo {
  protocol: string
  label: string
  form_schema: FieldSchema[]
  sensitive_fields: string[]
  link_mappings: LinkMapping
}

export interface NodeForm {
  name: string
  protocol: string
  host: string
  port: number
  protocol_json: Record<string, unknown>
}

export interface ImportLineResult {
  line: number
  raw: string
  ok: boolean
  name?: string
  reason?: string
}

export const listNodes = (source?: 'manual' | 'xray') =>
  http.get<any, { list: NodeItem[]; total: number }>('/admin/nodes', { params: source ? { source } : {} }).then((d) => d.list)
export const getProtocols = () =>
  http.get<any, { list: ProtocolInfo[] }>('/admin/nodes/protocols').then((d) => d.list)
export const createNode = (data: NodeForm) =>
  http.post<any, NodeItem>('/admin/nodes', data)
export const importNodes = (text: string) =>
  http.post<any, { list: ImportLineResult[]; total: number }>('/admin/nodes/import', { text })
export const updateNode = (id: number, data: NodeForm) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}`, data)
export const deleteNode = (id: number) =>
  http.delete(`/admin/nodes/${id}`)
export const toggleNode = (id: number, data: { enabled?: boolean; is_public?: boolean }) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}/toggle`, data)
export const setNodeDisplayName = (id: number, display_name: string) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}/display-name`, { display_name })
