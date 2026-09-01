// 测试全局 setup：为 jsdom 补齐 AntD 依赖的浏览器 API
import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import AppSelect from '@/components/AppSelect.vue'

config.global.components = { AppSelect }

// Node 26 + 当前 jsdom 组合下 localStorage 可能为 undefined：无条件补齐内存实现（仅测试环境）
const store = new Map<string, string>()
const localStorageMock: Storage = {
  get length() { return store.size },
  clear: () => store.clear(),
  getItem: (key: string) => (store.has(key) ? store.get(key)! : null),
  key: (index: number) => Array.from(store.keys())[index] ?? null,
  removeItem: (key: string) => { store.delete(key) },
  setItem: (key: string, value: string) => { store.set(key, String(value)) },
}
Object.defineProperty(window, 'localStorage', { value: localStorageMock, writable: true, configurable: true })
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true, configurable: true })

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

// jsdom 对 getComputedStyle(element, pseudoElement) 会输出「Not implemented」；
// AntD 仅以伪元素参数探测滚动条尺寸，忽略该参数即可获得等价的元素样式。
const nativeGetComputedStyle = window.getComputedStyle.bind(window)
Object.defineProperty(window, 'getComputedStyle', {
  value: (element: Element) => nativeGetComputedStyle(element),
  writable: true,
  configurable: true,
})
