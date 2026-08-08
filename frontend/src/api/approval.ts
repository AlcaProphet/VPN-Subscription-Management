// api/approval.ts：审批中心接口（Build3 Step 2）
import { http } from './request'

export interface PendingUser {
  id: number
  username: string
  email: string
  source: 'oidc' | 'selfreg'
  oidc_claims: string
  created_at: string // UTC
}

// 分页列表保留 {list,total} 包裹（调用方取 list/total，R02-01）
export const listApprovals = (q: { page: number; size: number }) =>
  http.get<any, { list: PendingUser[]; total: number }>('/admin/approvals', { params: q })
export const approve = (id: number) => http.post(`/admin/approvals/${id}/approve`)
export const reject = (id: number) => http.post(`/admin/approvals/${id}/reject`)
export const batchApproveApi = (ids: number[]) =>
  http.post<any, { succeeded: number; failed: number }>('/admin/approvals/batch_approve', { ids })
