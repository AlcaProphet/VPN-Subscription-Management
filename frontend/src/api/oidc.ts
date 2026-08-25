// api/oidc.ts：OIDC 相关接口封装
import { http } from './request'

export const oidcTest = (data: { provider_type: string } & Record<string, string>) =>
  http.post<any, { ok: boolean; message: string; warnings: string[] }>('/setup/oidc/test', data)
export const setupOidc = (data: { provider_type: string } & Record<string, string>) =>
  http.post('/setup/oidc', data)
export const exchangeOidc = () =>
  http.post<any, { token: string; expires_at: number }>('/auth/oidc/exchange')
export const mockLogin = (data: { email: string; username?: string; email_verified: boolean; roles?: string[]; groups?: string[] }) =>
  http.post<any, { token?: string; expires_at?: number; status?: string; message?: string }>('/auth/oidc/mock/login', data)
export const startBind = () => http.post<any, { auth_url: string }>('/auth/oidc/bind')
