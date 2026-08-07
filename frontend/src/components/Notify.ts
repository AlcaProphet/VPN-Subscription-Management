// Notify.ts：message/notification 统一封装（全项目禁止直接调 message.*）
import { message, notification } from 'ant-design-vue'

export const Notify = {
  success: (msg: string) => message.success(msg),
  error: (msg: string) => message.error(msg),
  warning: (msg: string) => message.warning(msg),
  info: (msg: string) => message.info(msg),
  detail: (title: string, desc: string) => notification.error({ message: title, description: desc }),
}
