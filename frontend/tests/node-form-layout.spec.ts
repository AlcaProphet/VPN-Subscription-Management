import { describe, expect, it } from 'vitest'
import { collectSwitchFields, matchesCondition, replaceNestedValue } from '@/utils/nodeFormLayout'
import type { FieldSchema } from '@/api/node'
import { smuxSchema, smuxValue } from './fixtures/smux'

describe('节点开关展示投影', () => {
  it('插件补集条件与其它维度保持且关系', () => {
    const rule = { plugin_not: ['', 'obfs', 'v2ray-plugin'], network: ['tcp'] }
    expect(matchesCondition(rule, { plugin: 'custom', network: 'tcp' })).toBe(true)
    expect(matchesCondition(rule, { plugin: 'obfs', network: 'tcp' })).toBe(false)
    expect(matchesCondition(rule, { plugin: 'custom', network: 'ws' })).toBe(false)
    expect(matchesCondition(rule, { plugin: null, network: 'tcp' })).toBe(false)
  })

  it('继承祖先条件和高级分类，关闭父功能后子开关消失', () => {
    const schema: FieldSchema[] = [{ ...smuxSchema, when: { network: ['tcp'] } }]
    expect(collectSwitchFields(schema, { network: 'ws' })).toEqual([])
    const closed = collectSwitchFields(schema, { network: 'tcp', features: [] })
    expect(closed.map((item) => item.path)).toEqual(['smux.enabled'])
    const opened = collectSwitchFields(schema, { network: 'tcp', features: ['smux', 'smux.brutal'] })
    expect(opened.map((item) => item.path)).toEqual(['smux.enabled', 'smux.padding', 'smux.brutal-opts.enabled'])
    expect(opened.every((item) => item.advanced)).toBe(true)
    expect(opened[2].field.label).toContain('多路复用：Brutal 参数：')
    expect(smuxSchema.properties![0].label).toBe('SMux 启用')
  })

  it('嵌套更新不修改原对象、不扁平化路径、不丢失其它键', () => {
    const original = smuxValue()
    const next = replaceNestedValue(original, ['brutal-opts', 'enabled'], false) as typeof original
    expect(next['brutal-opts'].enabled).toBe(false)
    expect(original['brutal-opts'].enabled).toBe(true)
    expect(next['max-connections']).toBe(7)
    expect(next.future).toEqual(original.future)
    expect(next['brutal-opts'].future).toBe('old')
  })

  it('非首批协议的对象数组保留条目内控件，不产生失去索引的开关', () => {
    const schema: FieldSchema[] = [{ name: 'peers', label: 'Peer', type: 'object', object_kind: 'list', required: false,
      properties: [{ name: 'enabled', label: '启用', type: 'bool', required: false }] }]
    expect(collectSwitchFields(schema, {})).toEqual([])
  })
})
