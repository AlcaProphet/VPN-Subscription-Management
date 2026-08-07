// theme.spec.ts：暗色切换后 localStorage 持久化 + document 类名联动
import { describe, expect, it, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { theme as antTheme } from 'ant-design-vue'
import { useTheme } from '@/theme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  it('toggle 后 localStorage 键值翻转', async () => {
    const { dark, toggle } = useTheme()
    expect(dark.value).toBe(false)
    toggle()
    await nextTick() // watch 回调异步执行
    expect(dark.value).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    toggle()
    await nextTick()
    expect(localStorage.getItem('theme')).toBe('light')
  })

  it('document 根元素带 dark 类且 antdTheme 切换算法', async () => {
    const { toggle, antdTheme } = useTheme()
    toggle()
    await nextTick()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(antdTheme.value.algorithm).toBe(antTheme.darkAlgorithm)
    toggle()
    await nextTick()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(antdTheme.value.algorithm).toBe(antTheme.defaultAlgorithm)
  })
})
