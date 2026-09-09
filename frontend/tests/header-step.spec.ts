// header-step.spec.ts：头部参数的分区、开关与默认值。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import HeaderStep from '@/views/admin/assembly/HeaderStep.vue'
import { CLASH_HEADER_DEFAULTS } from '@/views/admin/assembly/clashHeaderDefaults'

describe('HeaderStep', () => {
  it('Clash 头部按四个默认折叠分区展示，更多参数集中开关', async () => {
    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: JSON.stringify(CLASH_HEADER_DEFAULTS) }, targetSyntax: 'clash-yaml' },
    })

    const headers = wrapper.findAll('.ant-collapse-header')
    expect(headers).toHaveLength(4)
    expect(wrapper.findAll('.ant-collapse-item-active')).toHaveLength(0)
    expect(wrapper.text()).toContain('端口配置')
    expect(wrapper.text()).toContain('Geo 数据')
    expect(wrapper.text()).toContain('DNS 配置')
    expect(wrapper.text()).toContain('更多参数')

    await headers[3].trigger('click')
    expect(wrapper.find('[data-section="more"] .header-switch-fields').text()).toContain('Allow LAN')
    expect(wrapper.find('[data-section="more"] .header-input-fields').text()).not.toContain('Allow LAN')
  })

  it('SR 分流规则继续将 bool 参数放入独立开关区', () => {
    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: '{}' }, targetSyntax: 'sr-conf' },
    })

    expect(wrapper.find('.header-switch-fields').text()).toContain('IPv6')
    expect(wrapper.find('.header-input-fields').text()).not.toContain('IPv6')
  })

  it('Clash 默认值完整采用个人模板，分区可切换高级 JSON', async () => {
    expect(CLASH_HEADER_DEFAULTS).toMatchObject({
      port: 7890,
      'socks-port': 7891,
      'redir-port': 7892,
      'tproxy-port': 7893,
      'geo-auto-update': true,
      'geo-update-interval': 168,
      'log-level': 'warning',
      dns: { enable: true, fallback: ['1.1.1.1', '8.8.8.8'] },
    })

    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: JSON.stringify(CLASH_HEADER_DEFAULTS) }, targetSyntax: 'clash-yaml' },
    })
    await wrapper.findAll('.ant-collapse-header')[0].trigger('click')

    expect(wrapper.text()).toContain('使用默认值')
    expect(wrapper.text()).toContain('高级 JSON')
    expect(wrapper.find('[data-section="ports"] .ant-switch').exists()).toBe(true)
  })

  it('Clash 分区清晰提示预填状态，DNS 子开关受启用状态控制', async () => {
    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: JSON.stringify(CLASH_HEADER_DEFAULTS) }, targetSyntax: 'clash-yaml' },
    })
    const headers = wrapper.findAll('.ant-collapse-header')

    expect(wrapper.text()).toContain('按需展开编辑')
    expect(wrapper.text()).toContain('不确定参数用途时请保持默认值')
    expect(wrapper.findAll('.collapse-panel-title span:last-child')).toHaveLength(4)

    await headers[2].trigger('click')
    const dns = wrapper.find('[data-section="dns"]')
    expect(dns.find('.dns-filter-collapse').text()).toContain('fake-ip-filter（13 项，默认已预填）')
    expect(dns.findAll('.section-switch-option')).toHaveLength(3)
    await dns.find('.section-switch-option .ant-switch').trigger('click')
    expect(dns.findAll('.section-switch-option')).toHaveLength(1)
  })

  it('更多参数默认选中阿里云 NTP，并提供自定义入口', async () => {
    const wrapper = mount(HeaderStep, {
      props: { form: { fixed_params_text: JSON.stringify(CLASH_HEADER_DEFAULTS) }, targetSyntax: 'clash-yaml' },
    })
    await wrapper.findAll('.ant-collapse-header')[3].trigger('click')

    const more = wrapper.find('[data-section="more"]')
    expect(more.text()).toContain('常用 NTP 服务器')
    expect(more.text()).toContain('阿里云（ntp.aliyun.com）')
    expect(more.text()).toContain('自定义 NTP 服务器（可选）')
  })
})
