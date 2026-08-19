// api/home.ts：用户端首页数据接口封装
import { http } from './request'

export interface TrafficSummary {
  unlimited: boolean // 基础模式恒 true；高级模式配额不限时亦 true 且 quota_bytes=null
  used_bytes?: number
  quota_bytes?: number | null // 不限流量时为 null
  exceeded?: boolean
}

export interface HomeRuleSummary {
  rule_id: number
  name: string
  current_version: number
  token: string
  download_url: string
}

export interface HomeSummary {
  traffic: TrafficSummary
  home_rule: HomeRuleSummary | null
}

// 首页安装包条目（本地与外部统一为 {name, url}，下拉菜单直接打开）
export interface InstallerEntry {
  name: string // 本地=原始文件名 / 外部=展示名（可空）
  url: string
}

// 管理员平台卡片的订阅预览信息（每平台唯一）
export interface AdminPreviewSubscription {
  name: string
  product_type: 'yaml' | 'subs' | 'generic-subs'
  content_kind: 'blueprint' | 'upload' | ''
  current_version: number
  version_updated_at?: string | null
}

export interface PlatformCard {
  platform_id: number
  slug: string
  name: string
  description: string
  schemes: string[] // 含 {url} 占位符；一键导入取首项
  installer_files: InstallerEntry[] // 本地安装包（url 已拼为 /public/installers/ 公开路径）
  installer_urls: InstallerEntry[] // 外部下载链接
  status: 'custom' | 'unassigned' | 'ready' | 'admin_preview'
  download_token?: string
  download_url?: string // {frontend_url}/subscriptions/{平台标识}/download?token=
  subscription_name?: string
  subscription_product_type?: 'yaml' | 'subs' | 'generic-subs'
  version_updated_at?: string | null
  subscription?: AdminPreviewSubscription | null // 管理员预览形态
  preview_available: boolean
}

export const homePlatforms = () =>
  http.get<any, { list: PlatformCard[]; total: number }>('/home/platforms').then((d) => d.list)
export const getHomeSummary = () => http.get<any, HomeSummary>('/home/summary')
export const refreshHomeToken = (platformId: number) =>
  http.post<any, { token: string }>('/home/token/refresh', { platform_id: platformId })
export const homeUpdatedAt = () => http.get<any, { updated_at: string | null }>('/home/updated_at')
