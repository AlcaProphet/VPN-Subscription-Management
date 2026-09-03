import { describe, expect, it } from 'vitest'
import { activeFeatures, cleanDisabledFeatures, resetProtocolScope } from '@/utils/nodeFeatures'
import { smuxSchema, smuxValue } from './fixtures/smux'
import type { FieldSchema } from '@/api/node'

describe('功能元数据驱动的草稿清理', () => {
  it.each(['udp-over-tcp', 'udp-over-stream'])('保留 %s 顶层关闭行为，不影响普通 UDP 开关', (name) => {
    const schema: FieldSchema[] = [
      { name, type: 'bool', label: name, required: false, feature: { name }, reset_on: [`feature.${name}`] },
      { name: `${name}-version`, type: 'number', label: '版本', required: false, reset_on: [`feature.${name}`] },
    ]
    expect(cleanDisabledFeatures(schema, { [name]: false, [`${name}-version`]: 2, udp: false })).toEqual({ [name]: false, udp: false })
    expect(activeFeatures(schema, { [name]: true })).toEqual([name])
  })
  it('关闭父功能清空已知和未知子树，保留关闭状态且不修改原对象', () => {
    const params = { uuid: 'keep', smux: { ...smuxValue(), enabled: false } }
    expect(cleanDisabledFeatures([smuxSchema], params)).toEqual({ uuid: 'keep', smux: { enabled: false } })
    expect(params.smux['max-connections']).toBe(7)
    expect(activeFeatures([smuxSchema], params)).toEqual([])
  })
  it('关闭 Brutal 保留父功能和普通开关；启用时保留未知键', () => {
    const params = { smux: smuxValue() }
    expect(activeFeatures([smuxSchema], params)).toEqual(['smux', 'smux.brutal'])
    expect(cleanDisabledFeatures([smuxSchema], params)).toEqual(params)
    params.smux['brutal-opts'].enabled = false
    const result = cleanDisabledFeatures([smuxSchema], params).smux
    expect(result).toEqual({ ...params.smux, 'brutal-opts': { enabled: false } })
    params.smux.padding = false
    expect((cleanDisabledFeatures([smuxSchema], params).smux as Record<string, unknown>)['max-connections']).toBe(7)
  })
  it('递归 reset_on 精确删除 Brutal，不清空 SMux 的其他字段', () => {
    const result = resetProtocolScope([smuxSchema], { smux: smuxValue() }, 'feature.smux.brutal')
    expect(result.paths).toEqual(['smux.brutal-opts'])
    expect(result.params.smux).toEqual({ enabled: true, 'max-connections': 7, padding: true, future: { value: 'old' } })
  })
})
