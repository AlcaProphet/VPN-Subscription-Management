// 主题与暗色模式：AntD 与 Tailwind 共用同一 uiTokens。
// AntD 使用具体色值（不能传 CSS 变量给 TinyColor/算法），Tailwind 通过 CSS 变量自动适配明暗。
import { computed, ref, watch } from 'vue'
import { theme as antTheme } from 'ant-design-vue'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

export const antdLocale = zhCN // ConfigProvider 全局中文

// 唯一 Token 源（Build11 Step 6；对比度已按 Build11 §0.1.2 预校验）
export const uiTokens = {
  light: {
    page: '#F1F5F9', surface: '#FFFFFF', surfaceSubtle: '#F8FAFC',
    surfaceRaised: '#FFFFFF', border: '#CBD5E1', borderSubtle: '#E2E8F0',
    text: '#0F172A', textSecondary: '#475569', textTertiary: '#64748B',
    primary: '#2563EB', primaryHover: '#1D4ED8', primarySoft: '#EFF6FF',
    success: '#15803D', successSoft: '#F0FDF4', warning: '#B45309',
    warningSoft: '#FFFBEB', danger: '#B91C1C', dangerSoft: '#FEF2F2',
    info: '#0369A1', infoSoft: '#F0F9FF',
  },
  dark: {
    page: '#0B1120', surface: '#111827', surfaceSubtle: '#172033',
    surfaceRaised: '#1E293B', border: '#334155', borderSubtle: '#273449',
    text: '#F8FAFC', textSecondary: '#CBD5E1', textTertiary: '#94A3B8',
    primary: '#60A5FA', primaryHover: '#93C5FD', primarySoft: '#172554',
    success: '#4ADE80', successSoft: '#052E16', warning: '#FBBF24',
    warningSoft: '#451A03', danger: '#F87171', dangerSoft: '#450A0A',
    info: '#38BDF8', infoSoft: '#082F49',
  },
  radius: { control: 8, card: 12, modal: 14 },
  spacing: { pageDesktop: 24, pageMobile: 16, section: 24, card: 20 },
  type: {
    pageTitle: { size: 24, lineHeight: 32, weight: 650 },
    sectionTitle: { size: 18, lineHeight: 26, weight: 600 },
    body: { size: 14, lineHeight: 22, weight: 400 },
    helper: { size: 13, lineHeight: 20, weight: 400 },
  },
} as const

type ThemeColors = { [K in keyof typeof uiTokens.light]: string }
const COLOR_KEYS = Object.keys(uiTokens.light) as Array<keyof ThemeColors>

function cssVarName(key: string): string {
  // page -> --ui-page，surfaceSubtle -> --ui-surface-subtle
  return `--ui-${key.replace(/[A-Z]/g, (m) => `-${m.toLowerCase()}`)}`
}

function applyColorVars(colors: ThemeColors) {
  const root = document.documentElement
  for (const key of COLOR_KEYS) {
    root.style.setProperty(cssVarName(key), colors[key])
  }
  root.style.colorScheme = colors.page === '#0B1120' ? 'dark' : 'light'
}

const STORAGE_KEY = 'theme'
const dark = ref(localStorage.getItem(STORAGE_KEY) === 'dark')

// useTheme：暗色切换 composable；localStorage 持久化 + document 类名联动 Tailwind
export function useTheme() {
  const tokens = computed(() => uiTokens[dark.value ? 'dark' : 'light'])
  watch(dark, (v) => {
    localStorage.setItem(STORAGE_KEY, v ? 'dark' : 'light')
    document.documentElement.classList.toggle('dark', v)
    applyColorVars(uiTokens[v ? 'dark' : 'light'])
  }, { immediate: true })
  return {
    dark,
    tokens,
    toggle: () => { dark.value = !dark.value },
    antdTheme: computed(() => {
      const t = tokens.value
      return {
        algorithm: dark.value ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
        token: {
          colorPrimary: t.primary,
          colorPrimaryHover: t.primaryHover,
          colorInfo: t.info,
          colorSuccess: t.success,
          colorWarning: t.warning,
          colorError: t.danger,
          colorText: t.text,
          colorTextSecondary: t.textSecondary,
          colorTextTertiary: t.textTertiary,
          colorBgContainer: t.surface,
          colorBgElevated: t.surfaceRaised,
          colorBgLayout: t.page,
          colorBorder: t.border,
          colorBorderSecondary: t.borderSubtle,
          borderRadius: uiTokens.radius.control,
          borderRadiusLG: uiTokens.radius.card,
        },
      }
    }),
  }
}
