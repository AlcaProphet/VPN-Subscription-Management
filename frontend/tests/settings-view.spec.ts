import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { Modal, Select } from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/settings', () => ({
  getOidc: vi.fn().mockResolvedValue({ provider_type: '', base_url: '', realm: '', client_id: '', client_secret: '', client_secret_configured: false, frontend_url: '', callback_url: '' }),
  saveOidc: vi.fn(),
  clearOidc: vi.fn(),
  testOidc: vi.fn(),
  getOidcRules: vi.fn().mockResolvedValue({}),
  saveOidcRules: vi.fn(),
  getLocalAuth: vi.fn().mockResolvedValue({}),
  saveLocalAuth: vi.fn(),
  getCaptcha: vi.fn().mockResolvedValue({}),
  saveCaptcha: vi.fn(),
  getSMTP: vi.fn().mockResolvedValue({}),
  saveSMTP: vi.fn(),
  testSMTP: vi.fn(),
  getSite: vi.fn().mockResolvedValue({}),
  saveSite: vi.fn(),
  deleteSiteIcon: vi.fn(),
  getRateLimit: vi.fn().mockResolvedValue({}),
  saveRateLimit: vi.fn(),
  getLogLevel: vi.fn().mockResolvedValue({ level: 'info' }),
  saveLogLevel: vi.fn(),
  getAnnouncement: vi.fn().mockResolvedValue({}),
  saveAnnouncement: vi.fn(),
  getDebug: vi.fn().mockResolvedValue({ on: false }),
  saveDebug: vi.fn(),
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
  clearAll: vi.fn(),
  downloadBackup: vi.fn(),
  getAdvancedSettings: vi.fn().mockResolvedValue({ advanced_mode: false, collect_interval_minutes: 10, traffic_card_enabled: true }),
  saveAdvancedSettings: vi.fn(),
  getAdminTask: vi.fn(),
}))

vi.mock('@/api/system', () => ({
  getSystemStatus: vi.fn().mockResolvedValue({ configured: true, app_mode: 'dev', advanced_mode: false }),
}))

import SettingsView from '@/views/admin/SettingsView.vue'
import { getOidc, saveOidc } from '@/api/settings'

describe('SettingsView 基础渲染', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('渲染高级模式卡片', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('高级模式')
  })

  it('渲染配置导入导出入口', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toMatch(/导出|导入/)
  })

  it('高级模式未开启时展示 DISABLE 确认说明', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('DISABLE')
    expect(wrapper.text()).toContain('关闭高级模式会清空')
  })

  it('Dev 模式展示配置导入导出不可用提示', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('Dev 模式不提供配置导入导出')
  })

  it('OIDC Secret 空回显、状态标签与空值保存提交', async () => {
    vi.mocked(getOidc).mockResolvedValueOnce({
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: '',
      callback_url: '',
    })
    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('已配置')
    const password = card.find('input[type="password"]')
    expect(password.exists()).toBe(true)
    expect((password.element as HTMLInputElement).value).toBe('')

    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).toHaveBeenCalledTimes(1)
    expect(saveOidc).toHaveBeenCalledWith(expect.objectContaining({ client_secret: '' }))
  })

})

describe('SettingsView OIDC 目标切换', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })


  it('切换提供商读取目标字段、丢弃参数草稿并保留站点地址', async () => {
    const source = {
      provider_type: 'generic',
      base_url: 'https://source.example.com',
      realm: '',
      client_id: 'source-client',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://site.example.com',
      callback_url: '',
    }
    const target = {
      provider_type: 'keycloak',
      base_url: 'https://target.example.com',
      realm: 'master',
      client_id: 'target-client',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://should-not-apply.example.com',
      callback_url: '',
    }
    vi.mocked(getOidc).mockImplementation((providerType?: string) =>
      Promise.resolve(providerType === 'keycloak' ? target : source))
    let confirmOptions: any
    vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions = options
      return {} as any
    })

    const wrapper = mount(SettingsView, {
      global: {
        mocks: { $router: { push: vi.fn() } },
      },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    await card.find('input[placeholder="https://idp.example.com"]').setValue('https://draft.example.com') // 制造源提供商参数草稿

    card.findComponent(Select).vm.$emit('change', 'keycloak')
    await flushPromises()
    expect(confirmOptions).toBeTruthy()
    expect(confirmOptions.content).toContain('目标提供商自己的')
    expect(confirmOptions.content).toContain('未保存参数草稿将被丢弃')
    await confirmOptions.onOk()
    await flushPromises()

    expect(getOidc).toHaveBeenLastCalledWith('keycloak')
    expect((card.find('input[placeholder="https://idp.example.com"]').element as HTMLInputElement).value).toBe('https://target.example.com')
    expect((card.find('input[placeholder="Keycloak 专用，如 master"]').element as HTMLInputElement).value).toBe('master')
    expect((card.find('input[placeholder="客户端标识"]').element as HTMLInputElement).value).toBe('target-client')
    expect((card.find('input[type="password"]').element as HTMLInputElement).value).toBe('') // Secret 始终为空
    expect((card.find('input[placeholder="https://app.example.com"]').element as HTMLInputElement).value).toBe('https://site.example.com') // 站点地址不被目标读取覆盖
    expect(card.text()).toContain('已配置')
  })

  it('目标读取损坏时显示警示并允许重填', async () => {
    const source = {
      provider_type: 'generic', base_url: 'https://source.example.com', realm: '', client_id: 'source-client',
      client_secret: '', client_secret_configured: true, frontend_url: '', callback_url: '',
    }
    const target = {
      provider_type: 'keycloak', base_url: 'https://target.example.com', realm: 'master', client_id: 'target-client',
      client_secret: '', client_secret_configured: false, frontend_url: '', callback_url: '',
      params_damaged: true, params_warning: '已存 Client Secret 损坏或为脱敏占位符，请输入新的 Client Secret 后保存',
    }
    vi.mocked(getOidc).mockImplementation((providerType?: string) =>
      Promise.resolve(providerType === 'keycloak' ? target : source))
    vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      void options.onOk()
      return {} as any
    })

    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    card.findComponent(Select).vm.$emit('change', 'keycloak')
    await flushPromises()

    expect(card.text()).toContain('已存 Client Secret 损坏')
    expect(card.text()).toContain('必须输入新的 Client Secret')
    expect((card.find('input[placeholder="https://idp.example.com"]').element as HTMLInputElement).value).toBe('https://target.example.com')
  })

  it('目标读取失败时保持源提供商字段与草稿', async () => {
    const source = {
      provider_type: 'generic', base_url: 'https://source.example.com', realm: '', client_id: 'source-client',
      client_secret: '', client_secret_configured: true, frontend_url: '', callback_url: '',
    }
    vi.mocked(getOidc).mockImplementation((providerType?: string) =>
      providerType === 'keycloak' ? Promise.reject(new Error('目标配置读取失败')) : Promise.resolve(source))
    vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      void options.onOk()
      return {} as any
    })

    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    await card.find('input[placeholder="https://idp.example.com"]').setValue('https://draft.example.com')
    card.findComponent(Select).vm.$emit('change', 'keycloak')
    await flushPromises()

    expect((card.find('input[placeholder="https://idp.example.com"]').element as HTMLInputElement).value).toBe('https://draft.example.com')
    expect(card.findComponent(Select).props('value')).toBe('generic')
  })

})

