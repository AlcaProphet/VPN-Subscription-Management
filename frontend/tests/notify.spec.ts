// notify.spec.ts：Notify 2 秒内相同提示去重
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { message, notification } from 'ant-design-vue'
import { Notify } from '@/components/Notify'

describe('Notify 去重', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('相同 success 在 2 秒内只显示一次', () => {
    const spy = vi.spyOn(message, 'success').mockImplementation(() => undefined as any)
    Notify.success('相同提示')
    Notify.success('相同提示')
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('不同内容不会被去重', () => {
    const spy = vi.spyOn(message, 'success').mockImplementation(() => undefined as any)
    Notify.success('提示 A')
    Notify.success('提示 B')
    expect(spy).toHaveBeenCalledTimes(2)
  })

  it('detail 通知同样去重', () => {
    const spy = vi.spyOn(notification, 'error').mockImplementation(() => undefined as any)
    Notify.detail('标题', '描述')
    Notify.detail('标题', '描述')
    expect(spy).toHaveBeenCalledTimes(1)
  })
})
