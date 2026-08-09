<!-- HomeView.vue：用户首页（UI §4.1）——通用顶栏 + 平台卡片网格（三态）+ 规则入口 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Card, Collapse, Empty, Modal, TypographyText } from 'ant-design-vue'
import dayjs from 'dayjs'
import { homePlatforms, refreshHomeToken, homeUpdatedAt, type PlatformCard } from '@/api/home'
import { getPublicAnnouncement } from '@/api/settings'
import { buildImportUrl } from '@/utils/importUrl'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import AppHeader from '@/components/AppHeader.vue'
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
    // 公告公开端点独立获取：失败不阻塞平台卡片渲染（Design1 §3.3 有内容才显示）
    try {
      announcement.value = (await getPublicAnnouncement())?.content ?? ''
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

// 复制链接：弹窗展示 URL + 复制；绑定类 Token 警示
const copyTarget = ref<{ card: PlatformCard; url: string } | null>(null)
function openCopy(card: PlatformCard, urlOverride?: string) {
  const url = urlOverride ?? card.download_url
  if (isUserBound(card)) Notify.warning('该链接与您的账号绑定，请勿分享')
  copyTarget.value = { card, url }
}
async function doCopy() {
  if (!copyTarget.value) return
  try {
    await navigator.clipboard.writeText(copyTarget.value.url)
    Notify.success('链接已复制')
    copyTarget.value = null
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
const closeCopy = () => { copyTarget.value = null }

// 下载客户端
const hasInstaller = (card: PlatformCard) => !!card.installer_file_url || !!card.installer_url
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
        <!-- 公告栏卡片（Design1 §3.3：有内容才显示；纯文本插值天然转义禁 HTML，§3.4.8） -->
        <Card v-if="announcement" class="mb-4 shadow-sm">
          <div class="text-sm whitespace-pre-wrap">{{ announcement }}</div>
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

            <!-- 订阅区段：普通用户三态 -->
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

            <!-- 管理员：池内全部订阅折叠列表（每份三按钮） -->
            <Collapse v-else ghost>
              <Collapse.Panel :key="card.platform_id" :header="`池内订阅（${card.subscriptions?.length ?? 0}）`">
                <!-- 圆角浅色块行（方案 C）：每行独立浅灰圆角容器，块状分隔，暗色模式深灰底；
                     按钮用普通 flex 容器（AntD Space 在 flex 行内有 4px 垂直偏移异常，导致文本与按钮不对齐） -->
                <div class="space-y-2">
                  <div v-for="sub in card.subscriptions ?? []" :key="sub.id"
                       class="rounded-md bg-gray-100 dark:bg-gray-700/50 px-3 py-2">
                    <div class="flex items-center justify-between gap-2 flex-wrap">
                      <!-- 仅展示订阅名称，不展示标识（R09-11：标识为系统内部唯一 ID，主界面无需暴露） -->
                      <div class="text-sm min-w-0 truncate">
                        <span class="font-medium">{{ sub.name }}</span>
                      </div>
                      <div class="flex items-center gap-2 flex-shrink-0">
                        <Button size="small" type="primary" @click="oneClickImport(card, sub.download_url)">一键导入</Button>
                        <Button size="small" @click="openCopy(card, sub.download_url)">复制链接</Button>
                      </div>
                    </div>
                  </div>
                </div>
              </Collapse.Panel>
            </Collapse>

            <!-- 操作按钮组（未分配时三按钮隐藏；管理员显式 Token 无刷新接口不显示刷新） -->
            <div v-if="!unassigned(card) || isAdmin" class="mt-3 flex flex-wrap gap-2">
              <Button v-if="card.schemes?.length" type="primary" @click="oneClickImport(card)">一键导入</Button>
              <Button @click="openCopy(card)">复制链接</Button>
              <Button v-if="!isAdmin" @click="refreshTarget = card">刷新链接</Button>
            </div>

            <!-- 底部：下载客户端（本地/外链并存则两个都显示） -->
            <div v-if="hasInstaller(card)" class="mt-3 pt-3 border-t flex flex-wrap gap-2">
              <Button v-if="card.installer_file_url" size="small" @click="openInstaller(card.installer_file_url)">
                下载客户端
              </Button>
              <Button v-if="card.installer_url" size="small" @click="openInstaller(card.installer_url)">
                官网下载
              </Button>
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

    <!-- 复制链接弹窗（绑定类 Token 复制时已弹警示） -->
    <Modal :open="copyTarget !== null" title="复制链接" :footer="null" :width="560" @cancel="closeCopy">
      <TypographyText code class="break-all text-xs">{{ copyTarget?.url }}</TypographyText>
      <div class="mt-3">
        <Button type="primary" @click="doCopy">复制链接</Button>
      </div>
    </Modal>

    <!-- 刷新确认 -->
    <ConfirmModal :open="refreshTarget !== null" title="刷新链接" :loading="refreshing"
                  content="刷新后旧链接立即失效，请更新客户端中的订阅地址" @confirm="doRefresh"
                  @update:open="closeRefresh" />
  </div>
</template>
