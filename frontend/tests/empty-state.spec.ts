// empty-state.spec.ts：统一空状态的标题、说明与下一步操作。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import EmptyState from '@/components/EmptyState.vue'

describe('EmptyState', () => {
  it('展示明确的下一步与操作插槽', () => {
    const wrapper = mount(EmptyState, {
      props: { title: '还没有节点', description: '请手动添加节点，或完成 Xray 节点检测。' },
      slots: { actions: '<button type="button">新建节点</button>' },
    })
    expect(wrapper.text()).toContain('还没有节点')
    expect(wrapper.text()).toContain('请手动添加节点')
    expect(wrapper.text()).toContain('新建节点')
  })
})
