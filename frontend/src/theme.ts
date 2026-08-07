// 主题与暗色模式：AntD ConfigProvider 全局中文 + 主色 + darkAlgorithm 联动（UI §1.1/1.2）
import { computed, ref, watch } from 'vue'
import { theme as antTheme } from 'ant-design-vue'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

export const antdLocale = zhCN // ConfigProvider 全局中文
export const primaryColor = '#1677FF' // AntD 默认科技蓝，零定制（UI §1.1）

const STORAGE_KEY = 'theme'
const dark = ref(localStorage.getItem(STORAGE_KEY) === 'dark')

// useTheme：暗色切换 composable；localStorage 持久化 + document 类名联动 Tailwind
export function useTheme() {
  watch(dark, (v) => {
    localStorage.setItem(STORAGE_KEY, v ? 'dark' : 'light')
    document.documentElement.classList.toggle('dark', v)
  }, { immediate: true })
  return {
    dark,
    toggle: () => { dark.value = !dark.value },
    antdTheme: computed(() => ({
      algorithm: dark.value ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
      token: { colorPrimary: primaryColor },
    })),
  }
}
