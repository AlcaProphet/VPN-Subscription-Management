// 系统状态：缓存 /api/system/status（守卫与页面共用）
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getSystemStatus } from '@/api/system'

export const useSystemStore = defineStore('system', () => {
  const status = ref<SystemStatus | null>(null)
  async function fetchStatus(force = false) {
    if (status.value && !force) return status.value
    status.value = await getSystemStatus()
    return status.value
  }
  return { status, fetchStatus }
})
