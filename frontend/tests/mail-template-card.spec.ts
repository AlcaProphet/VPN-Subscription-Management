import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { Modal, Radio, Select } from 'ant-design-vue'
import AppSelect from '@/components/AppSelect.vue'

vi.mock('@/api/settings', () => ({
  getMailTemplates: vi.fn(),
  saveMailTemplate: vi.fn(),
  restoreMailTemplate: vi.fn(),
  previewMailTemplate: vi.fn(),
}))

import MailTemplateCard from '@/components/settings/MailTemplateCard.vue'
import {
  getMailTemplates, previewMailTemplate, saveMailTemplate, restoreMailTemplate,
  type MailTemplateView,
} from '@/api/settings'

const templates: MailTemplateView[] = [
  {
    id: 'password_reset', label: '密码重置', scope: 'password_reset',
    subject: '密码重置', body: '请在 1 小时内使用以下链接重置密码（一次性）：\n{{reset_url}}',
    state: 'default', warning: '', subject_variables: [], body_variables: ['reset_url'], required_body_variables: ['reset_url'],
  },
  {
    id: 'approval_approved', label: '审批通过', scope: 'approval_notify',
    subject: '{{site_name}} 审批通知', body: '您在 {{site_name}} 的账号已通过审批，现在可以登录：\n{{login_url}}',
    state: 'default', warning: '', subject_variables: ['site_name'], body_variables: ['site_name', 'login_url'], required_body_variables: ['login_url'],
  },
  {
    id: 'approval_rejected', label: '审批拒绝', scope: 'approval_notify',
    subject: '{{site_name}} 审批通知', body: '您在 {{site_name}} 的账号申请未通过审批。',
    state: 'default', warning: '', subject_variables: ['site_name'], body_variables: ['site_name'], required_body_variables: [],
  },
  {
    id: 'welcome_local', label: '本地欢迎', scope: 'welcome',
    subject: '{{site_name}} 账号已激活', body: '{{site_name}}\n\n您的账号已激活，请使用邮箱与密码登录：{{login_url}}',
    state: 'default', warning: '', subject_variables: ['site_name'], body_variables: ['site_name', 'login_url'], required_body_variables: ['login_url'],
  },
  {
    id: 'welcome_oidc', label: 'OIDC 欢迎', scope: 'welcome',
    subject: '{{site_name}} 账号已激活', body: '{{site_name}}\n\n您的账号已激活，请使用单点登录（OIDC）登录：{{login_url}}',
    state: 'default', warning: '', subject_variables: ['site_name'], body_variables: ['site_name', 'login_url'], required_body_variables: ['login_url'],
  },
]

function cloneTemplates() {
  return templates.map((item) => ({
    ...item,
    subject_variables: [...item.subject_variables],
    body_variables: [...item.body_variables],
    required_body_variables: [...item.required_body_variables],
  }))
}

function lastDirty(wrapper: any) {
  const events = wrapper.emitted('dirty-change') as unknown[][] | undefined
  return events?.[events.length - 1]
}

