// preview-step.spec.ts：预览工具栏自动换行开关与复制按钮移除回归测试。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PreviewStep from '@/views/admin/assembly/PreviewStep.vue'

function makeWrapper() {
  return mount(PreviewStep, {
    props: {
      previewing: false,
      previewWarnings: [],
      previewSkipped: [],
      previewText: 'proxies:\n  - name: a\n',
      previewStale: false,
      previewedAt: Date.now(),
      previewedTargetSyntax: 'clash-yaml' as const,
      showDiff: false,
      diffOld: '',
      diffMissing: false,
      diffLoading: false,
    },
  })
}

describe('PreviewStep 工具栏', () => {
  it('自动换行使用 Switch 表达，且不渲染复制按钮', () => {
    const wrapper = makeWrapper()
    expect(wrapper.find('.ant-switch').exists()).toBe(true)
    expect(wrapper.text()).toContain('自动换行')
    expect(wrapper.text()).not.toContain('复制')
    expect(wrapper.text()).not.toContain('取消换行')
  })

  it('切换自动换行开关后状态仍保持为 Switch', async () => {
    const wrapper = makeWrapper()
    const before = wrapper.find('.ant-switch')
    await before.trigger('click')
    expect(wrapper.find('.ant-switch').exists()).toBe(true)
  })

  it('差异对比未加载时通过 Segmented 进入并显示加载动作', async () => {
    const wrapper = makeWrapper()
    const vm = wrapper.vm as unknown as { activeView: 'preview' | 'diff' }
    expect(vm.activeView).toBe('preview')
    vm.activeView = 'diff'
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('差异对比')
    expect(wrapper.text()).toContain('加载当前激活版本差异')
    expect(wrapper.text()).not.toContain('与当前激活版本对比')
  })
})
