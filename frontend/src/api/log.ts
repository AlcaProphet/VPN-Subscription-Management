// api/log.ts：日志接口（Build3 Step 5）——访问日志查询/清空 + 实时日志流 SSE（R28-07I fetch/ReadableStream）
import { http } from './request'

export interface AccessLog {
  id: number
  user_id: number
  username: string
  user_email: string
  ip: string
  download_type: string
  platform: string
  platform_name: string
  resource_slug: string
  resource_name: string
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
// openLogStream 用 fetch 携带现有 Bearer 会话凭据连接管理员 SSE 路由；不再换取/传递查询 Token。
export async function openLogStream(signal?: AbortSignal): Promise<Response> {
  const token = localStorage.getItem('token')
  return fetch('/api/admin/logs/stream', {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    cache: 'no-store',
    signal,
  })
}
