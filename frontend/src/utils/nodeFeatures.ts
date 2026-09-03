// nodeFeatures.ts：消费服务端声明的功能归属，不在前端维护协议字段清单。
import type { FieldSchema } from '@/api/node'

type Params = Record<string, unknown>

function isObject(value: unknown): value is Params {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function clone(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(clone)
  if (isObject(value)) return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clone(item)]))
  return value
}

export function pathContains(parent: string, path: string): boolean {
  return path === parent || path.startsWith(`${parent}.`) || path.startsWith(`${parent}[`)
}

function pathPattern(path: string): string {
  return path.replace(/\[[^\]]*\]/g, '[]')
}

export function matchesSensitivePath(pattern: string, path: string): boolean {
  return pathPattern(path) === pattern
}

export function concreteSensitivePaths(params: Params, patterns: string[]): string[] {
  const paths: string[] = []
  function expand(value: unknown, segments: string[], prefix: string) {
    if (segments.length === 0) {
      paths.push(prefix)
      return
    }
    if (!isObject(value)) return
    const [segment, ...rest] = segments
    const list = segment.endsWith('[]')
    const name = list ? segment.slice(0, -2) : segment
    const child = value[name]
    const nextPrefix = prefix ? `${prefix}.${name}` : name
    if (!list) {
      if (name in value) expand(child, rest, nextPrefix)
      return
    }
    if (!Array.isArray(child)) return
    for (const item of child) {
      if (!isObject(item)) continue
      const id = typeof item._credential_id === 'string' ? item._credential_id : ''
      if (id) expand(item, rest, `${nextPrefix}[${id}]`)
    }
  }
  for (const pattern of patterns) expand(params, pattern.split('.'), '')
  return [...new Set(paths)].sort()
}

export function featureEnabled(field: FieldSchema, value: unknown): boolean {
  const feature = field.feature
  if (!feature) return false
  if (feature.toggle) value = isObject(value) ? value[feature.toggle] : undefined
  if (typeof value === 'boolean') return value
  return typeof value === 'string' && !!feature.disabled_value && value !== '' && value !== feature.disabled_value
}

export function activeFeatures(schema: FieldSchema[], params: Params): string[] {
  const features: string[] = []
  for (const field of schema) {
    const value = params[field.name]
    if (field.feature) {
      if (!featureEnabled(field, value)) continue
      features.push(field.feature.name)
    }
    if (isObject(value)) features.push(...activeFeatures(field.properties ?? [], value))
  }
  return [...new Set(features)].sort()
}

export function cleanDisabledFeatures(schema: FieldSchema[], params: Params, onClear?: (scope: string) => void): Params {
  const next = clone(params) as Params
  const active = activeFeatures(schema, params)
  function clean(fields: FieldSchema[], object: Params) {
    for (const field of fields) {
      const value = object[field.name]
      if (field.feature?.toggle && isObject(value) && !featureEnabled(field, value)) {
        const toggle = field.feature.toggle
        if (!(toggle in value)) {
          delete object[field.name]
          onClear?.(`feature.${field.feature.name}`)
          continue
        }
        if (value[toggle] === false) {
          object[field.name] = { [toggle]: false }
          if (Object.keys(value).some((key) => key !== toggle)) onClear?.(`feature.${field.feature.name}`)
          continue
        }
      }
      if (!field.feature && field.reset_on?.some((scope) => scope.startsWith('feature.') && !active.includes(scope.slice(8)))) {
        if (field.name in object) {
          for (const scope of field.reset_on) {
            if (scope.startsWith('feature.') && !active.includes(scope.slice(8))) onClear?.(scope)
          }
        }
        delete object[field.name]
        continue
      }
      if (isObject(value)) clean(field.properties ?? [], value)
    }
  }
  clean(schema, next)
  return next
}

export function resetProtocolScope(schema: FieldSchema[], params: Params, scope: string): { params: Params; paths: string[] } {
  const next = clone(params) as Params
  const paths: string[] = []
  function clear(fields: FieldSchema[], object: Params, prefix: string) {
    for (const field of fields) {
      const path = prefix ? `${prefix}.${field.name}` : field.name
      if (scope === 'protocol' || field.reset_on?.includes(scope)) {
        paths.push(path)
        delete object[field.name]
      } else {
        // 即使当前对象为空，仍收集需失效的凭据和草稿路径。
        const value = object[field.name]
        clear(field.properties ?? [], isObject(value) ? value : {}, path)
      }
    }
  }
  clear(schema, next, '')
  return { params: next, paths }
}

export function valueAtPath(params: Params, path: string): unknown {
  let value: unknown = params
  for (const part of path.split('.')) {
    const match = part.match(/^([^[]+)\[([^\]]+)\]$/)
    if (!match) {
      value = isObject(value) ? value[part] : undefined
      continue
    }
    if (!isObject(value)) return undefined
    const list = value[match[1]]
    if (!Array.isArray(list)) return undefined
    value = list.find((item) => isObject(item) && item._credential_id === match[2])
  }
  return value
}
