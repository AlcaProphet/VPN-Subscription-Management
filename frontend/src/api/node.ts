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
  edit_revision: number
  state_format_version: number
  current_state: CurrentState
  extensions: ExtensionSummary[]
}

export interface ConditionRule {
  network?: string[]
  security?: string[]
  plugin?: string[]
  features?: string[]
  targets?: string[]
}

export interface OptionItem {
  value: string
  label?: string
  group?: string
  verified?: string
}

export interface TargetEvidence {
  target: string
  client?: string
  version?: string
  entry?: string
  status: 'complete' | 'equivalent' | 'partial' | 'unsupported' | 'unverified'
}

export interface FieldSchema {
  name: string
  type: string
  required: boolean
  default?: unknown
  label: string
  help?: string
  options?: string[]
  section?: 'auth' | 'transport' | 'security' | 'switches' | 'advanced'
  group?: 'basic' | 'auth' | 'connection' | 'switches' | 'advanced'
  advanced?: boolean
  object_kind?: 'fields' | 'map' | 'list'
  properties?: FieldSchema[]
  allow_unknown?: boolean
  when?: ConditionRule
  required_when?: ConditionRule
  reset_on?: string[]
  option_items?: OptionItem[]
  allow_custom?: boolean
  canonical_path?: string
  aliases?: string[]
  target_evidence?: TargetEvidence[]
}

export interface CurrentState {
  network?: string
  security?: string
  plugin?: string | null
  features?: string[]
}

export interface CredentialOp {
  path: string
  op: 'keep' | 'clear'
}

export interface ExtensionInput {
  id?: string
  scope: string
  targets: string[]
  label: string
  payload: string
}

export interface ExtensionOp {
  op: 'keep' | 'replace' | 'clear' | 'add'
  id?: string
  scope?: string
  targets?: string[]
  label?: string
  payload?: string
}

export interface ExtensionSummary {
  id: string
  scope: string
  targets?: string[]
  label?: string
  configured: boolean
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
  current_state?: CurrentState
  base_revision?: number
  reset_scopes?: string[]
  credential_ops?: CredentialOp[]
  extension_ops?: ExtensionOp[]
  extensions?: ExtensionInput[]
}

export interface NodeCheckRequest {
  node_id?: number
  base_revision?: number
  protocol: string
  host: string
  port: number
  protocol_json: Record<string, unknown>
  current_state?: CurrentState
  reset_scopes?: string[]
  credential_ops?: CredentialOp[]
  extension_ops?: ExtensionOp[]
  extensions?: ExtensionInput[]
  targets?: string[]
}

export interface TargetDiagnostic {
  severity: string
  code: string
  target?: string
  field_path?: string
  message: string
  evidence?: string
}

export interface TargetCheckResult {
  status: string
  preview?: string | null
  diagnostics: TargetDiagnostic[]
}

export interface NodeCheckResponse {
  check_id: string
  check_version: number
  targets: Record<string, TargetCheckResult>
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
export const checkNode = (data: NodeCheckRequest) =>
  http.post<any, NodeCheckResponse>('/admin/nodes/check', data)
export const updateNode = (id: number, data: NodeForm) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}`, data)
export const deleteNode = (id: number) =>
  http.delete(`/admin/nodes/${id}`)
export const toggleNode = (id: number, data: { enabled?: boolean; is_public?: boolean }) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}/toggle`, data)
export const setNodeDisplayName = (id: number, display_name: string) =>
  http.put<any, NodeItem>(`/admin/nodes/${id}/display-name`, { display_name })
