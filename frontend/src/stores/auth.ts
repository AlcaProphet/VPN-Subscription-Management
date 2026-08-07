// 认证状态：凭据存取（localStorage 键 token）+ 登录/注册/登出 action
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login, logout, register } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') ?? '')
  const user = ref<UserInfo | null>(null)

  // setSession：登录/注册成功写入
  function setSession(t: string, u?: UserInfo) {
    token.value = t
    localStorage.setItem('token', t)
    if (u) user.value = u
  }
  // logout：清除本地凭据（退出为客户端语义，Design1 §5.4）
  function logoutLocal() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }
  // loginAction：登录成功存 token 与用户信息
  async function loginAction(form: { email: string; password: string; remember: boolean }) {
    const data = await login(form)
    setSession(data.token, data.user)
  }
  // registerAction：注册成功（直接激活时 token 已存）
  async function registerAction(form: { username: string; email: string; password: string }) {
    const data = await register(form)
    if (data.token) setSession(data.token) // 直接激活
    return data
  }
  // logoutAction：退出为客户端语义，接口失败不阻断
  async function logoutAction() {
    try { await logout() } catch { /* 退出为客户端语义，接口失败不阻断 */ }
    logoutLocal()
  }
  return { token, user, setSession, logout: logoutLocal, loginAction, registerAction, logoutAction }
})
