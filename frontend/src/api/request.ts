// api/request.ts：Axios 实例 + Bearer 注入 + 401 拦截 + 错误码→UI 映射（UI §7.3）
import axios, { AxiosError } from 'axios'
import { message } from 'ant-design-vue'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

export const http = axios.create({ baseURL: '/api', timeout: 15000 })

// 请求拦截：自动携带会话凭据
http.interceptors.request.use((cfg) => {
  const token = localStorage.getItem('token')
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

// 响应拦截：解包统一响应结构 + 401 清凭据跳登录
http.interceptors.response.use(
  (resp) => {
    // 非 JSON 响应（预览类 text/plain、blob 下载等）原样返回，跳过统一包裹解包（Issue1 R04-01）
    if (resp.config.responseType && resp.config.responseType !== 'json') {
      return resp.data
    }
    const body = resp.data
    if (body && typeof body.code === 'number' && body.code !== 0) {
      return Promise.reject(new ApiError(body.code, body.message ?? '请求失败'))
    }
    return body.data // 调用方直接拿到 data
  },
  (err: AxiosError<{ code: number; message: string }>) => {
    const st = err.response?.status ?? 0
    const msg = err.response?.data?.message
    if (st === 401) {
      if (import.meta.env.DEV) console.warn('[request] 401:', err.config?.url) // 诊断日志（R07-08）
      const auth = useAuthStore()
      auth.logout()
      // 登录页自身的 401（密码错误）不跳转，由页面展示统一措辞
      if (router.currentRoute.value.path !== '/login') {
        void router.push('/login')
      }
    }
    return Promise.reject(new ApiError(st, msg ?? defaultMsg(st)))
  },
)

function defaultMsg(st: number): string {
  switch (st) {
    case 400: return '输入校验失败，请检查表单'
    case 403: return '权限不足'
    case 409: return '数据冲突，请刷新后重试'
    case 429: return '操作过于频繁，请稍后再试'
    case 500: return '服务器内部错误'
    default: return '网络异常，请重试'
  }
}

// ApiError：携带 HTTP 状态码，供页面区分「表单定位」与「全局提示」
export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

// handleApiError：错误码 → UI 映射统一入口（UI §7.3）
// 400：页面优先做表单定位/message.error；403/409：message.error；
// 429：message.warning（有 Retry-After 时提示等待秒数）；500：通用文案
export function handleApiError(err: unknown, fallback?: () => void) {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 429: {
        const retryAfter = (err as unknown as { retryAfter?: string }).retryAfter
        if (retryAfter) {
          message.warning(`操作过于频繁，请 ${retryAfter} 秒后重试`)
        } else {
          message.warning('操作过于频繁，请稍后再试')
        }
        return
      }
      case 400:
      case 403:
      case 409:
        message.error(err.message)
        return
      case 500:
        message.error('服务器内部错误')
        return
    }
  }
  message.error(err instanceof Error ? err.message : '网络异常，请重试')
  fallback?.()
}

// --- pollTask：异步任务轮询封装（Design2-UI §9.2） ---

export class PollTimeoutError extends Error {
  constructor(message = '任务仍在后台执行，请稍后刷新查看结果') { super(message) }
}
export class PollNetworkError extends Error {
  constructor(message = '状态查询失败，点击重试') { super(message) }
}

export interface PollTaskOptions<T> {
  submit: () => Promise<unknown>
  query: () => Promise<T>
  isDone: (r: T) => boolean
  interval?: number
  timeoutMs?: number
  signal?: AbortSignal
}

export interface PollTaskHandle<T> {
  run: () => Promise<T>
  cancel: () => void
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(new PollTimeoutError('轮询已取消')); return }
    const timer = window.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    const onAbort = () => {
      window.clearTimeout(timer)
      reject(new PollTimeoutError('轮询已取消'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

// pollTask 提交任务后按 interval 轮询 query；终态判定由 isDone 提供；
// 单次网络失败容忍（连续 3 次才报错）；超时抛 PollTimeoutError。
// 返回 { run, cancel }：run 执行提交+轮询，cancel 仅停止前端轮询（后端任务继续执行）。
export function pollTask<T>(opts: PollTaskOptions<T>): PollTaskHandle<T> {
  let aborted = false
  return {
    async run() {
      await opts.submit()
      const deadline = Date.now() + (opts.timeoutMs ?? 5 * 60 * 1000)
      let failures = 0
      while (Date.now() < deadline) {
        if (aborted || opts.signal?.aborted) throw new PollTimeoutError('轮询已取消')
        try {
          const r = await opts.query()
          failures = 0
          if (opts.isDone(r)) return r
        } catch {
          failures += 1
          if (failures >= 3) throw new PollNetworkError()
        }
        try {
          await sleep(opts.interval ?? 1500, opts.signal)
        } catch {
          if (aborted || opts.signal?.aborted) throw new PollTimeoutError('轮询已取消')
          throw new PollTimeoutError()
        }
      }
      throw new PollTimeoutError()
    },
    cancel() { aborted = true },
  }
}
