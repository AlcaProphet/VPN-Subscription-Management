// 节点表单的展示投影：只消费 schema，不维护协议字段全集或另一份协议数据。
import type { ConditionRule, CurrentState, EndpointPolicy, FieldSchema } from '@/api/node'

export const DEFAULT_ENDPOINT_POLICY: EndpointPolicy = {
  host_mode: 'required', port_mode: 'required', emit_host: true, emit_port: true,
}

export type Params = Record<string, unknown>

export function endpointPolicyFor(policies: EndpointPolicy[] | undefined, state?: CurrentState, params?: Params): EndpointPolicy | null {
  if (!policies?.length) return DEFAULT_ENDPOINT_POLICY
  const matched = policies.filter((policy) => matchesCondition(policy.when, state, undefined, params))
  return matched.length === 1 ? matched[0] : null
}

// valueAtDotPath 读取规范点路径；与后端 GetPath 的顶层/嵌套路径语义一致。
function valueAtDotPath(params: Params | undefined, path: string): unknown {
  if (!params) return undefined
  let current: unknown = params
  for (const part of path.split('.')) {
    if (!current || typeof current !== 'object' || Array.isArray(current)) return undefined
    current = (current as Params)[part]
  }
  return current
}

function isNonEmptyValue(value: unknown): boolean {
  if (value === undefined || value === null) return false
  if (typeof value === 'string') return value.trim() !== ''
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === 'object') return Object.keys(value as Params).length > 0
  return true
}

export function matchesCondition(rule: ConditionRule | undefined, state?: CurrentState, target?: string, params?: Params): boolean {
  if (!rule || !state) return true
  if (rule.network?.length && !rule.network.includes(state.network ?? '')) return false
  if (rule.security?.length && !rule.security.includes(state.security ?? '')) return false
  if (rule.plugin?.length && !rule.plugin.includes(state.plugin ?? '')) return false
  if (rule.plugin_not?.length && rule.plugin_not.includes(state.plugin ?? '')) return false
  if (rule.features?.length && !rule.features.some((item) => state.features?.includes(item))) return false
  if (rule.selectors) {
    for (const [name, allowed] of Object.entries(rule.selectors)) {
      if (!allowed.length) continue
      if (!allowed.includes(state.selectors?.[name] ?? '')) return false
    }
  }
  // non_empty：依赖的兄弟字段必须存在有效值；缺少 params 时按不匹配处理，避免误显示。
  if (rule.non_empty?.length) {
    for (const path of rule.non_empty) {
      if (!isNonEmptyValue(valueAtDotPath(params, path))) return false
    }
  }
  return !(target && rule.targets?.length && !rule.targets.includes(target))
}

// isTriStateBool 判定未声明 default 的 bool 字段：保留 unset／false／true 三态，不用 false 吞掉“未设置”。
export function isTriStateBool(field: FieldSchema): boolean {
  return field.type === 'bool' && field.default === undefined
}

export function fieldGroup(field: FieldSchema): string {
  if (field.group) return field.group
  if (field.section === 'transport' || field.section === 'security') return 'connection'
  return field.section ?? 'advanced'
}

export interface SwitchField { path: string; field: FieldSchema; advanced: boolean }

export function collectSwitchFields(fields: FieldSchema[], state: CurrentState, params?: Params, prefix = '', labels: string[] = [], advanced = false): SwitchField[] {
  return fields.flatMap((field) => {
    // 祖先不活动时不遍历；功能启用控件本身不依赖自身已启用。
    if (!matchesCondition(field.when, state, undefined, params)) return []
    const path = prefix ? `${prefix}.${field.name}` : field.name
    const isAdvanced = advanced || !!field.advanced || fieldGroup(field) === 'advanced'
    // 三态 bool 不能用集中开关区的普通 Switch 表达，改在所属分组内用三态控件渲染。
    if (field.type === 'bool') {
      return isTriStateBool(field) ? [] : [{ path, advanced: isAdvanced, field: { ...field, label: [...labels, field.label].join('：') } }]
    }
    // 对象数组保留条目内的编辑与开关，避免动态索引脱离所属条目；首批四协议没有此类运行开关。
    return field.type === 'object' && field.object_kind === 'fields'
      ? collectSwitchFields(field.properties ?? [], state, params, path, [...labels, field.label], isAdvanced)
      : []
  })
}

export function hasConfiguredValue(value: unknown): boolean {
  if (value === undefined || value === null || value === '') return false
  if (Array.isArray(value)) return value.some(hasConfiguredValue)
  if (typeof value === 'object') return Object.values(value).some(hasConfiguredValue)
  return true // 显式 false/0 同样属于已配置值。
}

// 路径来自服务端固定对象 schema；仍以顶层对象为更新单位进入原 setField 清理链。
export function replaceNestedValue(value: unknown, segments: string[], nextValue: unknown): unknown {
  if (segments.length === 0) return nextValue
  const object = value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
  const [key, ...rest] = segments
  return { ...object, [key]: replaceNestedValue(object[key], rest, nextValue) }
}
