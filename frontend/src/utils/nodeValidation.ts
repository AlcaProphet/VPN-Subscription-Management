// 保存校验使用提交时的索引与稳定身份，不读取响应到达时的列表顺序，也不保存字段值。
import type { FieldSchema } from '@/api/node'
import { pathContains } from './nodeFeatures'

interface ValidationPath {
  requestPath: string
  path: string
  kind: 'field' | 'object' | 'item'
}
export interface ValidationLocation {
  path: string
  kind: ValidationPath['kind']
  exact: boolean
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export function snapshotValidationPaths(schema: FieldSchema[], params: Record<string, unknown>): ValidationPath[] {
  const paths: ValidationPath[] = []
  function walk(fields: FieldSchema[], data: Record<string, unknown>, requestPrefix: string, prefix: string) {
    for (const field of fields) {
      if (field.state_only) continue
      const requestPath = requestPrefix ? `${requestPrefix}.${field.name}` : field.name
      const path = prefix ? `${prefix}.${field.name}` : field.name
      paths.push({ requestPath, path, kind: field.type === 'object' ? 'object' : 'field' })
      const value = data[field.name]
      if (field.type !== 'object') continue
      if (field.object_kind === 'fields') {
        walk(field.properties ?? [], isObject(value) ? value : {}, requestPath, path)
      } else if (field.object_kind === 'list' && Array.isArray(value)) {
        value.forEach((item, index) => {
          const id = isObject(item) && field.item_id_field ? item[field.item_id_field] : undefined
          const itemPath = `${path}[${typeof id === 'string' && id ? id : index}]`
          const requestItemPath = `${requestPath}[${index}]`
          paths.push({ requestPath: requestItemPath, path: itemPath, kind: 'item' })
          walk(field.properties ?? [], isObject(item) ? item : {}, requestItemPath, itemPath)
        })
      }
    }
  }
  walk(schema, params, '', '')
  return paths
}

export function resolveValidationLocation(message: string, paths: readonly ValidationPath[]): ValidationLocation | null {
  const body = message.replace(/^参数错误:\s*/, '')
    .match(/^(?:字段 |凭据路径 |数组条目稳定身份重复:\s*)(.*)$/)?.[1]
  if (body === undefined) return null
  // 只匹配已知错误格式中的路径前缀；避免嵌套字段或错误说明中的同名字段误命中顶层。
  const candidates = paths.flatMap((entry) => [entry.requestPath, entry.path].map((alias) => ({ entry, alias })))
    .sort((left, right) => right.alias.length - left.alias.length)
  for (const { entry, alias } of candidates) {
    if (!body.startsWith(alias)) continue
    const suffix = body.slice(alias.length)
    if (suffix && !/^(?:[\s:.]|\[)/.test(suffix)) continue
    return { path: entry.path, kind: entry.kind, exact: !suffix || /^[\s:]/.test(suffix) }
  }
  return null
}

// 父级回退仅进入所属 JSON；禁止通过父列表随意聚焦另一个条目的输入框。
export function validationFocusTarget(fields: readonly HTMLElement[], location: ValidationLocation): { field: HTMLElement; control: HTMLElement } | null {
  const field = fields.find((element) => element.dataset.fieldPath === location.path)
    ?? fields.filter((element) => pathContains(element.dataset.fieldPath ?? '', location.path))
      .sort((left, right) => (right.dataset.fieldPath?.length ?? 0) - (left.dataset.fieldPath?.length ?? 0))[0]
  if (!field) return null
  function ownControl(selector: string): HTMLElement | undefined {
    return Array.from(field!.querySelectorAll<HTMLElement>(selector))
      .find((element) => element.closest('[data-field-path]') === field && !element.matches(':disabled, [aria-disabled="true"]'))
  }
  const exactField = field.dataset.fieldPath === location.path
  const control = location.kind === 'field' && exactField
    ? ownControl('textarea:not(:disabled), input:not(:disabled):not([type="hidden"])')
      ?? ownControl('button:not(:disabled), [tabindex]:not([tabindex="-1"])')
    : ownControl('[data-field-json-editor]')
      ?? (exactField && location.exact ? ownControl('[data-field-list-add]') : undefined)
      ?? ownControl('[data-field-json-toggle]')
  if (!control) return null
  for (let element: HTMLElement | null = control; element; element = element.parentElement) {
    const style = getComputedStyle(element)
    if (element.hidden || element.hasAttribute('inert') || element.getAttribute('aria-hidden') === 'true'
      || style.display === 'none' || style.visibility === 'hidden') return null
  }
  return { field, control }
}
