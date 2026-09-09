// diff-view.spec.ts：DiffView jsdiff 切换与超大文本手动启动开关（R13-01）
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import DiffView from '@/components/DiffView.vue'

describe('DiffView', () => {
  it('常规文本按 jsdiff 行级对比渲染旧/新行号与变更列', () => {
    const wrapper = mount(DiffView, {
      props: { oldText: 'a\nb\nc', newText: 'a\nx\nc' },
    })
    const removed = wrapper.find('[data-diff-kind="removed"]')
    const added = wrapper.find('[data-diff-kind="added"]')
    const contexts = wrapper.findAll('[data-diff-kind="context"]')
    expect(removed.exists()).toBe(true)
    expect(added.exists()).toBe(true)
    // 删除行只显示旧行号；新增行只显示新行号；上下文行同时显示旧/新行号
    expect(removed.find('[data-old-line]').attributes('data-old-line')).toBe('2')
    expect(removed.find('[data-new-line]').attributes('data-new-line')).toBe('')
    expect(added.find('[data-old-line]').attributes('data-old-line')).toBe('')
    expect(added.find('[data-new-line]').attributes('data-new-line')).toBe('2')
    expect(contexts[0].find('[data-old-line]').attributes('data-old-line')).toBe('1')
    expect(contexts[0].find('[data-new-line]').attributes('data-new-line')).toBe('1')
    expect(removed.text()).toContain('-')
    expect(removed.text()).toContain('b')
    expect(added.text()).toContain('+')
    expect(added.text()).toContain('x')
    // 不应出现超大文本警告
    expect(wrapper.text()).not.toContain('仍要启动行级对比')
  })

  it('targetMissing 时整体视为新增并给出注释', () => {
    const wrapper = mount(DiffView, {
      props: { oldText: '', newText: 'l1\nl2', targetMissing: true },
    })
    const text = wrapper.text()
    expect(text).toContain('目标尚无激活版本，本次对比为整体新增')
    expect(text).toContain('+l1')
    expect(text).toContain('+l2')
  })

  it('超大文本默认不自动计算，需手动点击开关后才执行对比', async () => {
    const N = 3000
    const oldLines: string[] = []
    const newLines: string[] = []
    for (let i = 0; i < N; i++) {
      oldLines.push(`rule-${i}`)
      newLines.push(i === 1500 ? 'rule-changed' : `rule-${i}`)
    }
    const wrapper = mount(DiffView, {
      props: { oldText: oldLines.join('\n'), newText: newLines.join('\n') },
    })
    // 未启动：显示警告与手动启动按钮，不渲染 diff 结果
    expect(wrapper.text()).toContain('超过安全阈值')
    expect(wrapper.text()).toContain('仍要启动行级对比')
    expect(wrapper.text()).not.toContain('+rule-changed')

    // 手动启动
    await wrapper.find('button').trigger('click')
    await new Promise((r) => setTimeout(r, 30))
    await wrapper.vm.$nextTick()

    const text = wrapper.text()
    expect(text).toContain('+rule-changed')
    expect(text).toContain('-rule-1500')
  })
})
