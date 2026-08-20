// rules-step.spec.ts：装配规则素材池有序列表桌面拖拽（R14-15）
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import RulesStep from '@/views/admin/assembly/RulesStep.vue'

const baseProps = {
  form: {
    pools: [
      { pool_id: 1, target: 'PROXY' },
      { pool_id: 2, target: 'PROXY' },
    ],
    custom_rules: [],
  },
  context: {
    pools: [
      { id: 1, name: '池A', urls: [], entry_count: 0, last_synced_at: '', sync_status: '', sync_error: '', auto_sync: false, sync_time: '04:00' },
      { id: 2, name: '池B', urls: [], entry_count: 0, last_synced_at: '', sync_status: '', sync_error: '', auto_sync: false, sync_time: '04:00' },
    ],
    nodes: [],
    proxy_groups: [],
    platforms: [],
    rules: [],
  },
  targetSyntax: 'sr-conf' as const,
  outputGroups: ['PROXY', 'DIRECT'],
  ruleTypeOptions: ['DOMAIN', 'DOMAIN-SUFFIX'],
}

describe('RulesStep 素材池拖拽排序', () => {
  it('拖拽行到目标行触发 move-pool', async () => {
    const wrapper = mount(RulesStep, { props: baseProps })
    const rows = wrapper.findAll('[draggable="true"]')
    expect(rows.length).toBe(2)
    await rows[0].trigger('dragstart')
    await rows[1].trigger('drop')
    const emitted = wrapper.emitted('move-pool')
    expect(emitted).toBeTruthy()
    expect(emitted![0]).toEqual([0, 1])
  })
})
