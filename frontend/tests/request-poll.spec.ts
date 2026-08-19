// request-poll.spec.ts：pollTask 轮询封装（终态判定/取消/连续失败）
import { describe, expect, it, vi } from 'vitest'
import { pollTask, PollNetworkError } from '@/api/request'

describe('pollTask', () => {
  it('提交后轮询到终态返回结果', async () => {
    const submit = vi.fn().mockResolvedValue({ task_id: 1 })
    const query = vi.fn()
      .mockResolvedValueOnce({ status: 'running' })
      .mockResolvedValueOnce({ status: 'running' })
      .mockResolvedValue({ status: 'succeeded', result: 'ok' })
    const result = await pollTask<{ status: string; result?: string }>({
      submit,
      query,
      isDone: (r) => ['succeeded', 'failed', 'partial'].includes(r.status),
      interval: 1,
    }).run()
    expect(submit).toHaveBeenCalledTimes(1)
    expect(query).toHaveBeenCalledTimes(3)
    expect(result.result).toBe('ok')
  })

  it('cancel 后停止轮询并抛出取消错误', async () => {
    const submit = vi.fn().mockResolvedValue(undefined)
    const query = vi.fn().mockResolvedValue({ status: 'running' })
    const handle = pollTask<{ status: string }>({
      submit,
      query,
      isDone: () => false,
      interval: 10,
    })
    const p = handle.run()
    await new Promise((r) => setTimeout(r, 5))
    handle.cancel()
    await expect(p).rejects.toThrow('轮询已取消')
  })

  it('连续 3 次网络失败抛 PollNetworkError', async () => {
    const submit = vi.fn().mockResolvedValue(undefined)
    const query = vi.fn().mockRejectedValue(new Error('network'))
    await expect(pollTask<{ status: string }>({
      submit,
      query,
      isDone: () => false,
      interval: 1,
    }).run()).rejects.toBeInstanceOf(PollNetworkError)
  })
})
