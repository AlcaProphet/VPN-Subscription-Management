// overlay-manager.spec.ts：Build12 Step 2 全局浮层管理单测
import { describe, expect, it, beforeEach, vi } from 'vitest'
import {
  registerOverlay,
  closeTopOverlay,
  getActiveOverlay,
  nextOverlayId,
  unregisterOverlay,
} from '@/utils/overlayManager'

describe('overlayManager', () => {
  beforeEach(() => {
    // 清理当前测试进程中可能残留的注册项
    while (getActiveOverlay()) closeTopOverlay()
  })

  it('非模态浮层唯一：打开第二个时自动关闭第一个', () => {
    const closed: string[] = []
    const un1 = registerOverlay({
      id: 'a',
      type: 'dropdown',
      close: () => closed.push('a'),
    })
    registerOverlay({
      id: 'b',
      type: 'dropdown',
      close: () => closed.push('b'),
    })
    expect(closed).toContain('a')
    expect(getActiveOverlay()?.id).toBe('b')
    un1()
  })

  it('Modal 允许层叠', () => {
    registerOverlay({ id: 'm1', type: 'modal', close: () => {} })
    registerOverlay({ id: 'm2', type: 'modal', close: () => {} })
    expect(getActiveOverlay()?.id).toBe('m2')
  })

  it('closeTop 只关闭栈顶', () => {
    const closed: string[] = []
    registerOverlay({ id: 'm1', type: 'modal', close: () => closed.push('m1') })
    registerOverlay({ id: 'm2', type: 'modal', close: () => closed.push('m2') })
    expect(closeTopOverlay()).toBe(true)
    expect(closed).toEqual(['m2'])
    expect(getActiveOverlay()?.id).toBe('m1')
  })

  it('Esc 关闭当前浮层并回焦', () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    const focusSpy = vi.spyOn(trigger, 'focus')
    trigger.focus()
    registerOverlay({
      id: 'esc',
      type: 'select',
      close: () => {},
      focusTrigger: () => trigger.focus(),
    })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(getActiveOverlay()).toBeNull()
    expect(focusSpy).toHaveBeenCalled()
  })

  it('nextOverlayId 生成唯一 id', () => {
    const a = nextOverlayId('x')
    const b = nextOverlayId('x')
    expect(a).not.toBe(b)
  })

  it('unregister 后不再位于活动栈', () => {
    const un = registerOverlay({ id: 'u1', type: 'popover', close: () => {} })
    un()
    expect(getActiveOverlay()).toBeNull()
  })

  it('unregisterOverlay 支持直接按 id 移除', () => {
    registerOverlay({ id: 'u2', type: 'picker', close: () => {} })
    unregisterOverlay('u2')
    expect(getActiveOverlay()).toBeNull()
  })
})
