// reset-view.spec.ts：重置链接四态与验证失败恢复（Build11 Step 2）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const pushMock = vi.fn()
const route = { path: '/reset', hash: '' }

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: pushMock }),
}))
vi.mock('@/api/auth', () => ({
  resetPassword: vi.fn(),
  validateResetToken: vi.fn(),
}))
vi.mock('@/theme', () => ({
  useTheme: () => ({ dark: { value: false }, toggle: vi.fn() }),
}))

import ResetView from '@/views/ResetView.vue'
import { validateResetToken } from '@/api/auth'

const mockValidate = validateResetToken as unknown as ReturnType<typeof vi.fn>

describe('ResetView 重置链接状态', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mockValidate.mockReset()
    pushMock.mockReset()
    route.hash = ''
  })

  it('缺失 token 时不渲染密码表单', async () => {
    const wrapper = mount(ResetView)
    await flushPromises()
    expect(wrapper.text()).toContain('重置链接无效')
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
    expect(mockValidate).not.toHaveBeenCalled()
  })

  it.each([
    ['used', '重置链接已使用'],
    ['expired', '重置链接已过期'],
  ] as const)('%s token 显示 Result 而非密码表单', async (status, title) => {
    route.hash = '#token=valid-token'
    mockValidate.mockResolvedValue({ status })
    const wrapper = mount(ResetView)
    await flushPromises()
    expect(wrapper.text()).toContain(title)
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
  })

  it('有效 token 才渲染密码表单', async () => {
    route.hash = '#token=valid-token'
    mockValidate.mockResolvedValue({ status: 'valid' })
    const wrapper = mount(ResetView)
    await flushPromises()
    expect(wrapper.text()).toContain('确认重置')
    expect(wrapper.findAll('input[type="password"]')).toHaveLength(2)
  })

  it('校验网络失败提供重试动作', async () => {
    route.hash = '#token=valid-token'
    mockValidate.mockRejectedValue(new Error('网络异常'))
    const wrapper = mount(ResetView)
    await flushPromises()
    expect(wrapper.text()).toContain('暂时无法验证重置链接')
    expect(wrapper.text()).toMatch(/重\s*试/)
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
  })
})
