// Notify.ts：message/notification 统一封装（全项目禁止直接调 message.*）
// 相同 Toast 2 秒内去重，避免重复操作时连续弹出相同提示。
import { message, notification } from 'ant-design-vue'

const lastShown = new Map<string, number>()
const DEDUP_MS = 2000

function shouldShow(key: string): boolean {
  const now = Date.now()
  const last = lastShown.get(key) ?? 0
  if (now - last < DEDUP_MS) return false
  lastShown.set(key, now)
  return true
}

function show(type: 'success' | 'error' | 'warning' | 'info', msg: string) {
  if (!shouldShow(`${type}:${msg}`)) return
  if (type === 'success') message.success(msg)
  else if (type === 'error') message.error(msg)
  else if (type === 'warning') message.warning(msg)
  else message.info(msg)
}

export const Notify = {
  success: (msg: string) => show('success', msg),
  error: (msg: string) => show('error', msg),
  warning: (msg: string) => show('warning', msg),
  info: (msg: string) => show('info', msg),
  detail: (title: string, desc: string) => {
    if (!shouldShow(`detail:${title}:${desc}`)) return
    notification.error({ message: title, description: desc })
  },
}
