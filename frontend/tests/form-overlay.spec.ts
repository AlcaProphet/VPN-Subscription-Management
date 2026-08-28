// form-overlay.spec.ts：桌面弹窗与手机全屏抽屉的统一表单载体（Build11 Step 4）
import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import FormOverlay from '@/components/FormOverlay.vue'

function setViewport(isMobile: boolean) {
  const listeners = new Set<(event: MediaQueryListEvent) => void>()
  const mediaQuery = {
    matches: isMobile,
    media: '(max-width: 767px)',
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn((type: string, listener: (event: MediaQueryListEvent) => void) => {
      if (type === 'change') listeners.add(listener)
    }),
    removeEventListener: vi.fn((type: string, listener: (event: MediaQueryListEvent) => void) => {
      if (type === 'change') listeners.delete(listener)
    }),
    dispatchEvent: vi.fn(),
  }
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockImplementation(() => mediaQuery),
  })
  return {
    switchTo(matches: boolean) {
      mediaQuery.matches = matches
      listeners.forEach((listener) => listener({ matches } as MediaQueryListEvent))
    },
  }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('FormOverlay', () => {
  it('桌面渲染 Modal，并提供可访问的关闭按钮与默认底部操作', async () => {
    setViewport(false)
    const wrapper = mount(FormOverlay, {
      attachTo: document.body,
      props: { open: true, title: '编辑节点', width: 720 },
      slots: { default: '<input aria-label="节点名称" />' },
    })
    await flushPromises()

    expect(document.body.querySelector('.ant-modal')).not.toBeNull()
    expect(document.body.querySelector('.ant-drawer')).toBeNull()
    expect(document.body.querySelector('button[aria-label="关闭编辑节点"]')).not.toBeNull()
    const footerLabels = Array.from(document.body.querySelectorAll('.form-overlay__footer button'))
      .map((button) => button.textContent?.replace(/\s/g, ''))
    expect(footerLabels).toContain('取消')
    expect(footerLabels).toContain('保存')
    wrapper.unmount()
  })

  it('手机渲染底部全屏 Drawer，并保留固定底部操作', async () => {
    setViewport(true)
    const wrapper = mount(FormOverlay, {
      attachTo: document.body,
      props: { open: true, title: '新建订阅', loading: true },
      slots: { default: '<input aria-label="订阅名称" />' },
    })
    await flushPromises()

    expect(document.body.querySelector('.ant-modal')).toBeNull()
    expect(document.body.querySelector('.ant-drawer')).not.toBeNull()
    expect(document.body.querySelector('.form-overlay-drawer')).not.toBeNull()
    expect(document.body.querySelector('.form-overlay__footer')).not.toBeNull()
    expect(document.body.querySelector('button[aria-label="关闭新建订阅"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('在断点切换时同步切换 Modal 与 Drawer', async () => {
    const viewport = setViewport(false)
    const wrapper = mount(FormOverlay, {
      attachTo: document.body,
      props: { open: true, title: '编辑规则' },
    })
    await flushPromises()
    expect(document.body.querySelector('.ant-modal')).not.toBeNull()

    viewport.switchTo(true)
    await flushPromises()
    expect(document.body.querySelector('.ant-modal')).toBeNull()
    expect(document.body.querySelector('.ant-drawer')).not.toBeNull()

    viewport.switchTo(false)
    await flushPromises()
    expect(document.body.querySelector('.ant-drawer')).toBeNull()
    expect(document.body.querySelector('.ant-modal')).not.toBeNull()
    wrapper.unmount()
  })
})
