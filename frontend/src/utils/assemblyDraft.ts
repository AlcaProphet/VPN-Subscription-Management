// assemblyDraft.ts：装配跨页草稿的 sessionStorage 协议（Build11 Step 2）
import type { OverlayInput, PoolSelection, RuleLine, TargetSyntax } from '@/api/assembly'

export const ASSEMBLY_CONTEXT_KEY = 'assembly_ctx_v1'
export const ASSEMBLY_CONTEXT_TTL_MS = 30 * 60 * 1000
const targetSyntaxes: TargetSyntax[] = ['clash-yaml', 'sr-subs', 'generic-subs', 'sr-conf']

export interface AssemblyDraftForm {
  platform_id?: number
  rule_id?: number
  rule_name: string
  sr_rule_mode: 'existing' | 'new'
  fixed_params_text: string
  node_names: string[]
  group_names: string[]
  group_node_orders: Record<string, string[]>
  overseas_members: string[]
  fallback_group_members: string[]
  pools: PoolSelection[]
  custom_rules: RuleLine[]
  final_direction: string
  overlay: Required<OverlayInput>
}

export interface AssemblyDraft {
  version: 1
  createdAt: number
  expiresAt: number
  sourceLabel: string
  returnPath: string
  mainTab: string
  subTab: TargetSyntax
  currentStep: number
  layoutMode: 'step' | 'page'
  form: AssemblyDraftForm
}

function isValidDraft(value: unknown): value is AssemblyDraft {
  if (!value || typeof value !== 'object') return false
  const draft = value as Partial<AssemblyDraft>
  const form = draft.form as Partial<AssemblyDraftForm> | undefined
  return draft.version === 1
    && typeof draft.expiresAt === 'number'
    && typeof draft.sourceLabel === 'string'
    && typeof draft.returnPath === 'string'
    && typeof draft.mainTab === 'string'
    && targetSyntaxes.includes(draft.subTab as TargetSyntax)
    && typeof draft.currentStep === 'number'
    && (draft.layoutMode === 'step' || draft.layoutMode === 'page')
    && !!form
    && typeof form.rule_name === 'string'
    && (form.sr_rule_mode === 'existing' || form.sr_rule_mode === 'new')
    && typeof form.fixed_params_text === 'string'
    && Array.isArray(form.node_names)
    && Array.isArray(form.group_names)
    && !!form.group_node_orders
    && Array.isArray(form.overseas_members)
    && Array.isArray(form.fallback_group_members)
    && Array.isArray(form.pools)
    && Array.isArray(form.custom_rules)
    && typeof form.final_direction === 'string'
    && !!form.overlay
}

// readAssemblyDraft 同时完成过期和损坏数据清理，避免 ContextBar 展示无法恢复的旧任务。
export function readAssemblyDraft(): AssemblyDraft | null {
  const raw = sessionStorage.getItem(ASSEMBLY_CONTEXT_KEY)
  if (!raw) return null
  try {
    const draft: unknown = JSON.parse(raw)
    if (!isValidDraft(draft) || draft.expiresAt <= Date.now()) {
      sessionStorage.removeItem(ASSEMBLY_CONTEXT_KEY)
      return null
    }
    return draft
  } catch {
    sessionStorage.removeItem(ASSEMBLY_CONTEXT_KEY)
    return null
  }
}

export function saveAssemblyDraft(draft: Omit<AssemblyDraft, 'version' | 'createdAt' | 'expiresAt'>) {
  const now = Date.now()
  const value: AssemblyDraft = {
    ...draft,
    version: 1,
    createdAt: now,
    expiresAt: now + ASSEMBLY_CONTEXT_TTL_MS,
  }
  sessionStorage.setItem(ASSEMBLY_CONTEXT_KEY, JSON.stringify(value))
}

export function clearAssemblyDraft() {
  sessionStorage.removeItem(ASSEMBLY_CONTEXT_KEY)
}
