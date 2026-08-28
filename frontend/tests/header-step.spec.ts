// header-step.spec.ts：头部参数按输入与开关分区（R24-15）
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import HeaderStep from '@/views/admin/assembly/HeaderStep.vue'

describe('HeaderStep', () => {
  it.each([
    ['clash-yaml', 'Allow LAN'],
    ['sr-conf', 'IPv6'],
  ] as const)('%s 将 bool 参数放入独立开关区', (targetSyntax, label) => {
    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: '{}' }, targetSyntax },
    })

    expect(wrapper.find('.header-switch-fields').text()).toContain(label)
    expect(wrapper.find('.header-input-fields').text()).not.toContain(label)
  })
})
