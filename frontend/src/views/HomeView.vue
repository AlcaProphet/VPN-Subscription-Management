<!-- HomeView.vue：用户首页（UI §4.1）——通用顶栏 + 平台卡片网格（三态）+ 规则入口 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Card, Dropdown, Empty } from 'ant-design-vue'
import dayjs from 'dayjs'
import { homePlatforms, refreshHomeToken, homeUpdatedAt, type PlatformCard } from '@/api/home'
import { getPublicAnnouncement } from '@/api/settings'
import { buildImportUrl } from '@/utils/importUrl'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import AppHeader from '@/components/AppHeader.vue'
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

onMounted(async () => {
  try {
    if (!auth.user) auth.user = await me()
    const [c, u] = await Promise.all([homePlatforms(), homeUpdatedAt()])
    cards.value = c
    updatedAt.value = u?.updated_at ?? null
    // 首页公告公开端点独立获取（R10-07：与登录页公告/页脚独立配置）：失败不阻塞平台卡片渲染（Design1 §3.3 有内容才显示）
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
  void system.fetchSiteInfo(true) // 站点名保持最新（面板修改后回到首页即刷新）
})

// 顶栏
const isAdmin = computed(() => auth.user?.role === 'admin')
const updatedText = computed(() =>
  updatedAt.value ? `订阅更新于 ${dayjs(updatedAt.value).format('YYYY-MM-DD HH:mm')}` : '暂无订阅',
)

// 绑定类 Token（复制时警示）：group_selected/custom 与账号绑定
const isUserBound = (card: PlatformCard) => card.status !== 'admin_pool'

// --- 操作按钮组 ---

// 一键导入：无 scheme 隐藏；多个取首项；对下载 URL 做 URL 编码后替换 {url}
function oneClickImport(card: PlatformCard, urlOverride?: string) {
  const scheme = card.schemes[0]
  if (!scheme) return
  const target = urlOverride ?? card.download_url
  window.location.href = buildImportUrl(scheme, target)
  // 跳转后无响应提示
  setTimeout(() => Notify.info('请确认已安装对应客户端'), 3000)
}

// 复制链接：点击即复制 + Toast（与规则管理端一致）；绑定类 Token 复制时警示勿分享
async function copyLink(card: PlatformCard, urlOverride?: string) {
  const url = urlOverride ?? card.download_url
  try {
    await navigator.clipboard.writeText(url)
    Notify.success(isUserBound(card) ? '链接已复制（该链接与您的账号绑定，请勿分享）' : '链接已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

// 刷新链接：ConfirmModal 后刷新，旧链接立即失效
const refreshTarget = ref<PlatformCard | null>(null)
const refreshing = ref(false)
async function doRefresh() {
  if (!refreshTarget.value) return
  refreshing.value = true
  try {
    const res = await refreshHomeToken(refreshTarget.value.platform_id)
    const card = refreshTarget.value
    // 重建 download_url（保持原平台标识）
    card.download_token = res.token
    card.download_url = card.download_url.replace(/token=.*$/, `token=${res.token}`)
    Notify.success('链接已刷新，旧链接立即失效')
    refreshTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    refreshing.value = false
  }
}

// 弹窗关闭（供 :open + @update:open 使用）
const closeRefresh = () => { refreshTarget.value = null }

// 下载客户端：本地安装包 + 外部下载链接合并为 Dropdown 菜单条目（点击按钮弹出）
interface InstallerEntryUI { kind: 'local' | 'url'; label: string; url: string }
function installerEntries(card: PlatformCard): InstallerEntryUI[] {
  const out: InstallerEntryUI[] = []
  for (const it of card.installer_files ?? []) {
    out.push({ kind: 'local', label: it.name || it.url, url: it.url })
  }
  for (const it of card.installer_urls ?? []) {
    out.push({ kind: 'url', label: it.name || it.url, url: it.url })
  }
  return out
}
function openInstaller(url: string) {
  window.open(url, '_blank')
}

// 卡片订阅区段状态展示
const unassigned = (card: PlatformCard) => card.status === 'unassigned'
const custom = (card: PlatformCard) => card.status === 'custom'
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- 通用顶栏（UI §4.1：站点名 + 更新时间 + 管理面板按钮 + 组名 + 用户名 Dropdown + 暗色开关） -->
    <AppHeader :updated-at="updatedText" manage-btn />

    <main class="max-w-6xl mx-auto p-4">
      <div v-if="loading" class="text-center py-16 text-gray-400">加载中…</div>

      <template v-else>
        <!-- 公告栏卡片（Design1 §3.3：有内容才显示；MarkdownView html:false 渲染 MD，原始 HTML 转义禁注入，R10-06） -->
        <Card v-if="announcement" class="mb-4 shadow-sm dark:text-gray-100">
          <MarkdownView :source="announcement" />
        </Card>
        <!-- 平台卡片网格：大屏 3 列 / 中屏 2 列 / 小屏 1 列 -->
        <div v-if="cards.length" class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          <Card v-for="card in cards" :key="card.platform_id" class="shadow-sm">
            <template #title>
              <div>
                <span class="font-medium">{{ card.name }}</span>
                <div v-if="card.description" class="text-xs text-gray-400 font-normal">{{ card.description }}</div>
              </div>
            </template>

            <!-- 订阅区段：普通用户三态 / 管理员直接平铺展示池内全部订阅 -->
            <div v-if="!isAdmin">
              <Alert v-if="custom(card)" type="info" show-icon class="mb-2"
                     message="已被分配自定义订阅" description="由管理员单独配置，覆盖组内分发" />
              <div v-else-if="unassigned(card)" class="text-gray-400 text-center py-6 border rounded bg-gray-50 dark:bg-gray-700">
                未分配，请联系管理员
              </div>
              <div v-else class="mb-3 text-sm">
                <span class="text-gray-500">当前订阅：</span>{{ card.subscription_name || '—' }}
              </div>
            </div>

            <!-- 管理员：池内全部订阅直接平铺展示（每份订阅自带按钮，不展示卡片级通用按钮） -->
            <div v-else>
              <div class="mb-2 text-sm font-medium">池内订阅（{{ card.subscriptions?.length ?? 0 }}）</div>
              <!-- 圆角浅色块行（方案 C）：每行独立浅灰圆角容器，块状分隔，暗色模式深灰底；
                   按钮用普通 flex 容器（AntD Space 在 flex 行内有 4px 垂直偏移异常，导致文本与按钮不对齐） -->
              <div v-if="card.subscriptions?.length" class="space-y-2">
                <div v-for="sub in card.subscriptions" :key="sub.id"
                     class="rounded-md bg-gray-100 dark:bg-gray-700/50 px-3 py-2">
                  <div class="flex items-center justify-between gap-2 flex-wrap">
                    <!-- 仅展示订阅名称，不展示标识（R09-11：标识为系统内部唯一 ID，主界面无需暴露） -->
                    <div class="text-sm min-w-0 truncate">
                      <span class="font-medium">{{ sub.name }}</span>
                    </div>
                    <div class="flex items-center gap-2 flex-shrink-0">
                      <Button v-if="card.schemes?.length" size="small" type="primary"
                              @click="oneClickImport(card, sub.download_url)">一键导入</Button>
                      <Button size="small" @click="copyLink(card, sub.download_url)">复制链接</Button>
                    </div>
                  </div>
                </div>
              </div>
              <div v-else class="text-gray-400 text-center py-4 border rounded bg-gray-50 dark:bg-gray-700">
                该平台暂无池内订阅
              </div>
            </div>

            <!-- 操作按钮组（仅普通用户；管理员直接在池内订阅行操作，见上方平铺列表） -->
            <div v-if="!unassigned(card) && !isAdmin" class="mt-3 flex flex-wrap gap-2">
              <Button v-if="card.schemes?.length" type="primary" @click="oneClickImport(card)">一键导入</Button>
              <Button @click="copyLink(card)">复制链接</Button>
              <Button @click="refreshTarget = card">刷新链接</Button>
            </div>

            <!-- 底部：下载客户端（本地安装包 + 外部下载链接合并，点击按钮弹出下拉） -->
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

        <!-- 分流规则入口卡片 -->
        <Card class="mt-4 shadow-sm" hoverable @click="router.push('/rules')">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">分流规则</div>
              <div class="text-xs text-gray-400">规则内容公开，链接请谨慎分发</div>
            </div>
            <Button type="link">前往</Button>
          </div>
        </Card>
      </template>
    </main>

    <!-- 刷新确认 -->
    <ConfirmModal :open="refreshTarget !== null" title="刷新链接" :loading="refreshing"
                  content="刷新后旧链接立即失效，请更新客户端中的订阅地址" @confirm="doRefresh"
                  @update:open="closeRefresh" />
  </div>
</template>
