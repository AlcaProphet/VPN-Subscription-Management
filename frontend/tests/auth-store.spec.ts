// auth-store.spec.ts：登录/注册 action（mock api）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
}))

import { login, register } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const mockLogin = login as unknown as ReturnType<typeof vi.fn>
const mockRegister = register as unknown as ReturnType<typeof vi.fn>

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mockLogin.mockReset()
    mockRegister.mockReset()
  })

  it('loginAction 成功写入 token 与用户信息', async () => {
    mockLogin.mockResolvedValue({
      token: 'jwt-token',
      expires_at: 1234567890,
      user: { id: 1, username: 'kyle', email: 'k@e.com', role: 'admin', status: 'active', user_source: 'local' },
    })
    const auth = useAuthStore()
    await auth.loginAction({ email: 'k@e.com', password: 'password123', remember: true })
    expect(localStorage.getItem('token')).toBe('jwt-token')
    expect(auth.token).toBe('jwt-token')
    expect(auth.user?.username).toBe('kyle')
    expect(mockLogin).toHaveBeenCalledWith({ email: 'k@e.com', password: 'password123', remember: true })
  })

  it('registerAction 直接激活时写入 token', async () => {
    mockRegister.mockResolvedValue({ status: 'active', token: 'reg-token', is_admin: true })
    const auth = useAuthStore()
    const res = await auth.registerAction({ username: 'kyle', email: 'k@e.com', password: 'password123' })
    expect(res.status).toBe('active')
    expect(auth.token).toBe('reg-token')
  })

  it('registerAction 待审批时不写入 token', async () => {
    mockRegister.mockResolvedValue({ status: 'pending', message: '账号已提交，等待管理员审批' })
    const auth = useAuthStore()
    const res = await auth.registerAction({ username: 'kyle', email: 'k@e.com', password: 'password123' })
    expect(res.status).toBe('pending')
    expect(auth.token).toBe('')
  })

  it('logoutAction 清除本地凭据（接口失败不阻断）', async () => {
    const { logout } = await import('@/api/auth')
    ;(logout as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('network'))
    const auth = useAuthStore()
    auth.setSession('some-token', { id: 1, username: 'k', email: '', role: 'user', group_id: null, status: 'active', user_source: 'local' })
    await auth.logoutAction()
    expect(auth.token).toBe('')
    expect(localStorage.getItem('token')).toBeNull()
  })
})
