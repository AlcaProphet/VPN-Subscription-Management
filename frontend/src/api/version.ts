// api/version.ts：通用版本接口封装（四类资源复用，前缀参数化，UI §5.1/7.1）
import { http } from './request'

export interface VersionItem {
  version_no: number
  file_path: string
  current: boolean
  created_at: string
  updated_at: string
}

export function versionApi(prefix: string) {
  return {
    list: (ownerId: number) =>
      http.get<any, { list: VersionItem[]; total: number }>(`${prefix}/${ownerId}/versions`).then((d) => d.list),
    create: (ownerId: number, payload: FormData | { text: string }) =>
      http.post<any, { version_no: number; yaml_warning?: string }>(`${prefix}/${ownerId}/versions`, payload),
    switchCurrent: (ownerId: number, versionNo: number) =>
      http.put(`${prefix}/${ownerId}/versions/current`, { version_no: versionNo }),
    preview: (ownerId: number, ver: number) =>
      http.get<any, string>(`${prefix}/${ownerId}/versions/${ver}/preview`, { responseType: 'text' }),
    remove: (ownerId: number, ver: number) => http.delete(`${prefix}/${ownerId}/versions/${ver}`),
  }
}
