// editable-combobox.spec.ts：标准单选下拉与显式自定义草稿交互测试。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { Select } from 'ant-design-vue'
import EditableCombobox from '@/components/EditableCombobox.vue'
import type { OptionItem } from '@/api/node'

const items: OptionItem[] = [
  { value: 'tcp', label: 'TCP', group: 'common', verified: 'mihomo-1.19.29' },
  { value: 'ws', label: 'WebSocket', group: 'common', verified: 'mihomo-1.19.29' },
  { value: 'grpc', label: 'gRPC', group: 'common', verified: 'mihomo-1.19.29' },
]

async function selectValue(wrapper: ReturnType<typeof mount>, value: string) {
  await wrapper.findComponent(Select).vm.$emit('change', value)
}

describe('EditableCombobox', () => {
  it('使用与协议一致的按钮式 Select，整框有箭头且展开显示全部候选', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'tcp', items, allowCustom: true }, attachTo: document.body })
    expect(wrapper.find('.ant-select').exists()).toBe(true)
    expect(wrapper.find('.ant-select-show-search').exists()).toBe(false)
    expect(wrapper.find('.ant-select-arrow').exists()).toBe(true)
    await wrapper.find('.ant-select-selector').trigger('mousedown')
    expect(wrapper.find('.ant-select-show-search').exists()).toBe(false)
    expect(document.body.textContent).toContain('WebSocket')
    expect(document.body.textContent).toContain('gRPC')
    wrapper.unmount()
  })

  it('选择正式候选时直接回写规范值', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'tcp', items, allowCustom: true } })
    await selectValue(wrapper, 'ws')
    expect(wrapper.emitted('update:modelValue')).toEqual([['ws']])
  })

  it('选择其他后仅创建草稿，应用时才回写一次', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: '', items, allowCustom: true } })
    await selectValue(wrapper, '__vpn_sub_custom_value__')
    expect(wrapper.text()).toContain('自定义值草稿未应用')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    const customInput = wrapper.find('.custom-value-editor input')
    await customInput.setValue('custom-v2')
    await customInput.trigger('keydown', { key: 'Enter', isComposing: true })
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.findAll('.custom-value-editor button').find((button) => button.text() === '应用自定义值')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([['custom-v2']])
    expect(wrapper.emitted('draft-dirty-change')).toEqual([[true], [false]])
  })

  it('取消自定义草稿恢复实际已应用值', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'tcp', items, allowCustom: true } })
    await selectValue(wrapper, '__vpn_sub_custom_value__')
    await wrapper.find('.custom-value-editor input').setValue('discard-me')
    await wrapper.find('.custom-value-editor .ant-btn-default').trigger('click')
    expect(wrapper.find('.custom-value-editor').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('旧自定义值自动回填且不编辑时不产生未应用草稿', () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'custom-old', items, allowCustom: true } })
    expect((wrapper.find('.custom-value-editor input').element as HTMLInputElement).value).toBe('custom-old')
    expect(wrapper.text()).not.toContain('草稿未应用')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('draft-dirty-change')).toBeUndefined()
  })

  it('allowCustom=false 时不展示其他入口', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'tcp', items, allowCustom: false }, attachTo: document.body })
    await wrapper.find('.ant-select-selector').trigger('mousedown')
    expect(document.body.textContent).not.toContain('其他（自定义）')
    wrapper.unmount()
  })

  it('空值候选与其他使用不同内部值并正确回写空字符串', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: 'tcp', items: [{ value: '', label: '无' }, ...items], allowCustom: true },
    })
    await selectValue(wrapper, '__vpn_sub_empty_value__')
    expect(wrapper.emitted('update:modelValue')).toEqual([['']])
  })

  it('选项元数据保持可读名称和明确分隔', async () => {
    const wrapper = mount(EditableCombobox, { props: { value: 'tcp', items, allowCustom: true }, attachTo: document.body })
    await wrapper.find('.ant-select-selector').trigger('mousedown')
    expect(document.body.textContent).toContain('Mihomo 1.19.29 · 常用')
    expect(document.body.textContent).not.toContain('mihomo-1.19.29常用')
    wrapper.unmount()
  })
})
