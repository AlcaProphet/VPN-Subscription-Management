// form-submit.spec.ts：表单校验防回归——Form 必须绑定 :model，否则 required 校验永远失败无法提交
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { Form } from 'ant-design-vue'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  forgot: vi.fn(),
  resetPassword: vi.fn(),
}))
vi.mock('@/api/system', () => ({
  getSystemStatus: vi.fn().mockResolvedValue({ configured: true, app_mode: 'dev', emergency: false }),
}))
vi.mock('@/api/settings', () => ({
  siteInfoPublic: vi.fn().mockResolvedValue({ site_name: '', icon_url: '' }),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ params: { token: 'abc' }, query: {} }),
  RouterLink: { template: '<a><slot /></a>' },
}))
// mock 应用路由（api/request.ts 依赖链引用）
vi.mock('@/router', () => ({
  default: {
    currentRoute: { value: { path: '/login' } },
    push: vi.fn(),
    replace: vi.fn(),
  },
}))

import LoginView from '@/views/LoginView.vue'
import RegisterView from '@/views/RegisterView.vue'
import ForgotView from '@/views/ForgotView.vue'
import { login } from '@/api/auth'

const mockLogin = login as unknown as ReturnType<typeof vi.fn>

describe('认证表单提交', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mockLogin.mockReset()
    mockLogin.mockResolvedValue({ token: 't', expires_at: 1, user: { id: 1, username: 'k', email: 'k@e.com', role: 'user', group_id: null, status: 'active', user_source: 'local' } })
  })

  // 回归防护：AntD Form 未绑定 :model 时，Form.Item 校验取不到字段值，
  // required 规则永远失败 → 表单永远无法提交（本次修复的 bug）
  it('LoginView 的 AntD Form 已绑定 model', () => {
    const wrapper = mount(LoginView)
    const formComp = wrapper.findComponent(Form)
    expect(formComp.exists()).toBe(true)
    expect(formComp.props('model')).toBeDefined()
  })

  it('LoginView 绑定 model 后：填写字段提交可触发 login 调用', async () => {
    const wrapper = mount(LoginView)
    const vm = wrapper.vm as unknown as { form: { email: string; password: string; remember: boolean } }
    vm.form.email = 'user@example.com'
    vm.form.password = 'password123'
    await wrapper.find('form').trigger('submit')
    // AntD 内部校验为异步流程，等待 microtask 后断言
    await new Promise((r) => setTimeout(r, 50))
    expect(mockLogin).toHaveBeenCalled()
  })

  it('RegisterView 的 AntD Form 已绑定 model', () => {
    const wrapper = mount(RegisterView)
    expect(wrapper.findComponent(Form).props('model')).toBeDefined()
  })

  it('ForgotView 的 AntD Form 已绑定 model', () => {
    const wrapper = mount(ForgotView)
    expect(wrapper.findComponent(Form).props('model')).toBeDefined()
  })
})
