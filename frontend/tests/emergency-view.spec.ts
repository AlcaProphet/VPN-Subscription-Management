// emergency-view.spec.ts：/emergency 按真实系统状态渲染（Build11 Step 2）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'

vi.mock('@/api/system', () => ({ getSystemStatus: vi.fn() }))
vi.mock('@/api/emergency', () => ({
  emergencyVerify: vi.fn(), emergencyResetPassword: vi.fn(), emergencyReinitialize: vi.fn(),
}))
vi.mock('@/components/Notify', () => ({
  Notify: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() },
}))

import EmergencyView from '@/views/EmergencyView.vue'
import { getSystemStatus } from '@/api/system'

const mockStatus = getSystemStatus as unknown as ReturnType<typeof vi.fn>

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/emergency', component: EmergencyView },
      { path: '/login', component: { template: '<div>login</div>' } },
      { path: '/', component: { template: '<div>home</div>' } },
    ],
  })
}

describe('EmergencyView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockStatus.mockReset()
  })

  it('非应急状态显示信息页，不展示操作码或暂停服务文案', async () => {
    mockStatus.mockResolvedValue({ configured: true, app_mode: 'prod', advanced_mode: false, emergency: false })
    const router = makeRouter()
    await router.push('/emergency')
    await router.isReady()
    const wrapper = mount(EmergencyView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('当前未处于应急恢复模式')
    expect(wrapper.text()).not.toContain('正常服务已暂停')
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it('应急状态才展示操作码验证', async () => {
    mockStatus.mockResolvedValue({ configured: true, app_mode: 'prod', advanced_mode: false, emergency: true, emergency_reason: 'manual' })
    const router = makeRouter()
    await router.push('/emergency')
    await router.isReady()
    const wrapper = mount(EmergencyView, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.text()).toContain('应急恢复模式')
    expect(wrapper.text()).toContain('正常服务已暂停')
    expect(wrapper.find('input').exists()).toBe(true)
  })
})
