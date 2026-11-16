import type { CurrentState, FieldSchema } from '@/api/node'
import { matchesSensitivePath } from './nodeFeatures'
import { isTriStateBool, matchesCondition } from './nodeFormLayout'

type Params = Record<string, unknown>

function isObject(value: unknown): value is Params {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function clone(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(clone)
  if (isObject(value)) return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clone(item)]))
  return value
}

// 保存与检查共用：只把普通数字／三态 bool 的明确清空编码为 null，不修改内存草稿。
// 基准用于识别局部 JSON 删除的已知标量；解析器的部分结果必须先应用到完整草稿。
export function buildProtocolPayload(
  schema: FieldSchema[],
  params: Params,
  baseline: Params,
  state: CurrentState,
  sensitivePaths: string[],
): Params {
  const out = clone(params) as Params
  function walk(fields: FieldSchema[], current: Params, previous: Params, prefix: string) {
    for (const field of fields) {
      const path = prefix ? `${prefix}.${field.name}` : field.name
      if (field.state_only || sensitivePaths.some((pattern) => matchesSensitivePath(pattern, path))
          || !matchesCondition(field.when, state, undefined, params)) continue
      const value = current[field.name]
      if ((field.type === 'number' || isTriStateBool(field)) && value === undefined
          && (Object.prototype.hasOwnProperty.call(current, field.name) || Object.prototype.hasOwnProperty.call(previous, field.name))) {
        current[field.name] = null
      } else if (field.type === 'object' && field.object_kind === 'fields' && isObject(value) && Object.keys(value).length > 0) {
        // 显式空对象已有整体清空语义，不能补标记后使它退回局部合并并恢复邻接字段。
        const old = previous[field.name]
        walk(field.properties ?? [], value, isObject(old) ? old : {}, path)
      } else if (field.type === 'object' && field.object_kind === 'list' && Array.isArray(value)) {
        // 对象列表整体替换，普通参数不从旧条目补值；凭据继续由稳定身份与 credential_ops 处理。
        for (const item of value) if (isObject(item)) walk(field.properties ?? [], item, {}, `${path}[]`)
      }
    }
  }
  walk(schema, out, baseline, '')
  return out
}
