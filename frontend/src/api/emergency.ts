// api/emergency.ts：应急恢复端点（Build3 Step 6）——独立 axios 实例，不走 401 拦截器（应急模式无会话）
import axios from 'axios'

const emergencyHttp = axios.create({ baseURL: '/api/emergency' })

export interface AdminOption {
  id: number
  username: string
  email: string
  has_password: boolean
}

// 解包统一响应结构（应急模式无会话，直接取 data.data）
interface Envelope<T> {
  code: number
  message?: string
  data: T
}
function unwrap<T>(resp: { data: Envelope<T> }): T {
  if (resp.data.code !== 0) {
    throw new Error(resp.data.message || '请求失败')
  }
  return resp.data.data
}

export const emergencyVerify = (data: { op_code: string }) =>
  emergencyHttp.post<any, { data: Envelope<{ can_reset_password: boolean; admins?: AdminOption[] }> }>('/verify', data)
    .then(unwrap)
export const emergencyResetPassword = (data: { op_code: string; user_id: number; new_password: string }) =>
  emergencyHttp.post('/reset_password', data)
export const emergencyReinitialize = (data: { op_code: string; confirm: string }) =>
  emergencyHttp.post('/reinitialize', data)
