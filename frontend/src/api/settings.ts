// api/settings.ts：面板配置接口（Build3 Step 3）——按分区独立 GET/PUT
import axios from 'axios'
import { http } from './request'

export interface OidcSettings {
  provider_type: string
  base_url: string
  realm: string
  client_id: string
  client_secret: string // GET 脱敏（***/""）；PUT 空=不修改
  frontend_url: string
  callback_url: string
}

export interface WhitelistConfig {
  role_claim_path: string
  role_values: string[]
  group_claim_path: string
  group_values: string[]
}

export interface LocalAuthSettings {
  allow_local_login: boolean
  allow_selfreg: boolean
  selfreg_approval: boolean
}

export interface CaptchaSettings {
  provider: 'off' | 'recaptcha' | 'turnstile'
  site_key: string
  secret_key: string
  pages: string[]
}

export interface SMTPSettings {
  host: string
  port: string
  user: string
  password: string
  from: string
  tls: boolean
  scopes: string[]
}

export interface RateLimitSettings {
  login: number
  register: number
  forgot: number
  download: number
  reset_validate: number
  http_read_header_timeout_sec: number
  http_read_timeout_sec: number
  http_write_timeout_sec: number
  http_idle_timeout_sec: number
  http_max_body_mb: number
}

export interface SiteInfo {
  site_name: string
  icon_url: string
}

export interface AdvancedSettings {
  advanced_mode: boolean
  collect_interval_minutes: number
  traffic_card_enabled: boolean
}

export const getAdvancedSettings = () => http.get<any, AdvancedSettings>('/admin/settings/advanced')
export const saveAdvancedSettings = (data: AdvancedSettings & { confirm_word?: string }) =>
  http.put<any, { task_id?: string; message?: string }>('/admin/settings/advanced', data)
export const getAdminTask = (id: string) => http.get<any, { id: string; kind: string; status: string; result?: unknown; error?: string }>(`/admin/tasks/${id}`)

export const getOidc = () => http.get<any, OidcSettings>('/admin/settings/oidc')
export const saveOidc = (data: OidcSettings) => http.put<any, { need_restart?: boolean }>('/admin/settings/oidc', data)
export const clearOidc = () => http.delete('/admin/settings/oidc')
export const testOidc = (data: Partial<OidcSettings>) => http.post<any, { ok: boolean; message: string; warnings: string[] }>(
  '/admin/settings/oidc/test', data,
)
export const getOidcRules = () => http.get<any, { approval_on: boolean; whitelist: WhitelistConfig }>('/admin/settings/oidc-rules')
export const saveOidcRules = (data: { approval_on: boolean; whitelist: WhitelistConfig }) =>
  http.put<any, { warning?: string }>('/admin/settings/oidc-rules', data)
export const getLocalAuth = () => http.get<any, LocalAuthSettings>('/admin/settings/local-auth')
export const saveLocalAuth = (data: LocalAuthSettings) => http.put('/admin/settings/local-auth', data)
export const getCaptcha = () => http.get<any, CaptchaSettings>('/admin/settings/captcha')
export const saveCaptcha = (data: CaptchaSettings) => http.put('/admin/settings/captcha', data)
export const getSMTP = () => http.get<any, SMTPSettings>('/admin/settings/smtp')
export const saveSMTP = (data: SMTPSettings) => http.put('/admin/settings/smtp', data)
export const testSMTP = () => http.post('/admin/settings/smtp/test')
export const getSite = () => http.get<any, SiteInfo>('/admin/settings/site')
export const saveSite = (form: FormData) => http.put<any, SiteInfo>('/admin/settings/site', form)
export const deleteSiteIcon = () => http.delete('/admin/settings/site/icon')
export const getRateLimit = () =>
  http.get<any, { settings: RateLimitSettings; trust_proxy: string; trust_proxy_cidrs: string }>('/admin/settings/ratelimit')
export const saveRateLimit = (data: RateLimitSettings) => http.put('/admin/settings/ratelimit', data)
export const getLogLevel = () => http.get<any, { level: string }>('/admin/settings/log-level')
export const saveLogLevel = (level: string) => http.put('/admin/settings/log-level', { level })
// 公告与页脚三份独立配置（R10-07）：首页公告 / 登录页公告 / 登录页页脚
export interface NoticeSettings {
  home_announcement: string
  login_announcement: string
  login_footer: string
}
export const getAnnouncement = () => http.get<any, NoticeSettings>('/admin/settings/announcement')
export const saveAnnouncement = (data: NoticeSettings) => http.put('/admin/settings/announcement', data)
export const getDebug = () => http.get<any, { on: boolean }>('/admin/settings/debug')
export const saveDebug = (on: boolean) => http.put('/admin/settings/debug', { on })
// 站点信息公开端点（无需鉴权，独立 axios 实例）
export const siteInfoPublic = () => axios.get<any, { data: { code: number; data: SiteInfo } }>('/api/site/info').then((r) => r.data.data)
// 公告/页脚公开端点（无需鉴权，未登录可获取——Design1 §3.3/§5.2；R07-02/R10-07）
export const getPublicAnnouncement = () =>
  axios.get<any, { data: { code: number; data: NoticeSettings } }>('/api/public/announcement').then((r) => r.data.data)

// --- 运维端点（Build3 Step 4：配置导入导出/备份下载/一键清空） ---
export const exportConfig = (password: string) =>
  http.post<any, Blob>('/admin/settings/export', { password }, { responseType: 'blob' })
export const importConfig = (form: FormData) =>
  http.post<any, { task_id?: string; message?: string }>('/admin/settings/import', form)
// Setup 导入（未配置状态暴露，无会话保护；依赖导出密码 + 按 IP 限流 5/min）
export const setupImportConfig = (form: FormData) => http.post('/setup/import', form)
export const clearAll = (confirmWord: string) => http.post('/admin/settings/clear_all', { confirm_word: confirmWord })
export const downloadBackup = () => http.get<any, Blob>('/admin/settings/backup', { responseType: 'blob' })
