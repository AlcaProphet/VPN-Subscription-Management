// editable-combobox.spec.ts：Build19 Step 2 可编辑下拉交互测试。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import EditableCombobox from '@/components/EditableCombobox.vue'
import type { OptionItem } from '@/api/node'

const items: OptionItem[] = [
  { value: 'tcp', label: 'TCP', group: 'common', verified: 'mihomo-1.19.29' },
  { value: 'ws', label: 'WebSocket', group: 'common', verified: 'mihomo-1.19.29' },
  { value: 'grpc', label: 'gRPC', group: 'common', verified: 'mihomo-1.19.29' },
]

describe('EditableCombobox', () => {
  it('旧值回显且失焦不自动改写', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: 'custom-old', items, allowCustom: true },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    expect((input.element as HTMLInputElement).value).toBe('custom-old')
    await input.trigger('focus')
    await input.trigger('blur')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('选择候选时回写规范值', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: '', items, allowCustom: true },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    await input.trigger('focus')
    const buttons = wrapper.findAll('button')
    expect(buttons.length).toBeGreaterThanOrEqual(3)
    await buttons[1].trigger('click')
    const events = wrapper.emitted('update:modelValue') ?? []
    expect(events[events.length - 1]).toEqual(['ws'])
    wrapper.unmount()
  })

  it('输入无匹配时可明确使用自定义值', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: '', items, allowCustom: true },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    await input.trigger('focus')
    await input.setValue('custom-v2')
    expect(wrapper.text()).toContain('使用自定义值：custom-v2')
    const custom = wrapper.findAll('button').find((btn) => btn.text().includes('使用自定义值'))
    expect(custom).toBeTruthy()
    await custom!.trigger('click')
    const events = wrapper.emitted('update:modelValue') ?? []
    expect(events[events.length - 1]).toEqual(['custom-v2'])
    wrapper.unmount()
  })

  it('allowCustom=false 时不展示自定义入口', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: '', items, allowCustom: false },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    await input.trigger('focus')
    await input.setValue('no-match')
    expect(wrapper.text()).not.toContain('使用自定义值')
    wrapper.unmount()
  })

  it('可按显示名过滤', async () => {
    const wrapper = mount(EditableCombobox, {
      props: { value: '', items, allowCustom: true },
      attachTo: document.body,
    })
    const input = wrapper.find('input')
    await input.trigger('focus')
    await input.setValue('socket')
    expect(wrapper.text()).toContain('WebSocket')
    expect(wrapper.text()).not.toContain('gRPC')
    wrapper.unmount()
  })
})
