// api/system.ts：公开系统状态（守卫专用，不携带 Bearer）
import axios from 'axios'

export async function getSystemStatus(): Promise<SystemStatus> {
  const resp = await axios.get('/api/system/status') // 独立实例，不走拦截器
  return resp.data.data
}
