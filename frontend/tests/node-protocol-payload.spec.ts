import { describe, expect, it } from 'vitest'
import type { FieldSchema } from '@/api/node'
import { buildProtocolPayload } from '@/utils/nodeProtocolPayload'

const number: FieldSchema = { name: 'number', type: 'number', label: '数字', required: false }
const tri: FieldSchema = { name: 'tri', type: 'bool', label: '三态', required: false }
const object: FieldSchema = { name: 'opts', type: 'object', object_kind: 'fields', label: '对象', required: false,
  properties: [number, { name: 'text', type: 'text', label: '文本', required: false }] }

describe('协议请求的普通标量清空表达', () => {
  it('缺失且从未配置不生成默认值，明确清空发送 null，0 和 false 保留', () => {
    expect(buildProtocolPayload([number, tri], {}, {}, {}, [])).toEqual({})
    expect(buildProtocolPayload([number, tri], { number: undefined, tri: undefined }, {}, {}, [])).toEqual({ number: null, tri: null })
    expect(buildProtocolPayload([number, tri], {}, { number: 0, tri: false }, {}, [])).toEqual({ number: null, tri: null })
    expect(buildProtocolPayload([number, tri], { number: 0, tri: false }, { number: 4, tri: true }, {}, [])).toEqual({ number: 0, tri: false })
  })

  it('递归固定对象支持 JSON 删除数字，不改变原草稿或基准', () => {
    const draft = { opts: { text: 'keep' } }
    const baseline = { opts: { number: 4, text: 'old' } }
    const result = buildProtocolPayload([object], draft, baseline, {}, [])
    expect(result).toEqual({ opts: { number: null, text: 'keep' } })
    expect(draft).toEqual({ opts: { text: 'keep' } })
    expect(baseline).toEqual({ opts: { number: 4, text: 'old' } })
    expect(result.opts).not.toBe(draft.opts)
  })

  it('显式空对象保持整体清空，未提交的父对象不被重新建立', () => {
    const baseline = { opts: { number: 4, text: 'old' } }
    expect(buildProtocolPayload([object], { opts: {} }, baseline, {}, [])).toEqual({ opts: {} })
    expect(buildProtocolPayload([object], {}, baseline, {}, [])).toEqual({})
  })

  it('隐藏字段、state_only、凭据和二态开关不生成清空标记', () => {
    const fields: FieldSchema[] = [
      { ...number, name: 'hidden', when: { network: ['ws'] } },
      { ...number, name: 'selector', state_only: true },
      { ...number, name: 'secret' },
      { ...tri, name: 'switch', default: false },
      { name: 'password', type: 'password', label: '凭据', required: false },
    ]
    const baseline = { hidden: 1, selector: 1, secret: 1, switch: true, password: 'saved' }
    expect(buildProtocolPayload(fields, { password: '' }, baseline, { network: 'tcp' }, ['secret', 'password'])).toEqual({ password: '' })
  })

  it('开放 Map 和未知键原样保留，不根据键名或值猜测清空与凭据', () => {
    const map: FieldSchema = { ...object, name: 'map', object_kind: 'map', allow_unknown: true }
    const params = { map: { number: undefined, literal: null, token: 'ordinary-value' }, unknown: { number: undefined } }
    expect(buildProtocolPayload([map], params, { map: { number: 3 }, unknown: { number: 4 } }, {}, [])).toEqual(params)
  })

  it('对象列表整体替换不按索引恢复旧条目；敏感通配路径不转为清空', () => {
    const list: FieldSchema = { ...object, name: 'items', object_kind: 'list', properties: [number, { ...number, name: 'secret' }] }
    const draft = { items: [{ number: undefined, secret: undefined }, {}] }
    expect(buildProtocolPayload([list], draft, { items: [{ number: 1 }, { number: 2 }] }, {}, ['items[].secret']))
      .toEqual({ items: [{ number: null, secret: undefined }, {}] })
    expect(draft.items[0].number).toBeUndefined()
  })
})
