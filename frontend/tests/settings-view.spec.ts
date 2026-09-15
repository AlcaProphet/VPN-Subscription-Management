import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { Modal, Select } from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/settings', () => ({
  getOidc: vi.fn().mockResolvedValue({ provider_type: '', base_url: '', realm: '', client_id: '', client_secret: '', client_secret_configured: false, frontend_url: '', callback_url: '' }),
  saveOidc: vi.fn(),
  disableOidc: vi.fn(),
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
import { getOidc, saveOidc, disableOidc, testOidc, type OidcSettings, type OidcParamsState } from '@/api/settings'
import { useSystemStore } from '@/stores/system'

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

describe('SettingsView OIDC R31-04 状态展示', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  function mountWithStatus(params_state: OidcParamsState, extra: Record<string, unknown> = {}) {
    vi.mocked(getOidc).mockResolvedValueOnce({
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: false,
      frontend_url: '',
      callback_url: '',
      params_state,
      ...extra,
    })
    return mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
  }

  it('signing_key_fault 显示独立系统错误，阻断保存/测试并禁用重填输入', async () => {
    const wrapper = mountWithStatus('signing_key_fault', {
      params_warning: '系统签名密钥缺失或不可读取，当前无法校验或保存 OIDC 凭据；请通过备份恢复或应急初始化处理，不要在此重填 Secret',
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('系统签名密钥缺失或不可读取')
    expect(card.text()).toContain('不要在此重填 Secret')

    const buttons = card.findAll('button')
    const saveButton = buttons.find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    const testButton = buttons.find((btn) => btn.text().replace(/\s/g, '').includes('测试连接'))
    expect(saveButton?.attributes('disabled')).toBeDefined()
    expect(testButton?.attributes('disabled')).toBeDefined()
    const password = card.find('input[type="password"]')
    expect((password.element as HTMLInputElement).disabled).toBe(true)

    await saveButton!.trigger('click')
    await testButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).not.toHaveBeenCalled()
    expect(testOidc).not.toHaveBeenCalled()
  })

  it('missing_secret 不再显示“留空保持原值”，必须输入新 Secret', async () => {
    const wrapper = mountWithStatus('missing_secret')
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('尚未配置可用 Client Secret')
    expect(card.text()).not.toContain('留空仅在')
    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    expect(saveButton?.attributes('disabled')).toBeUndefined()
  })

  it('json_damaged 不回显猜测字段并提示重填必要参数', async () => {
    const wrapper = mountWithStatus('json_damaged', {
      base_url: '',
      client_id: '',
      params_damaged: true,
      params_warning: '已存 OIDC 参数 JSON 无法解析，请重新填写必要的 Base URL/Realm/Client ID 并输入新的 Client Secret 后保存',
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('JSON 无法解析')
    expect((card.find('input[placeholder="https://idp.example.com"]').element as HTMLInputElement).value).toBe('')
    expect((card.find('input[placeholder="客户端标识"]').element as HTMLInputElement).value).toBe('')
  })

  it('目标读取 signing_key_fault 时只展示目标状态并阻断操作', async () => {
    const source: OidcSettings = {
      provider_type: 'generic', base_url: 'https://source.example.com', realm: '', client_id: 'source-client',
      client_secret: '', client_secret_configured: true, frontend_url: '', callback_url: '', params_state: 'usable',
    }
    const target: OidcSettings = {
      provider_type: 'keycloak', base_url: 'https://target.example.com', realm: 'master', client_id: 'target-client',
      client_secret: '', client_secret_configured: false, frontend_url: '', callback_url: '',
      params_state: 'signing_key_fault', params_warning: '系统签名密钥缺失或不可读取，当前无法校验或保存 OIDC 凭据；请通过备份恢复或应急初始化处理，不要在此重填 Secret',
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
    expect(card.text()).toContain('系统签名密钥缺失或不可读取')
    expect((card.find('input[placeholder="https://idp.example.com"]').element as HTMLInputElement).value).toBe('https://target.example.com')
    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    expect(saveButton?.attributes('disabled')).toBeDefined()
  })
})



describe('SettingsView OIDC R31-05 地址即时生效与显式清除', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('保存地址携带 clear_callback_url=false，页面不再提示需重启', async () => {
    vi.mocked(getOidc).mockResolvedValue({
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://app.example.com',
      callback_url: 'https://callback.example.com/api/auth/oidc/callback',
    })
    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('保存后即时生效')
    expect(card.text()).not.toContain('需重启容器生效')
    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    await saveButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).toHaveBeenCalledWith(expect.objectContaining({
      frontend_url: 'https://app.example.com',
      callback_url: 'https://callback.example.com/api/auth/oidc/callback',
      clear_callback_url: false,
    }))
  })

  it('恢复推导后以 clear_callback_url=true 和空 callback_url 保存', async () => {
    vi.mocked(getOidc).mockResolvedValue({
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://app.example.com',
      callback_url: 'https://callback.example.com/api/auth/oidc/callback',
    })
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      void options.onOk()
      return {} as any
    })
    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    const clearButton = card.findAll('button').find((btn) => btn.text().includes('恢复推导'))
    expect(clearButton).toBeTruthy()
    await clearButton!.trigger('click')
    await flushPromises()
    expect(card.text()).toContain('已标记清除独立回调')
    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    await saveButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).toHaveBeenCalledWith(expect.objectContaining({
      callback_url: '',
      clear_callback_url: true,
    }))
    confirmSpy.mockRestore()
  })

  it('独立回调 host 与前端地址不一致时提示 state Cookie 边界', async () => {
    vi.mocked(getOidc).mockResolvedValue({
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://app.example.com',
      callback_url: 'https://callback.example.com/api/auth/oidc/callback',
    })
    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    expect(wrapper.find('#oidc').text()).toContain('state Cookie')
  })
})

