// overlay-step.spec.ts：覆盖层默认折叠为高级配置展开编辑的回归测试。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import OverlayStep from '@/views/admin/assembly/OverlayStep.vue'

function makeWrapper(withContent = false) {
  const overlay = {
    merge_yaml: withContent ? '{}' : '',
    rules_yaml: withContent ? 'prepend: []' : '',
    proxies_yaml: '',
    groups_yaml: '',
  }
  return mount(OverlayStep, {
    props: { form: { overlay } },
  })
}

describe('OverlayStep', () => {
  it('默认折叠并显示高级配置提示', () => {
    const wrapper = makeWrapper()
    expect(wrapper.text()).toContain('覆盖层（高级配置）')
    expect(wrapper.text()).toContain('默认无需填写')
    expect(wrapper.text()).toContain('展开编辑')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('点击展开后显示四类编辑器与空模板按钮', async () => {
    const wrapper = makeWrapper()
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('Merge YAML')
    expect(wrapper.text()).toContain('Rules Seq')
    expect(wrapper.text()).toContain('Proxies Seq')
    expect(wrapper.text()).toContain('Groups Seq')
    expect(wrapper.text()).toContain('填入空模板')
    expect(wrapper.findAll('textarea').length).toBe(4)
  })

  it('已有内容时折叠态提示已填写', () => {
    const wrapper = makeWrapper(true)
    expect(wrapper.text()).toContain('已填写覆盖层配置')
  })
})
