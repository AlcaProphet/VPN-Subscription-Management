// 服务端索引路径与稳定编辑路径的转换、边界和父级定位回归。
import { describe, expect, it } from 'vitest'
import type { FieldSchema } from '@/api/node'
import { resolveValidationLocation, snapshotValidationPaths, validationFocusTarget } from '@/utils/nodeValidation'

const field = (name: string, type: string, extra: Partial<FieldSchema> = {}): FieldSchema => ({ name, type, required: false, label: name, ...extra })
const schema = [
  field('outer', 'object', { object_kind: 'fields', properties: [
    field('rows', 'object', { object_kind: 'list', item_id_field: 'row_id', properties: [
      field('children', 'object', { object_kind: 'list', item_id_field: 'child_id', properties: [field('allowed-ips', 'text-list')] }),
      field('secret', 'password'),
    ] }),
  ] }),
  field('mode', 'select', { state_only: true }),
  field('headers', 'object', { object_kind: 'map' }),
  field('plain', 'object', { object_kind: 'list', properties: [field('value', 'text')] }),
]
const params = { outer: { rows: [{ row_id: 'A', secret: 'synthetic-secret', children: [{ child_id: 'B', 'allowed-ips': [] }] }] }, plain: [{}] }

describe('nodeValidation 路径快照', () => {
  it('递归使用 schema 的身份键，快照不保留取值也不修改输入', () => {
    const data = structuredClone(params)
    const before = structuredClone(data)
    const paths = snapshotValidationPaths(schema, data)
    expect(data).toEqual(before)
    expect(JSON.stringify(paths)).not.toContain('synthetic-secret')
    expect(paths.some((entry) => entry.path === 'mode')).toBe(false)
    data.outer.rows[0].row_id = 'changed'
    expect(resolveValidationLocation('参数错误: 字段 outer.rows[0].children[0].allowed-ips 必填', paths))
      .toEqual({ path: 'outer.rows[A].children[B].allowed-ips', kind: 'field', exact: true })
  })

  it.each([
    ['字段 outer.rows[0].children[0].allowed-ips[2]: 非法', 'outer.rows[A].children[B].allowed-ips', 'field', false],
    ['字段 outer.rows[0].unknown 必填', 'outer.rows[A]', 'item', false],
    ['字段 outer.rows[9].secret 必填', 'outer.rows', 'object', false],
    ['凭据路径 outer.rows[A].secret 重复操作', 'outer.rows[A].secret', 'field', true],
    ['参数错误: 数组条目稳定身份重复: outer.rows[0]', 'outer.rows[A]', 'item', true],
    ['字段 headers.X Name 类型应为 string', 'headers', 'object', false],
    ['字段 plain[0].value 必填', 'plain[0].value', 'field', true],
  ])('解析 %s', (message, path, kind, exact) => {
    expect(resolveValidationLocation(message, snapshotValidationPaths(schema, params))).toEqual({ path, kind, exact })
  })

  it.each(['网络错误 outer.rows', '字段 outer-extra 必填', '字段 outermost 必填', '字段 unknown 必填', '服务不可用，字段 outer.rows 必填'])('非路径前缀不猜测位置：%s', (message) => {
    expect(resolveValidationLocation(message, snapshotValidationPaths(schema, params))).toBeNull()
  })

  it('缺失固定对象仍能定位其必填子字段，未知 Map 不按内容递归', () => {
    const paths = snapshotValidationPaths([field('fixed', 'object', { object_kind: 'fields', properties: [field('required', 'text')] }), ...schema], {})
    expect(resolveValidationLocation('字段 fixed.required 必填', paths)?.path).toBe('fixed.required')
    expect(paths.filter((entry) => entry.path.startsWith('headers'))).toHaveLength(1)
  })
})

describe('nodeValidation 定位目标', () => {
  function fixture(html: string) {
    const root = document.createElement('div')
    root.innerHTML = html
    document.body.append(root)
    return { root, fields: Array.from(root.querySelectorAll<HTMLElement>('[data-field-path]')) }
  }

  it('未知条目子字段只回退到所属 JSON 入口，不选择其它条目输入框', () => {
    const { root, fields } = fixture('<div data-field-path="peers"><button data-field-json-toggle>JSON</button><div data-field-path="peers[A].server"><input></div></div>')
    try {
      const target = validationFocusTarget(fields, { path: 'peers[B]', kind: 'item', exact: false })
      expect(target?.control).toBe(root.querySelector('[data-field-json-toggle]'))
    } finally { root.remove() }
  })

  it('父对象不能借用后代 JSON 文本框，已打开的自身 JSON 文本框优先', () => {
    const { root, fields } = fixture('<div data-field-path="parent"><button data-field-json-toggle>parent</button><div data-field-path="parent.child"><textarea data-field-json-editor></textarea></div></div>')
    try {
      expect(validationFocusTarget(fields, { path: 'parent', kind: 'object', exact: false })?.control).toBe(root.querySelector('button'))
      const textarea = document.createElement('textarea')
      textarea.setAttribute('data-field-json-editor', '')
      fields[0].append(textarea)
      expect(validationFocusTarget(fields, { path: 'parent', kind: 'object', exact: false })?.control).toBe(textarea)
    } finally { root.remove() }
  })

  it('列表整体错误选择自身新增按钮，未知子字段选择 JSON 入口', () => {
    const { root, fields } = fixture('<div data-field-path="peers"><button data-field-json-toggle>JSON</button><button data-field-list-add>新增</button><input></div>')
    try {
      expect(validationFocusTarget(fields, { path: 'peers', kind: 'object', exact: true })?.control).toBe(root.querySelector('[data-field-list-add]'))
      expect(validationFocusTarget(fields, { path: 'peers', kind: 'object', exact: false })?.control).toBe(root.querySelector('[data-field-json-toggle]'))
    } finally { root.remove() }
  })

  it('空标量列表能定位按钮；隐藏、禁用及缺失目标不聚焦', () => {
    const { root, fields } = fixture('<details><div data-field-path="allowed-ips"><button>新增</button></div></details>')
    const location = { path: 'allowed-ips', kind: 'field', exact: true } as const
    try {
      expect(validationFocusTarget(fields, location)?.control).toBe(root.querySelector('button'))
      root.style.display = 'none'
      expect(validationFocusTarget(fields, location)).toBeNull()
      root.style.display = ''
      root.querySelector('button')!.disabled = true
      expect(validationFocusTarget(fields, location)).toBeNull()
      expect(validationFocusTarget(fields, { ...location, path: 'missing' })).toBeNull()
    } finally { root.remove() }
  })
})
