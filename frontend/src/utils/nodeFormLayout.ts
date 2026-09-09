// 节点表单的展示投影：只消费 schema，不维护协议字段全集或另一份协议数据。
import type { ConditionRule, CurrentState, FieldSchema } from '@/api/node'

export function matchesCondition(rule: ConditionRule | undefined, state?: CurrentState, target?: string): boolean {
  if (!rule || !state) return true
  if (rule.network?.length && !rule.network.includes(state.network ?? '')) return false
  if (rule.security?.length && !rule.security.includes(state.security ?? '')) return false
  if (rule.plugin?.length && !rule.plugin.includes(state.plugin ?? '')) return false
  if (rule.plugin_not?.length && rule.plugin_not.includes(state.plugin ?? '')) return false
  if (rule.features?.length && !rule.features.some((item) => state.features?.includes(item))) return false
  return !(target && rule.targets?.length && !rule.targets.includes(target))
}

export function fieldGroup(field: FieldSchema): string {
  if (field.group) return field.group
  if (field.section === 'transport' || field.section === 'security') return 'connection'
  return field.section ?? 'advanced'
}

export interface SwitchField { path: string; field: FieldSchema; advanced: boolean }

export function collectSwitchFields(fields: FieldSchema[], state: CurrentState, prefix = '', labels: string[] = [], advanced = false): SwitchField[] {
  return fields.flatMap((field) => {
    // 祖先不活动时不遍历；功能启用控件本身不依赖自身已启用。
    if (!matchesCondition(field.when, state)) return []
    const path = prefix ? `${prefix}.${field.name}` : field.name
    const isAdvanced = advanced || !!field.advanced || fieldGroup(field) === 'advanced'
    if (field.type === 'bool') return [{ path, advanced: isAdvanced, field: { ...field, label: [...labels, field.label].join('：') } }]
    // 对象数组保留条目内的编辑与开关，避免动态索引脱离所属条目；首批四协议没有此类运行开关。
    return field.type === 'object' && field.object_kind === 'fields'
      ? collectSwitchFields(field.properties ?? [], state, path, [...labels, field.label], isAdvanced)
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
