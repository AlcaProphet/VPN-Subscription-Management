// 系统状态：缓存 /api/system/status（守卫与页面共用）
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getSystemStatus } from '@/api/system'
import { siteInfoPublic, type SiteInfo } from '@/api/settings'

// 站点名称默认文案（Design1 §3.4.8：未设置时浏览器标题/登录页/首页展示默认标题）
const DEFAULT_SITE_NAME = 'VPN 订阅管理'

export const useSystemStore = defineStore('system', () => {
  const status = ref<SystemStatus | null>(null)
  async function fetchStatus(force = false) {
    if (status.value && !force) return status.value
    status.value = await getSystemStatus()
    return status.value
  }
  // 站点信息（公开端点，无需鉴权；浏览器标题/登录页/首页顶栏三处共用）
  const site = ref<SiteInfo | null>(null)
  async function fetchSiteInfo(force = false) {
    if (site.value && !force) return site.value
    site.value = await siteInfoPublic()
    return site.value
  }
  const siteName = computed(() => site.value?.site_name?.trim() || DEFAULT_SITE_NAME)
  const siteIconUrl = computed(() => site.value?.icon_url ?? '')
  return { status, fetchStatus, site, fetchSiteInfo, siteName, siteIconUrl }
})