describe('SettingsView R31-06 Production mock 只读边界', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('Production 历史 mock 显示警示、禁用保存/测试且不作为可选启用项', async () => {
    const system = useSystemStore()
    system.status = { configured: true, app_mode: 'prod', advanced_mode: false } as any
    vi.mocked(getOidc).mockResolvedValueOnce({
      provider_type: 'mock',
      base_url: '',
      realm: '',
      client_id: '',
      client_secret: '',
      client_secret_configured: false,
      frontend_url: 'https://app.example.com',
      callback_url: '',
      params_state: 'usable',
    } as OidcSettings)

    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('生产模式不支持模拟 OIDC')
    expect(card.text()).toContain('Mock（生产不可用）')

    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    const testButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('测试连接'))
    expect(saveButton?.attributes('disabled')).toBeDefined()
    expect(testButton?.attributes('disabled')).toBeDefined()

    await saveButton!.trigger('click')
    await testButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).not.toHaveBeenCalled()
    expect(testOidc).not.toHaveBeenCalled()
  })
})

describe('SettingsView R31-07 暂未启用草稿与停用', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('选择暂未启用只形成草稿，点击保存停用才调用 disableOidc', async () => {
    vi.mocked(getOidc).mockResolvedValue({
      enabled: true,
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://app.example.com',
      callback_url: '',
    })
    let confirmOptions: any
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((options: any) => {
      confirmOptions = options
      return {} as any
    })
    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const system = useSystemStore()
    const statusSpy = vi.spyOn(system, 'fetchStatus').mockResolvedValue(undefined as any)
    const card = wrapper.find('#oidc')
    card.findComponent(Select).vm.$emit('change', 'off')
    await flushPromises()

    expect(confirmOptions).toBeTruthy()
    expect(confirmOptions.content).toContain('只形成页面草稿')
    expect(disableOidc).not.toHaveBeenCalled()
    expect(saveOidc).not.toHaveBeenCalled()

    await confirmOptions.onOk()
    await flushPromises()
    expect(card.findComponent(Select).props('value')).toBe('off')
    expect(card.text()).toContain('尚未保存停用')
    const saveDisableButton = card.findAll('button').find((btn) => btn.text().includes('保存停用'))
    expect(saveDisableButton).toBeTruthy()
    await saveDisableButton!.trigger('click')
    await flushPromises()
    expect(disableOidc).toHaveBeenCalledTimes(1)
    expect(statusSpy).toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('停用响应展示保留 provider，选择 provider 保存后才重新启用', async () => {
    const disabled = {
      enabled: false,
      provider_type: 'generic',
      base_url: 'https://idp.example.com',
      realm: '',
      client_id: 'client-x',
      client_secret: '',
      client_secret_configured: true,
      frontend_url: 'https://app.example.com',
      callback_url: '',
      params_state: 'usable' as OidcParamsState,
    }
    vi.mocked(getOidc).mockResolvedValue(disabled)
    const wrapper = mount(SettingsView, {
      global: { mocks: { $router: { push: vi.fn() } } },
    })
    await flushPromises()
    const card = wrapper.find('#oidc')
    expect(card.text()).toContain('OIDC 已停用')
    expect(card.text()).toContain('generic')
    expect(card.findComponent(Select).props('value')).toBe('off')

    card.findComponent(Select).vm.$emit('change', 'generic')
    await flushPromises()
    expect(getOidc).toHaveBeenCalledWith('generic')
    expect(card.findComponent(Select).props('value')).toBe('generic')

    const saveButton = card.findAll('button').find((btn) => btn.text().replace(/\s/g, '').includes('保存'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()
    expect(saveOidc).toHaveBeenCalledTimes(1)
  })
})
