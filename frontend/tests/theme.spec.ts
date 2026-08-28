// theme.spec.ts：暗色切换后 localStorage 持久化 + document 类名联动 + Token/CSS 变量
import { describe, expect, it, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { theme as antTheme } from 'ant-design-vue'
import { useTheme, uiTokens } from '@/theme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.removeAttribute('style')
  })

  it('uiTokens 包含明暗双主题关键色值', () => {
    expect(uiTokens.light.primary).toBe('#2563EB')
    expect(uiTokens.dark.primary).toBe('#60A5FA')
    expect(uiTokens.light.surface).toBe('#FFFFFF')
    expect(uiTokens.dark.surface).toBe('#111827')
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

  it('antdTheme 使用具体色值而非 CSS 变量，CSS 变量随主题写入根节点', async () => {
    const { toggle, antdTheme } = useTheme()
    expect(antdTheme.value.token.colorPrimary).not.toContain('var(')
    expect(document.documentElement.style.getPropertyValue('--ui-primary')).toBe(uiTokens.light.primary)

    toggle()
    await nextTick()
    expect(document.documentElement.style.getPropertyValue('--ui-primary')).toBe(uiTokens.dark.primary)
    expect(document.documentElement.style.getPropertyValue('--ui-surface')).toBe(uiTokens.dark.surface)
    expect(antdTheme.value.token.colorPrimary).toBe(uiTokens.dark.primary)
  })
})
