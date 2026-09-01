// oidc-callback-view.spec.ts：OIDC 交换失败留在回调页并提供恢复入口（Build11 Step 2）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const replaceMock = vi.fn()

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ replace: replaceMock }),
}))
vi.mock('@/api/oidc', () => ({ exchangeOidc: vi.fn() }))

import OidcCallbackView from '@/views/OidcCallbackView.vue'
import { exchangeOidc } from '@/api/oidc'

const mockExchange = exchangeOidc as unknown as ReturnType<typeof vi.fn>

describe('OidcCallbackView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockExchange.mockReset()
    replaceMock.mockReset()
  })

  it('交换失败时展示可读错误与三种恢复动作，不跳转登录页', async () => {
    mockExchange.mockRejectedValue(new Error('OIDC 身份提供商暂时不可用'))
    const wrapper = mount(OidcCallbackView)
    await flushPromises()
    expect(wrapper.text()).toContain('无法完成 OIDC 登录')
    expect(wrapper.text()).toContain('OIDC 身份提供商暂时不可用')
    expect(wrapper.text()).toContain('重新使用 OIDC 登录')
    expect(wrapper.text()).toContain('使用本地账号')
    expect(wrapper.text()).toContain('联系管理员')
    expect(replaceMock).not.toHaveBeenCalled()
  })

  it('使用本地账号动作回到登录页', async () => {
    mockExchange.mockRejectedValue(new Error('交换失败'))
    const wrapper = mount(OidcCallbackView)
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('使用本地账号'))!.trigger('click')
    expect(replaceMock).toHaveBeenCalledWith('/login')
  })
})
