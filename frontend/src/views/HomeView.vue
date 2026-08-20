<!-- HomeView.vue：用户首页（Design2-UI §3.1）——流量卡 → 分流规则卡 → 平台卡网格 → 公告卡 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Card, Dropdown, Empty, Modal, Progress, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import {
  homePlatforms,
  refreshHomeToken,
  homeUpdatedAt,
  getHomeSummary,
  type HomeRuleSummary,
  type PlatformCard,
  type TrafficSummary,
} from '@/api/home'
import { getPublicAnnouncement } from '@/api/settings'
import { previewSubscriptionByPlatform } from '@/api/subscription'
import { buildImportUrl } from '@/utils/importUrl'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import AppHeader from '@/components/AppHeader.vue'
import CopyField from '@/components/CopyField.vue'
import MarkdownView from '@/components/MarkdownView.vue'
import { me } from '@/api/auth'
import ConfirmModal from '@/components/ConfirmModal.vue'
import { Notify } from '@/components/Notify'

const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()

const loading = ref(true)
const cards = ref<PlatformCard[]>([])
const updatedAt = ref<string | null>(null)
const announcement = ref('')
const traffic = ref<TrafficSummary>({ unlimited: true })
const homeRule = ref<HomeRuleSummary | null>(null)

onMounted(async () => {
  try {
    if (!auth.user) auth.user = await me()
    const [c, u, s] = await Promise.all([homePlatforms(), homeUpdatedAt(), getHomeSummary()])
    cards.value = c
    updatedAt.value = u?.updated_at ?? null
    traffic.value = s.traffic
    homeRule.value = s.home_rule
    try {
      announcement.value = (await getPublicAnnouncement())?.home_announcement ?? ''
    } catch {
      announcement.value = ''
    }
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
  void system.fetchSiteInfo(true)
})

const isAdmin = computed(() => auth.user?.role === 'admin')
const updatedText = computed(() =>
  updatedAt.value ? `订阅更新于 ${dayjs(updatedAt.value).format('YYYY-MM-DD HH:mm')}` : '暂无订阅',
)

const productTypeMeta: Record<string, { label: string; color: string }> = {
  yaml: { label: 'YAML', color: 'blue' },
  subs: { label: 'SR 节点订阅', color: 'cyan' },
  'generic-subs': { label: '通用节点订阅', color: 'purple' },
}
const productTypeLabel = (pt?: string) => productTypeMeta[pt ?? '']?.label ?? pt ?? '—'

// --- 流量卡片 ---
const trafficUsedGB = computed(() => ((traffic.value.used_bytes ?? 0) / 1024 ** 3).toFixed(2))
const trafficQuotaGB = computed(() =>
  traffic.value.quota_bytes ? (traffic.value.quota_bytes / 1024 ** 3).toFixed(2) : null,
)
const trafficPercent = computed(() => {
  if (!traffic.value.quota_bytes) return 0
  return Math.min(100, Math.round(((traffic.value.used_bytes ?? 0) / traffic.value.quota_bytes) * 100))
})
const trafficColor = computed(() => {
  if (traffic.value.exceeded || trafficPercent.value >= 100) return '#FF4D4F'
  if (trafficPercent.value >= 80) return '#FAAD14'
  return '#1677FF'
})

// --- 分流规则卡 ---
// 复制规则链接统一使用 CopyField（R14-17）

// --- 平台卡操作 ---
const isUserBound = (card: PlatformCard) => card.status === 'ready' || card.status === 'custom'

function oneClickImport(card: PlatformCard, urlOverride?: string) {
  const scheme = card.schemes[0]
  if (!scheme) return
  const target = urlOverride ?? card.download_url
  if (!target) return
  window.location.href = buildImportUrl(scheme, target)
  setTimeout(() => Notify.info('请确认已安装对应客户端'), 3000)
}

async function copyLink(card: PlatformCard, urlOverride?: string) {
  const url = urlOverride ?? card.download_url
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    Notify.success(isUserBound(card) ? '链接已复制（该链接与您的账号绑定，请勿分享）' : '链接已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

const refreshTarget = ref<PlatformCard | null>(null)
const refreshing = ref(false)
async function doRefresh() {
  if (!refreshTarget.value) return
  refreshing.value = true
  try {
    const res = await refreshHomeToken(refreshTarget.value.platform_id)
    const card = refreshTarget.value
    card.download_token = res.token
    if (card.download_url) card.download_url = card.download_url.replace(/token=.*$/, `token=${res.token}`)
    Notify.success('链接已刷新，旧链接立即失效')
    refreshTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    refreshing.value = false
  }
}
const closeRefresh = () => { refreshTarget.value = null }

// 管理员按平台预览当前版本
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewContent = ref('')
const previewTitle = ref('')
async function previewPlatform(card: PlatformCard) {
  previewOpen.value = true
  previewLoading.value = true
  previewTitle.value = card.name
  try {
    previewContent.value = await previewSubscriptionByPlatform(card.slug)
  } catch (err) {
    Notify.error((err as Error).message)
    previewContent.value = ''
  } finally {
    previewLoading.value = false
  }
}

// 下载客户端条目
interface InstallerEntryUI { kind: 'local' | 'url'; label: string; url: string }
function installerEntries(card: PlatformCard): InstallerEntryUI[] {
  const out: InstallerEntryUI[] = []
  for (const it of card.installer_files ?? []) out.push({ kind: 'local', label: it.name || it.url, url: it.url })
  for (const it of card.installer_urls ?? []) out.push({ kind: 'url', label: it.name || it.url, url: it.url })
  return out
}
function openInstaller(url: string) { window.open(url, '_blank') }

const unassigned = (card: PlatformCard) => card.status === 'unassigned'
const custom = (card: PlatformCard) => card.status === 'custom'
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <AppHeader :updated-at="updatedText" manage-btn />

    <main class="max-w-6xl mx-auto p-4">
      <div v-if="loading" class="text-center py-16 text-gray-400">加载中…</div>

      <template v-else>
        <!-- 1) 流量卡片（基础模式仅「不限流量」） -->
        <Card class="mb-4 shadow-sm dark:text-gray-100">
          <div class="text-sm font-medium mb-2">流量</div>
          <template v-if="traffic.unlimited">
            <div class="text-gray-500">不限流量</div>
          </template>
          <template v-else>
            <div class="text-sm">
              本月已用 <span class="font-medium">{{ trafficUsedGB }}</span> GB
              <template v-if="trafficQuotaGB"> / 配额 {{ trafficQuotaGB }} GB</template>
            </div>
            <Progress v-if="traffic.quota_bytes" :percent="trafficPercent" :stroke-color="trafficColor" />
            <Alert v-if="traffic.exceeded" type="error" show-icon class="mt-2"
                   message="本月流量已超限，代理账号已暂停，请联系管理员重置" />
          </template>
        </Card>

        <!-- 2) 分流规则卡片（全体用户可见） -->
        <Card class="mb-4 shadow-sm dark:text-gray-100" hoverable @click="router.push('/rules')">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div>
              <div class="font-medium">分流规则</div>
              <div v-if="homeRule" class="text-sm text-gray-500">
                {{ homeRule.name }} · v{{ homeRule.current_version }}
              </div>
              <div v-else class="text-sm text-gray-400">管理员暂未设置分流规则</div>
              <div class="text-xs text-gray-400 mt-1">
                Shadowrocket 使用指引：先添加订阅获取节点，再导入分流规则
              </div>
            </div>
            <div v-if="homeRule" @click.stop>
              <CopyField :value="homeRule.download_url" label="规则链接" />
            </div>
          </div>
        </Card>

        <!-- 3) 平台卡片网格 -->
        <div v-if="cards.length" class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 mb-4">
          <Card v-for="card in cards" :key="card.platform_id" class="shadow-sm">
            <template #title>
              <div>
                <span class="font-medium">{{ card.name }}</span>
                <div v-if="card.description" class="text-xs text-gray-400 font-normal">{{ card.description }}</div>
              </div>
            </template>

            <!-- 管理员：预览形态（模板信息 + 按平台预览当前版本） -->
            <template v-if="isAdmin">
              <template v-if="card.subscription">
                <div class="text-sm space-y-2">
                  <div>
                    <span class="text-gray-500">订阅：</span>{{ card.subscription.name }}
                    <Tag class="ml-1" :color="productTypeMeta[card.subscription.product_type]?.color">
                      {{ productTypeLabel(card.subscription.product_type) }}
                    </Tag>
                    <Tag class="ml-1" :color="card.subscription.content_kind === 'blueprint' ? 'purple' : ''">
                      {{ card.subscription.content_kind === 'blueprint' ? '装配模板' : '直接上传' }}
                    </Tag>
                  </div>
                  <div v-if="card.subscription.current_version > 0">
                    <span class="text-gray-500">当前版本：</span>v{{ card.subscription.current_version }}
                    <span v-if="card.subscription.version_updated_at" class="text-xs text-gray-400 ml-2">
                      {{ dayjs(card.subscription.version_updated_at).format('YYYY-MM-DD HH:mm') }}
                    </span>
                  </div>
                  <div v-else class="text-gray-400">未激活</div>
                </div>
              </template>
              <div v-else class="text-gray-400 text-center py-6 border rounded bg-gray-50 dark:bg-gray-700">
                暂无可用版本，请联系管理员
              </div>
              <div class="mt-3">
                <Tooltip :title="card.preview_available ? '' : '暂无激活版本'">
                  <Button size="small" :disabled="!card.preview_available" @click="previewPlatform(card)">
                    按平台预览当前版本
                  </Button>
                </Tooltip>
              </div>
            </template>

            <!-- 普通用户：custom / unassigned / ready -->
            <template v-else>
              <Alert v-if="custom(card)" type="info" show-icon class="mb-2"
                     message="已被分配自定义订阅" description="由管理员单独配置，覆盖组内分发" />
              <div v-else-if="unassigned(card)"
                   class="text-gray-400 text-center py-6 border rounded bg-gray-50 dark:bg-gray-700">
                暂无可用版本，请联系管理员
              </div>
              <div v-else class="mb-3 text-sm">
                <span class="text-gray-500">当前订阅：</span>{{ card.subscription_name || '—' }}
                <Tag v-if="card.subscription_product_type" class="ml-1"
                     :color="productTypeMeta[card.subscription_product_type]?.color">
                  {{ productTypeLabel(card.subscription_product_type) }}
                </Tag>
                <div v-if="card.version_updated_at" class="text-xs text-gray-400 mt-1">
                  更新于 {{ dayjs(card.version_updated_at).format('YYYY-MM-DD HH:mm') }}
                </div>
              </div>

              <div v-if="!unassigned(card)" class="mt-3 flex flex-wrap gap-2">
                <Button v-if="card.schemes?.length" type="primary" @click="oneClickImport(card)">一键导入</Button>
                <Button @click="copyLink(card)">复制链接</Button>
                <Button @click="refreshTarget = card">刷新链接</Button>
              </div>
            </template>

            <!-- 底部：下载客户端 -->
            <div v-if="installerEntries(card).length" class="mt-3 pt-3 border-t">
              <Dropdown :trigger="['click']">
                <Button size="small">下载客户端</Button>
                <template #overlay>
                  <div class="bg-white dark:bg-gray-800 p-1 shadow-lg rounded border border-gray-200 dark:border-gray-600 min-w-44 max-w-80 max-h-64 overflow-auto">
                    <div v-for="(it, i) in installerEntries(card)" :key="i"
                         class="px-3 py-1.5 rounded text-sm cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                         @click="openInstaller(it.url)">
                      <span class="flex-none">{{ it.kind === 'local' ? '📦' : '🔗' }}</span>
                      <span class="truncate" :title="it.url">{{ it.label }}</span>
                    </div>
                  </div>
                </template>
              </Dropdown>
            </div>
          </Card>
        </div>
        <Empty v-else description="暂无订阅" />

        <!-- 4) 公告栏卡片 -->
        <Card v-if="announcement" class="shadow-sm dark:text-gray-100">
          <MarkdownView :source="announcement" />
        </Card>
      </template>
    </main>

    <ConfirmModal :open="refreshTarget !== null" title="刷新链接" :loading="refreshing"
                  content="刷新后旧链接立即失效，请更新客户端中的订阅地址" @confirm="doRefresh"
                  @update:open="closeRefresh" />

    <Modal v-model:open="previewOpen" :title="`${previewTitle} · 按平台预览`" :footer="null" width="960px"
           :destroy-on-close="true">
      <div v-if="previewLoading" class="text-center py-8 text-gray-400">加载中…</div>
      <template v-else>
        <Alert v-if="previewContent.startsWith('# error:')" type="warning" show-icon class="mb-2"
               message="当前暂无可用版本" />
        <pre class="bg-gray-50 dark:bg-gray-900 p-3 rounded text-xs overflow-auto max-h-[60vh] whitespace-pre-wrap">{{ previewContent }}</pre>
      </template>
    </Modal>
  </div>
</template>
