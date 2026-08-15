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
    create: (ownerId: number, payload: FormData | { text: string }) => {
      // 文本模式必须带 ?mode=text（后端按查询参数区分文件/文本双模式）；FormData 为文件上传
      const isText = !(payload instanceof FormData)
      return http.post<any, { version_no: number }>(
        `${prefix}/${ownerId}/versions${isText ? '?mode=text' : ''}`,
        payload,
      )
    },
    switchCurrent: (ownerId: number, versionNo: number) =>
      http.put(`${prefix}/${ownerId}/versions/current`, { version_no: versionNo }),
    preview: (ownerId: number, ver: number) =>
      http.get<any, string>(`${prefix}/${ownerId}/versions/${ver}/preview`, { responseType: 'text' }),
    remove: (ownerId: number, ver: number) => http.delete(`${prefix}/${ownerId}/versions/${ver}`),
  }
}
