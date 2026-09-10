// preview-step.spec.ts：预览工具栏自动换行开关与复制按钮移除回归测试。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PreviewStep from '@/views/admin/assembly/PreviewStep.vue'
import type { ConversionReceipt } from '@/api/assembly'

function makeWrapper(receipt?: ConversionReceipt | null) {
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
      receipt,
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

  it('渲染完整六项转换回执', () => {
    const wrapper = makeWrapper({
      input: 7, direct_output: 4, equivalent_conversions: 2,
      skipped_unsupported: 1, target_validation_failed: 0, final_output: 6,
    })
    expect(wrapper.find('[data-testid="conversion-receipt"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('输入 7')
    expect(wrapper.text()).toContain('直接输出 4')
    expect(wrapper.text()).toContain('等价转换 2')
    expect(wrapper.text()).toContain('目标不支持跳过 1')
    expect(wrapper.text()).toContain('校验失败 0')
    expect(wrapper.text()).toContain('最终输出 6')
  })

  it('receipt 缺省时不显示虚假零值回执', () => {
    const wrapper = makeWrapper()
    expect(wrapper.find('[data-testid="conversion-receipt"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('转换回执')
    expect(wrapper.text()).not.toContain('最终输出 0')
  })
})
