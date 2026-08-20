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
  advanced_mode: boolean // Design2 高级模式开关（未加载按 off 处理）
  traffic_card_enabled?: boolean // 流量卡片开关（Build7）
  emergency: boolean
  emergency_reason?: 'manual' | 'db_corrupt' | 'key_missing' // Build3 Step 6：应急触发原因
  can_reset_password?: boolean // Build3 Step 6：应急模式可用能力
  allow_local_login?: boolean
  allow_selfreg?: boolean
  user_table_empty?: boolean
  oidc_configured?: boolean
  oidc_provider_type?: string
  captcha_provider?: string
  captcha_site_key?: string
  captcha_pages?: string[]
}