describe('MailTemplateCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getMailTemplates).mockResolvedValue({
      templates: cloneTemplates(),
      limits: { subject: 200, body: 10000 },
      preview_values: {
        site_name: 'VPN 订阅管理',
        login_url: 'https://example.invalid/login?source=preview',
        reset_url: 'https://example.invalid/reset/example-token?source=preview',
      },
    })
    vi.mocked(previewMailTemplate).mockResolvedValue({
      subject: '预览主题',
      text_body: '预览文本 https://example.invalid/login?source=preview',
      html_body: '<a href="https://example.invalid/login?source=preview">https://example.invalid/login?source=preview</a>',
    })
  })

  it('首次加载一次模板 GET 并默认选中 password_reset', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    expect(getMailTemplates).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('邮件内容')
    expect(wrapper.text()).toContain('密码重置')
    expect(wrapper.text()).toContain('默认文案')
  })

  it('加载服务端 limits/变量元数据并渲染五个固定分支', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const select = wrapper.findComponent(AppSelect).findComponent(Select)
    expect(select.exists()).toBe(true)
    expect(select.props('value')).toBe('password_reset')
    expect((select.props('options') as any[]).map((item) => item.value)).toEqual([
      'password_reset', 'approval_approved', 'approval_rejected', 'welcome_local', 'welcome_oidc',
    ])
    expect(wrapper.text()).toContain('/200')
    expect(wrapper.text()).toContain('/10000')
    expect(wrapper.text()).toContain('{{reset_url}}')
    expect(previewMailTemplate).toHaveBeenCalledTimes(1)
  })

  it('变量按钮按光标插入，未聚焦时插入末尾', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const textarea = wrapper.find('textarea[aria-label="邮件正文"]')
    await textarea.setValue('ABC')
    await textarea.trigger('focus')
    const element = textarea.element as HTMLTextAreaElement
    element.setSelectionRange(1, 1)
    const button = wrapper.findAll('button').find((item) => item.text().includes('{{reset_url}}'))
    expect(button).toBeTruthy()
    await button!.trigger('click')
    await nextTick()
    expect((wrapper.find('textarea[aria-label="邮件正文"]').element as HTMLTextAreaElement).value)
      .toBe('A{{reset_url}}BC')

    const appendWrapper = mount(MailTemplateCard)
    await flushPromises()
    const appendTextarea = appendWrapper.find('textarea[aria-label="邮件正文"]')
    await appendTextarea.setValue('XYZ')
    const appendButton = appendWrapper.findAll('button').find((item) => item.text().includes('{{reset_url}}'))
    await appendButton!.trigger('click')
    await nextTick()
    expect((appendWrapper.find('textarea[aria-label="邮件正文"]').element as HTMLTextAreaElement).value)
      .toBe('XYZ{{reset_url}}')
    wrapper.unmount()
    appendWrapper.unmount()
  })

  it('预览请求 299ms 不触发、300ms 触发一次', async () => {
    vi.useFakeTimers()
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    vi.mocked(previewMailTemplate).mockClear()
    const input = wrapper.find('input[aria-label="邮件主题"]')
    await input.setValue('新主题')
    await nextTick()
    expect(previewMailTemplate).not.toHaveBeenCalled()
    vi.advanceTimersByTime(299)
    await flushPromises()
    expect(previewMailTemplate).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    await flushPromises()
    expect(previewMailTemplate).toHaveBeenCalledTimes(1)
    vi.useRealTimers()
  })

  it('后发先至保护只允许最新预览落地', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const pending: Array<(value: any) => void> = []
    vi.mocked(previewMailTemplate).mockImplementation(() => new Promise((resolve) => {
      pending.push(resolve)
    }))
    const input = wrapper.find('input[aria-label="邮件主题"]')
    await input.setValue('第一版')
    await new Promise((resolve) => setTimeout(resolve, 310))
    await flushPromises()
    await input.setValue('第二版')
    await new Promise((resolve) => setTimeout(resolve, 310))
    await flushPromises()
    expect(pending.length).toBeGreaterThanOrEqual(2)
    pending[pending.length - 1]({ subject: '第二版', text_body: 'second-body', html_body: '<p>second</p>' })
    await flushPromises()
    expect(wrapper.text()).toContain('第二版')
    pending[pending.length - 2]({ subject: '第一版', text_body: 'first-body', html_body: '<p>first</p>' })
    await flushPromises()
    expect(wrapper.text()).toContain('第二版')
    expect(wrapper.text()).not.toContain('第一版')
  })

  it('切换分支时确认丢弃并可取消', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const confirmOptions: any[] = []
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions.push(options)
      return {} as any
    })
    await wrapper.find('input[aria-label="邮件主题"]').setValue('未保存主题')
    await nextTick()
    expect(lastDirty(wrapper)).toEqual([true])

    wrapper.findComponent(AppSelect).findComponent(Select).vm.$emit('change', 'approval_approved')
    await nextTick()
    expect(confirmOptions).toHaveLength(1)
    expect(confirmOptions[0].content).toContain('丢弃')

    confirmOptions[0].onOk()
    await flushPromises()
    expect(wrapper.findComponent(AppSelect).findComponent(Select).props('value')).toBe('approval_approved')
    expect(wrapper.text()).toContain('审批通过')
    expect(lastDirty(wrapper)).toEqual([false])
    confirmSpy.mockRestore()
  })

  it('保存成功重建基线并清除 dirty，失败保留草稿', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    await wrapper.find('input[aria-label="邮件主题"]').setValue('保存主题')
    await nextTick()
    vi.mocked(saveMailTemplate).mockResolvedValueOnce({
      ...templates[0], subject: '保存主题', state: 'customized',
    })
    const saveButton = wrapper.findAll('button').find((item) => item.text().includes('保存当前模板'))
    await saveButton!.trigger('click')
    await flushPromises()
    expect(saveMailTemplate).toHaveBeenCalledWith('password_reset', { subject: '保存主题', body: templates[0].body })
    expect(lastDirty(wrapper)).toEqual([false])
    expect(wrapper.text()).toContain('已自定义')

    const failed = mount(MailTemplateCard)
    await flushPromises()
    await failed.find('input[aria-label="邮件主题"]').setValue('失败主题')
    await nextTick()
    vi.mocked(saveMailTemplate).mockRejectedValueOnce(new Error('保存失败'))
    const failedButton = failed.findAll('button').find((item) => item.text().includes('保存当前模板'))
    await failedButton!.trigger('click')
    await flushPromises()
    expect(lastDirty(failed)).toEqual([true])
    expect((failed.find('input[aria-label="邮件主题"]').element as HTMLInputElement).value).toBe('失败主题')
  })

  it('恢复默认需二次确认，成功后回到 default 且按钮禁用', async () => {
    vi.mocked(getMailTemplates).mockResolvedValueOnce({
      templates: [
        { ...templates[0], state: 'customized', subject: '自定义主题', body: '自定义正文 {{reset_url}}' },
        ...templates.slice(1),
      ],
      limits: { subject: 200, body: 10000 },
      preview_values: {
        site_name: 'VPN 订阅管理',
        login_url: 'https://example.invalid/login?source=preview',
        reset_url: 'https://example.invalid/reset/example-token?source=preview',
      },
    } as any)
    vi.mocked(restoreMailTemplate).mockResolvedValueOnce(cloneTemplates()[0] as any)
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const restoreButton = wrapper.findAll('button').find((item) => item.text().includes('恢复默认'))
    expect(restoreButton!.attributes('disabled')).toBeUndefined()
    const confirmOptions: any[] = []
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions.push(options)
      return {} as any
    })
    await restoreButton!.trigger('click')
    await nextTick()
    expect(confirmOptions).toHaveLength(1)
    await confirmOptions[0].onOk()
    await flushPromises()
    expect(restoreMailTemplate).toHaveBeenCalledWith('password_reset')
    expect((wrapper.find('input[aria-label="邮件主题"]').element as HTMLInputElement).value).toBe(templates[0].subject)
    expect(wrapper.text()).toContain('默认文案')
    confirmSpy.mockRestore()
  })

  it('damaged 显示警示与默认值且恢复按钮可用', async () => {
    vi.mocked(getMailTemplates).mockResolvedValueOnce({
      templates: [
        { ...templates[0], state: 'damaged', warning: '模板配置损坏，已回退内置默认文案。' },
        ...templates.slice(1),
      ],
      limits: { subject: 200, body: 10000 },
      preview_values: {
        site_name: 'VPN 订阅管理',
        login_url: 'https://example.invalid/login?source=preview',
        reset_url: 'https://example.invalid/reset/example-token?source=preview',
      },
    } as any)
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('配置损坏')
    expect(wrapper.text()).toContain('已回退内置默认文案')
    expect((wrapper.find('textarea[aria-label="邮件正文"]').element as HTMLTextAreaElement).value)
      .toBe(templates[0].body)
    const restoreButton = wrapper.findAll('button').find((item) => item.text().includes('恢复默认'))
    expect(restoreButton!.attributes('disabled')).toBeUndefined()
  })

  it('HTML 预览使用 sandbox iframe，纯文本预览保留完整 URL', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const iframe = wrapper.find('iframe')
    expect(iframe.exists()).toBe(true)
    expect(iframe.attributes('sandbox')).toBe('')
    expect(iframe.attributes('srcdoc')).toContain('https://example.invalid/login?source=preview')

    const radioGroup = wrapper.findComponent(Radio.Group)
    radioGroup.vm.$emit('update:value', 'text')
    await nextTick()
    expect(wrapper.find('pre').text()).toContain('https://example.invalid/login?source=preview')
  })

  it('预览失败清空旧预览并显示错误', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('预览主题')
    vi.useFakeTimers()
    vi.mocked(previewMailTemplate).mockRejectedValueOnce(new Error('邮件内容预览失败'))
    await wrapper.find('input[aria-label="邮件主题"]').setValue('新主题')
    await nextTick()
    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(wrapper.text()).toContain('邮件内容预览失败')
    expect(wrapper.text()).not.toContain('预览主题')
    vi.useRealTimers()
  })

  it('首次读取失败后可重试成功', async () => {
    vi.mocked(getMailTemplates).mockRejectedValueOnce(new Error('加载失败'))
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('加载失败')
    vi.mocked(getMailTemplates).mockResolvedValueOnce({
      templates: cloneTemplates(),
      limits: { subject: 200, body: 10000 },
      preview_values: {
        site_name: 'VPN 订阅管理',
        login_url: 'https://example.invalid/login?source=preview',
        reset_url: 'https://example.invalid/reset/example-token?source=preview',
      },
    })
    await nextTick()
    const retry = wrapper.findAll('button').find((item) => item.text().replace(/\s/g, '').includes('重试'))
    expect(retry, wrapper.html()).toBeTruthy()
    await retry!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('密码重置')
    expect(wrapper.text()).toContain('默认文案')
  })

  it('新预览请求开始后立即清空旧预览，只允许最新结果落地', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    expect(wrapper.text()).toContain('预览主题')

    let resolveSecond: (value: any) => void = () => {}
    vi.mocked(previewMailTemplate).mockImplementationOnce(() => new Promise((resolve) => {
      resolveSecond = resolve
    }))
    await wrapper.find('input[aria-label="邮件主题"]').setValue('第二次预览')
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 310))
    await flushPromises()

    // 请求已开始但未返回，旧预览必须已清空。
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('预览主题')

    resolveSecond({ subject: '第二次预览结果', text_body: 'second', html_body: '<p>second</p>' })
    await flushPromises()
    expect(wrapper.text()).toContain('第二次预览结果')
  })

  it('组件卸载时清空 debounce 定时器，不再发起预览请求', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    vi.mocked(previewMailTemplate).mockClear()
    vi.useFakeTimers()
    try {
      await wrapper.find('input[aria-label="邮件主题"]').setValue('卸载前修改')
      await nextTick()
      wrapper.unmount()
      vi.advanceTimersByTime(400)
      await flushPromises()
      expect(previewMailTemplate).not.toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })

  it('切换分支取消时保留当前分支与未保存草稿', async () => {
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    const confirmOptions: any[] = []
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions.push(options)
      return {} as any
    })
    await wrapper.find('input[aria-label="邮件主题"]').setValue('未保存主题')
    await nextTick()

    const select = wrapper.findComponent(AppSelect).findComponent(Select)
    select.vm.$emit('change', 'approval_approved')
    await nextTick()
    expect(confirmOptions).toHaveLength(1)
    expect(confirmOptions[0].cancelText).toBe('继续编辑')
    expect(typeof confirmOptions[0].onOk).toBe('function')

    // 模拟“继续编辑”：不执行 onOk，分支与草稿均不得变化。
    await flushPromises()
    expect(select.props('value')).toBe('password_reset')
    expect((wrapper.find('input[aria-label="邮件主题"]').element as HTMLInputElement).value).toBe('未保存主题')
    confirmSpy.mockRestore()
  })

  it('恢复默认失败时保留自定义草稿与状态', async () => {
    vi.mocked(getMailTemplates).mockResolvedValueOnce({
      templates: [
        { ...templates[0], state: 'customized', subject: '自定义主题', body: '自定义正文 {{reset_url}}' },
        ...templates.slice(1),
      ],
      limits: { subject: 200, body: 10000 },
      preview_values: {
        site_name: 'VPN 订阅管理',
        login_url: 'https://example.invalid/login?source=preview',
        reset_url: 'https://example.invalid/reset/example-token?source=preview',
      },
    } as any)
    const wrapper = mount(MailTemplateCard)
    await flushPromises()
    vi.mocked(restoreMailTemplate).mockRejectedValueOnce(new Error('恢复失败'))
    const confirmOptions: any[] = []
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions.push(options)
      return {} as any
    })
    const restoreButton = wrapper.findAll('button').find((item) => item.text().includes('恢复默认'))
    await restoreButton!.trigger('click')
    await nextTick()
    expect(confirmOptions).toHaveLength(1)

    await confirmOptions[0].onOk()
    await flushPromises()
    expect(restoreMailTemplate).toHaveBeenCalledWith('password_reset')
    expect((wrapper.find('input[aria-label="邮件主题"]').element as HTMLInputElement).value).toBe('自定义主题')
    expect(wrapper.text()).toContain('已自定义')
    confirmSpy.mockRestore()
  })

})
