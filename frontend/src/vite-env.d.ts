/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// 全局类型声明：用户信息与系统状态
interface UserInfo {
  id: number
  username: string
  email: string
  role: 'admin' | 'user'
  group_id: number | null
  group_name?: string // me 接口附带（顶栏所属组标签）
  status: string
  user_source: string
}

interface SystemStatus {
  configured: boolean
  app_mode: 'dev' | 'prod'
  emergency: boolean
  allow_local_login?: boolean
  allow_selfreg?: boolean
  user_table_empty?: boolean
  oidc_configured?: boolean
  oidc_provider_type?: string
  captcha_provider?: string
  captcha_site_key?: string
  captcha_pages?: string[]
}
