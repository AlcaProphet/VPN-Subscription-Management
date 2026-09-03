// 与服务端功能元数据契约一致的 SMux/Brutal 测试样例。
import type { FieldSchema } from '@/api/node'

export const smuxSchema: FieldSchema = {
  name: 'smux', label: '多路复用', type: 'object', required: false, object_kind: 'fields', allow_unknown: true,
  group: 'advanced', feature: { name: 'smux', toggle: 'enabled' }, reset_on: ['feature.smux'],
  properties: [
    { name: 'enabled', label: 'SMux 启用', type: 'bool', required: false, default: false },
    { name: 'max-connections', label: '最大连接数', type: 'number', required: false, when: { features: ['smux'] } },
    { name: 'padding', label: '填充', type: 'bool', required: false, when: { features: ['smux'] } },
    {
      name: 'brutal-opts', label: 'Brutal 参数', type: 'object', required: false, object_kind: 'fields', allow_unknown: true,
      feature: { name: 'smux.brutal', toggle: 'enabled' }, reset_on: ['feature.smux.brutal'], when: { features: ['smux'] },
      properties: [
        { name: 'enabled', label: 'Brutal 启用', type: 'bool', required: false, default: false },
        { name: 'up', label: '上行', type: 'text', required: false, when: { features: ['smux.brutal'] } },
        { name: 'down', label: '下行', type: 'text', required: false, when: { features: ['smux.brutal'] } },
      ],
    },
  ],
}

export function smuxValue() {
  return { enabled: true, 'max-connections': 7, padding: true, future: { value: 'old' },
    'brutal-opts': { enabled: true, up: '100 Mbps', down: '200 Mbps', future: 'old' } }
}
