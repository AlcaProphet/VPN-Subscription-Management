// 测试全局 setup：为 jsdom 补齐 AntD 依赖的浏览器 API
import { vi } from 'vitest'

// AntD responsiveObserve 依赖 matchMedia
if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(), // 旧 API 兼容
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

// getComputedStyle 完整实现（AntD 部分组件依赖）
if (!window.getComputedStyle) {
  Object.defineProperty(window, 'getComputedStyle', { value: () => ({}) })
}
