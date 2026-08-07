// request.spec.ts：401 响应 → auth store 清空、跳转 /login（登录页内 401 不跳转）
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { AxiosError } from 'axios'
import { http, ApiError } from '@/api/request'

// mock 路由跳转
const pushMock = vi.fn()
vi.mock('@/router', () => ({
  default: {
    currentRoute: { value: { path: '/home' } },
    push: (...args: unknown[]) => pushMock(...args),
  },
}))

describe('axios 拦截器', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    pushMock.mockReset()
    delete http.defaults.adapter
  })

  it('401 响应清除本地凭据并跳转 /login', async () => {
    localStorage.setItem('token', 'old-token')
    // 通过自定义 adapter 构造 401 响应，走完整拦截器链
    const err = new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 401,
      data: { code: 401, message: '会话凭据无效或已过期' },
      headers: {},
      config: {},
      statusText: 'Unauthorized',
    } as never)
    http.defaults.adapter = async () => { throw err }
    await expect(http.get('/auth/me')).rejects.toBeInstanceOf(ApiError)
    expect(localStorage.getItem('token')).toBeNull()
    expect(pushMock).toHaveBeenCalledWith('/login')
  })

  it('登录页自身的 401 不跳转', async () => {
    // 修改 router mock 的当前路径为 /login
    const { default: routerMock } = await import('@/router')
    ;(routerMock as { currentRoute: { value: { path: string } } }).currentRoute.value.path = '/login'
    const err = new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 401,
      data: { code: 401, message: '邮箱或密码错误' },
      headers: {},
      config: {},
      statusText: 'Unauthorized',
    } as never)
    http.defaults.adapter = async () => { throw err }
    await expect(http.post('/auth/login', {})).rejects.toBeInstanceOf(ApiError)
    expect(pushMock).not.toHaveBeenCalled()
  })
})
