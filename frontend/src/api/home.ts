// api/home.ts：用户端首页数据接口封装
import { http } from './request'

export interface AdminPoolSub {
  id: number
  name: string
  slug: string
  current_version: number
  token: string
  download_url: string
}

// 首页安装包条目（本地与外部统一为 {name, url}，下拉菜单直接打开）
export interface InstallerEntry {
  name: string // 本地=原始文件名 / 外部=展示名（可空）
  url: string
}

export interface PlatformCard {
  platform_id: number
  name: string
  description: string
  schemes: string[] // 含 {url} 占位符；一键导入取首项
  installer_files: InstallerEntry[] // 本地安装包（url 已拼为 /public/installers/ 公开路径）
  installer_urls: InstallerEntry[] // 外部下载链接
  status: 'group_selected' | 'custom' | 'unassigned' | 'admin_pool'
  download_token: string
  download_url: string // {frontend_url}/subscriptions/{平台标识}/download?token=（R10-10 完整 URL）
  subscription_name?: string
  subscriptions?: AdminPoolSub[] // 管理员池内订阅列表
}

export const homePlatforms = () =>
  http.get<any, { list: PlatformCard[]; total: number }>('/home/platforms').then((d) => d.list)
export const refreshHomeToken = (platformId: number) =>
  http.post<any, { token: string }>('/home/token/refresh', { platform_id: platformId })
export const homeUpdatedAt = () => http.get<any, { updated_at: string | null }>('/home/updated_at')
