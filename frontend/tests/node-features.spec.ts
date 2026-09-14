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
  it('关闭父功能清空已知子树，保留关闭状态且不修改原对象', () => {
    const params = { uuid: 'keep', smux: { ...smuxValue(), enabled: false } }
    expect(cleanDisabledFeatures([smuxSchema], params)).toEqual({ uuid: 'keep', smux: { enabled: false } })
    expect(params.smux['max-connections']).toBe(7)
    expect(activeFeatures([smuxSchema], params)).toEqual([])
  })
  it('关闭父功能或子功能时清除历史未知子键且不修改原对象', () => {
    const parentClosed = {
      uuid: 'keep',
      smux: {
        ...smuxValue(), enabled: false, future: { value: 'historical' },
        'brutal-opts': { ...smuxValue()['brutal-opts'], future: 'historical-child' },
      },
    }
    expect(cleanDisabledFeatures([smuxSchema], parentClosed)).toEqual({ uuid: 'keep', smux: { enabled: false } })
    expect(parentClosed.smux.future).toEqual({ value: 'historical' })
    expect(parentClosed.smux['brutal-opts'].future).toBe('historical-child')

    const brutalClosed = {
      smux: {
        ...smuxValue(),
        'brutal-opts': { ...smuxValue()['brutal-opts'], enabled: false, future: 'historical-child' },
      },
    }
    const cleaned = cleanDisabledFeatures([smuxSchema], brutalClosed)
    const cleanedSmux = cleaned.smux as Record<string, any>
    expect(cleanedSmux['brutal-opts']).toEqual({ enabled: false })
    expect(cleanedSmux['max-connections']).toBe(7)
    expect(brutalClosed.smux['brutal-opts'].future).toBe('historical-child')
  })

  it('关闭 Brutal 保留父功能和普通开关；启用时保留普通配置', () => {
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
    expect(result.params.smux).toEqual({ enabled: true, 'max-connections': 7, padding: true })
  })
  it('插件切换一次清空通用和四个已知参数对象，不清空 SS 主字段且切回不恢复', () => {
    const resettable = ['plugin-opts', 'obfs-opts', 'v2ray-plugin-opts', 'shadow-tls-opts', 'restls-opts']
    const schema: FieldSchema[] = [
      { name: 'cipher', type: 'text', label: '加密方式', required: true },
      { name: 'password', type: 'password', label: '密码', required: true },
      { name: 'plugin', type: 'select', label: '插件', required: false },
      ...resettable.map((name) => ({ name, type: 'object' as const, label: name, required: false, object_kind: 'map' as const, reset_on: ['plugin'] })),
    ]
    const params = Object.fromEntries(resettable.map((name) => [name, { old: name }]))
    Object.assign(params, { cipher: 'aes-256-gcm', password: 'keep', host: 'keep.example.com', plugin: 'unknown-a' })
    const first = resetProtocolScope(schema, params, 'plugin')
    expect(first.paths).toEqual(resettable)
    expect(first.params).toEqual({ cipher: 'aes-256-gcm', password: 'keep', host: 'keep.example.com', plugin: 'unknown-a' })
    const second = resetProtocolScope(schema, { ...first.params, plugin: 'unknown-b' }, 'plugin')
    expect(second.params).not.toHaveProperty('plugin-opts')
    expect(params['plugin-opts']).toEqual({ old: 'plugin-opts' })
  })
})
