// api/auth.ts：认证接口封装
import { http } from './request'

export interface LoginResult {
  token: string
  expires_at: number
  user: UserInfo
}
export interface RegisterResult {
  token?: string
  status: 'active' | 'pending'
  is_admin?: boolean
  message?: string
}

export const register = (data: { username: string; email: string; password: string; captcha_token?: string }) =>
  http.post<any, RegisterResult>('/auth/register', data)
export const login = (data: { email: string; password: string; remember: boolean; captcha_token?: string }) =>
  http.post<any, LoginResult>('/auth/login', data)
export const me = () => http.get<any, UserInfo>('/auth/me')
export const logout = () => http.post('/auth/logout')
// Step 7：忘记/重置密码
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const forgot = (data: { email: string; captcha_token?: string }) =>
  http.post<any, { message: string }>('/auth/forgot', data)
export const resetPassword = (data: { token: string; password: string }) =>
  http.post<any, { message: string }>('/auth/reset', data)

export type ResetTokenStatus = 'missing' | 'used' | 'expired' | 'valid'
export const validateResetToken = (token: string) =>
  http.post<any, { status: ResetTokenStatus }>('/auth/reset/validate', { token })
