// api/log.ts：日志接口（Build3 Step 5）——访问日志查询/清空 + 实时日志流 SSE 短期 Token
import { http } from './request'

export interface AccessLog {
  id: number
  user_id: number
  username: string
  ip: string
  download_type: string
  platform: string
  resource_slug: string
  status: 'success' | 'fail'
  fail_reason: string
  created_at: string
}

export interface LogEntry {
  time: string
  level: string
  message: string
  attrs: string
}

// 分页列表保留 {list,total} 包裹（调用方取 list/total，R02-01）
export const queryAccessLogs = (q: { from: string; to: string; page: number; size: number }) =>
  http.get<any, { list: AccessLog[]; total: number }>('/admin/logs/access', { params: q })
export const clearAccessLogs = () => http.post('/admin/logs/access/clear')
export const issueStreamToken = () => http.post<any, { token: string }>('/admin/logs/stream/token')
