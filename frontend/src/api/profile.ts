// api/profile.ts：个人中心接口封装
import { http } from './request'
import type { TrafficSummary } from './home'

export const updateUsername = (username: string) => http.put('/profile/username', { username })
export const updateEmail = (email: string) => http.put<any, { message: string }>('/profile/email', { email })
export const updatePassword = (data: { current_password: string; new_password: string }) =>
  http.put<any, { message: string }>('/profile/password', data)
export const getProfileTraffic = () => http.get<any, TrafficSummary>('/profile/traffic')
