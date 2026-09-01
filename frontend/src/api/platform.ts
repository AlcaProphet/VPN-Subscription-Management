// api/platform.ts：平台管理接口封装
import { http } from './request'
import type { ProductType } from './subscription'

export interface CascadeCounts {
  subscriptions: number
  tokens: number
  customs: number
}

// 本地安装包条目：name=原始文件名（展示用），file=磁盘文件名（时间戳，删除定位用）
export interface InstallerFileItem {
  name: string
  file: string
}

// 外部下载链接条目：name=展示名（可空），url=下载地址
export interface InstallerURLItem {
  name: string
  url: string
}

export interface PlatformItem {
  id: number
  slug: string
  name: string
  description: string
  product_type: ProductType
  is_default?: boolean
  schemes: string[]
  extra_headers: Record<string, string>
  installer_files: InstallerFileItem[]
  installer_urls: InstallerURLItem[]
  cascade: CascadeCounts // 删除预览影响统计
}

export const listPlatforms = () =>
  http.get<any, { list: PlatformItem[]; total: number }>('/admin/platforms').then((d) => d.list)
export const getPlatform = (id: number) => http.get<any, PlatformItem>(`/admin/platforms/${id}`)
export const createPlatform = (data: Partial<PlatformItem>) =>
  http.post<any, PlatformItem>('/admin/platforms', data)
export const updatePlatform = (id: number, data: Partial<PlatformItem>) =>
  http.put(`/admin/platforms/${id}`, data)
export const deletePlatform = (id: number) => http.delete(`/admin/platforms/${id}`)
// 追加上传安装包：返回更新后的本地安装包列表
export const uploadInstaller = (id: number, file: File, onProgress?: (pct: number) => void) => {
  const form = new FormData()
  form.append('file', file)
  return http
    .post<any, { list: InstallerFileItem[]; total: number }>(`/admin/platforms/${id}/installers`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
    .then((d) => d.list)
}
// 按磁盘文件名删除单个安装包
export const deleteInstallerFile = (id: number, file: string) =>
  http.delete(`/admin/platforms/${id}/installers/${encodeURIComponent(file)}`)
