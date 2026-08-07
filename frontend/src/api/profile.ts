// api/profile.ts：个人中心接口封装
import { http } from './request'

export const updateUsername = (username: string) => http.put('/profile/username', { username })
export const updateEmail = (email: string) => http.put<any, { message: string }>('/profile/email', { email })
export const updatePassword = (data: { current_password: string; new_password: string }) =>
  http.put<any, { message: string }>('/profile/password', data)
