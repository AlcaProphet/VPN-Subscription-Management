import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/settings', () => ({
  getOidc: vi.fn().mockResolvedValue({ provider_type: '', base_url: '', realm: '', client_id: '', client_secret: '', frontend_url: '', callback_url: '' }),
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
})
