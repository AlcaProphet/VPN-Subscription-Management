// api/platform.ts：平台管理接口封装
import { http } from './request'

export interface CascadeCounts {
  subscriptions: number
  tokens: number
  customs: number
}

export interface PlatformItem {
  id: number
  slug: string
  name: string
  description: string
  schemes: string[]
  extra_headers: Record<string, string>
  installer_file: string
  installer_url: string
  cascade: CascadeCounts // 删除预览影响统计
}

export const listPlatforms = () => http.get<any, PlatformItem[]>('/admin/platforms')
export const getPlatform = (id: number) => http.get<any, PlatformItem>(`/admin/platforms/${id}`)
export const createPlatform = (data: Partial<PlatformItem>) =>
  http.post<any, PlatformItem>('/admin/platforms', data)
export const updatePlatform = (id: number, data: Partial<PlatformItem>) =>
  http.put(`/admin/platforms/${id}`, data)
export const deletePlatform = (id: number) => http.delete(`/admin/platforms/${id}`)
export const uploadInstaller = (id: number, file: File, onProgress?: (pct: number) => void) => {
  const form = new FormData()
  form.append('file', file)
  return http.post(`/admin/platforms/${id}/installer`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (e) => {
      if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
    },
  })
}
export const deleteInstaller = (id: number) => http.delete(`/admin/platforms/${id}/installer`)
