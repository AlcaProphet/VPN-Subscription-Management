// api/rulespec.ts：中央能力注册表只读元数据。
import { http } from './request'

export interface CapabilityMetadata {
  family: string
  matcher: string
  scope: 'common' | 'clash_only' | 'sr_only' | 'unsupported'
  clash_render_type?: string
  sr_render_type?: string
  supports_no_resolve: boolean
  material_pool: boolean
  advanced: boolean
}

export const listCapabilityMeta = () =>
  http.get<any, CapabilityMetadata[]>('/admin/rulespec/meta')
