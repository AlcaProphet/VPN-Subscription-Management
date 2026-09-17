<!-- LogsView.vue：日志查看（UI §5.9，Design1 §3.4.9；R32-02）——访问日志 / 邮件发送日志 / 实时日志流（SSE） -->
<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import dayjs from 'dayjs'
import { Alert, Badge, Button, Pagination, Select, Space, Table, Tabs } from 'ant-design-vue'
import AppRangePicker from '@/components/AppRangePicker.vue'
import {
  queryAccessLogs,
  clearAccessLogs,
  openLogStream,
  queryMailLogs,
  clearMailLogs,
  type AccessLog,
  type LogEntry,
  type MailActivityKind,
  type MailActivityRecord,
  type MailActivityStatus,
} from '@/api/log'
import { createSSEParser } from '@/utils/sse'
import ConfirmModal from '@/components/ConfirmModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

// --- 访问日志页签 ---
const loading = ref(false)
const list = ref<AccessLog[]>([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const range = ref<any>(null)
const displayMode = ref<'name' | 'unique'>('name')

// 本地日期 → 该时刻对应的 UTC 日期（后端 parseRange 按 UTC 解析，容器时区通常为 UTC，R07-03）
// 原理：本地 08-09 00:00（+08:00）→ "2026-08-08"；本地 08-09 23:59 → "2026-08-09"，后端 UTC 解析后恰好覆盖本地全天
const toUtcDate = (d: dayjs.Dayjs) => {
  const t = d.toDate()
  return new Date(t.getTime() - t.getTimezoneOffset() * 60000).toISOString().slice(0, 10)
}

async function loadAccess() {
  loading.value = true
  try {
    const q: { from: string; to: string; page: number; size: number } = { from: '', to: '', page: page.value, size: size.value }
    if (range.value && range.value[0] && range.value[1]) {
      q.from = toUtcDate(range.value[0])
      q.to = toUtcDate(range.value[1])
    }
    const res = await queryAccessLogs(q)
    list.value = res.list
    total.value = res.total
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(loadAccess)

const typeText: Record<string, string> = {
  subscription: '订阅下载', custom: '自定义订阅', explicit: '显式预览', share: '分享下载', rule: '规则下载',
}

// 清空日志（ConfirmModal 危险）
const clearOpen = ref(false)
const clearing = ref(false)
async function confirmClear() {
  clearing.value = true
  try {
    await clearAccessLogs()
    Notify.success('访问日志已清空')
    clearOpen.value = false
    await loadAccess()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    clearing.value = false
  }
}

// --- 邮件发送日志页签（R32-02：5 秒单飞轮询、只展示安全字段） ---
const mailLoading = ref(false)
const mailList = ref<MailActivityRecord[]>([])
const mailTotal = ref(0)
const mailPage = ref(1)
const mailSize = ref(20)
const mailKind = ref<MailActivityKind | ''>('')
const mailStatus = ref<MailActivityStatus | ''>('')
const mailError = ref('')
const mailInFlight = ref(false)
const mailClearOpen = ref(false)
const mailClearing = ref(false)
let mailTimer: ReturnType<typeof setInterval> | null = null
let mailQueued = false

const mailKindOptions: { label: string; value: MailActivityKind }[] = [
  { label: '本地欢迎', value: 'welcome_local' },
  { label: 'OIDC 欢迎', value: 'welcome_oidc' },
  { label: '审批通过', value: 'approval_approved' },
  { label: '审批拒绝', value: 'approval_rejected' },
  { label: '密码重置', value: 'password_reset' },
  { label: 'SMTP 测试', value: 'smtp_test' },
]
const mailStatusOptions: { label: string; value: MailActivityStatus }[] = [
  { label: '已排队', value: 'queued' },
  { label: '发送中', value: 'sending' },
  { label: 'SMTP 已接受', value: 'accepted' },
  { label: '失败', value: 'failed' },
]
const mailKindText = Object.fromEntries(mailKindOptions.map((item) => [item.value, item.label])) as Record<string, string>
const mailStatusText: Record<string, string> = {
  queued: '已排队', sending: '发送中', accepted: 'SMTP 已接受', failed: '失败',
}
const mailStageText: Record<string, string> = {
  queue_full: '队列已满',
  dispatcher_unavailable: '派发器不可用',
  config: '配置不可用',
  render: '内容渲染失败',
  connect: '连接失败',
  handshake: '握手失败',
  starttls: 'STARTTLS 失败',
  auth: '认证失败',
  mail_from: '发件人失败',
  rcpt_to: '收件人失败',
  data: '内容传输失败',
  quit: '结束会话失败',
  timeout: '超时',
  canceled: '已取消',
  internal: '内部错误',
}
const mailSourceText: Record<string, string> = {
  selfreg: '本地注册', local: '管理员创建', oidc: 'OIDC',
  approval: '审批', public_forgot: '公共忘记密码', admin_single: '管理员单个',
  admin_batch: '管理员批量', smtp_test: 'SMTP 测试',
}

function mailKindLabel(kind: string): string { return mailKindText[kind] ?? kind }
function mailStatusLabel(status: string): string { return mailStatusText[status] ?? status }
function mailSourceLabel(source: string): string { return mailSourceText[source] ?? source }
function mailStageLabel(stage: string | null): string { return stage ? (mailStageText[stage] ?? '失败') : '—' }
function mailStatusColor(status: string): string {
  if (status === 'accepted') return 'green'
  if (status === 'failed') return 'red'
  if (status === 'sending') return 'blue'
  return 'default'
}
function mailDuration(ms: number | null): string { return ms === null ? '—' : `${ms} ms` }

async function loadMail() {
  if (mailInFlight.value) {
    mailQueued = true
    return
  }
  mailInFlight.value = true
  mailLoading.value = true
  try {
    const res = await queryMailLogs({
      page: mailPage.value,
      size: mailSize.value,
      kind: mailKind.value,
      status: mailStatus.value,
    })
    mailList.value = res.list ?? []
    mailTotal.value = res.total
    mailError.value = ''
  } catch (err) {
    // 失败保留旧列表并暴露明确错误态；轮询不弹全局提示，避免干扰。
    mailError.value = (err as Error).message
  } finally {
    mailLoading.value = false
    mailInFlight.value = false
    if (mailQueued) {
      mailQueued = false
      void loadMail()
    }
  }
}

function startMailPolling() {
  if (mailTimer) return
  mailTimer = setInterval(() => { void loadMail() }, 5000)
  void loadMail()
}

function stopMailPolling() {
  if (mailTimer) {
    clearInterval(mailTimer)
    mailTimer = null
  }
  mailQueued = false
}

function onMailFilterChange() {
  mailPage.value = 1
  void loadMail()
}

async function confirmMailClear() {
  mailClearing.value = true
  try {
    await clearMailLogs()
    Notify.success('邮件发送日志已清空')
    mailClearOpen.value = false
    mailPage.value = 1
    await loadMail()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    mailClearing.value = false
  }
}

// --- 实时日志流页签（SSE：fetch + ReadableStream + 8 连接上限） ---
const activeTab = ref('access')
const paused = ref(false)
const levelFilter = ref('')
const lines = ref<LogEntry[]>([])
const connected = ref(false)
let streamAbort: AbortController | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectCount = 0
let stopReconnect = false

const levelColor: Record<string, string> = {
  info: 'level-info', warn: 'level-warn', error: 'level-error', debug: 'level-debug',
}

function scheduleReconnect() {
  if (stopReconnect || activeTab.value !== 'stream') return
  reconnectCount++
  if (reconnectCount > 3) {
    Notify.warning('多次重连失败：可能已达连接数上限，请关闭其他日志页后重试')
    return
  }
  Notify.warning('日志流连接断开，正在重连…')
  reconnectTimer = setTimeout(connect, 3000)
}

// SSE 连接：fetch 携带 Bearer 会话凭据，ReadableStream 分块交给 SSE 帧解析器。
async function connect() {
  if (streamAbort) streamAbort.abort()
  const controller = new AbortController()
  streamAbort = controller
  try {
    const resp = await openLogStream(controller.signal)
    if (controller.signal.aborted) return
    if (resp.status === 401) {
      stopReconnect = true
      connected.value = false
      Notify.error('会话已过期，请重新登录后查看实时日志')
      return
    }
    if (resp.status === 403) {
      stopReconnect = true
      connected.value = false
      Notify.error('权限不足，无法查看实时日志')
      return
    }
    if (!resp.ok || !resp.body) {
      throw new Error(`日志流连接失败（HTTP ${resp.status}）`)
    }
    connected.value = true
    reconnectCount = 0
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    const parser = createSSEParser((message) => {
      if (paused.value) return // 暂停：停止渲染（后端缓冲继续滚动）
      try {
        const entry = JSON.parse(message.data) as LogEntry
        if (levelFilter.value && entry.level !== levelFilter.value) return
        lines.value.push(entry)
        if (lines.value.length > 1000) lines.value = lines.value.slice(-1000) // 前端渲染上限
        void nextTick(() => scrollToBottom())
      } catch { /* 忽略畸形帧 */ }
    })
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      parser.feed(decoder.decode(value, { stream: true }))
    }
    parser.flush()
    if (!controller.signal.aborted) scheduleReconnect()
  } catch (err) {
    if (controller.signal.aborted || (err as Error).name === 'AbortError') return
    connected.value = false
    Notify.error((err as Error).message)
    scheduleReconnect()
  } finally {
    if (streamAbort === controller) {
      streamAbort = null
      connected.value = false
    }
  }
}

function disconnect() {
  if (streamAbort) streamAbort.abort()
  streamAbort = null
  connected.value = false
  stopReconnect = true
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

// 滚动跟随（终端容器底部）
const containerRef = ref<HTMLElement | null>(null)
function scrollToBottom() {
  const el = containerRef.value
  if (el) el.scrollTop = el.scrollHeight
}

function clearScreen() {
  lines.value = []
}

// 页签切换：实时流进入时连接、离开时断开；邮件日志仅在活动页签轮询。
function onTabChange(key: any) {
  if (activeTab.value === key) return
  activeTab.value = key
  if (key === 'stream') {
    stopMailPolling()
    lines.value = [] // 重新连接后先推缓冲历史
    stopReconnect = false
    reconnectCount = 0
    void connect()
    return
  }
  disconnect()
  if (key === 'mail') {
    startMailPolling()
  } else {
    stopMailPolling()
  }
}

onUnmounted(() => {
  disconnect()
  stopMailPolling()
})
</script>

<template>
  <div>
    <PageHeader title="日志查看" />
    <Tabs :active-key="activeTab" @change="onTabChange">
      <!-- 访问日志页签 -->
      <Tabs.TabPane key="access" tab="访问日志">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
          <Space>
            <AppRangePicker v-model:value="range" @change="page = 1; loadAccess()" />
            <Button @click="page = 1; loadAccess()">查询</Button>
            <Button @click="displayMode = displayMode === 'name' ? 'unique' : 'name'">
              {{ displayMode === 'name' ? '显示唯一值' : '显示名称' }}
            </Button>
          </Space>
          <Button v-if="total > 0" danger @click="clearOpen = true">清空日志</Button>
        </div>
        <TriStateList :loading="loading" :empty="list.length === 0 && total === 0" empty-text="所选日期范围内无记录">
          <!-- ≥768：表格 -->
          <Table :data-source="list" row-key="id" :pagination="false" size="small" class="hidden md:block">
            <Table.Column key="type" title="下载类型" width="120">
              <template #default="{ record }">{{ typeText[record.download_type] ?? record.download_type }}</template>
            </Table.Column>
            <Table.Column key="user" title="用户" width="110">
              <template #default="{ record }">
                {{ displayMode === 'name' ? (record.username || '—') : (record.user_email || record.username || '—') }}
              </template>
            </Table.Column>
            <Table.Column key="platform" title="平台" width="120">
              <template #default="{ record }">
                {{ displayMode === 'name' ? (record.platform_name || record.platform || '—') : (record.platform || '—') }}
              </template>
            </Table.Column>
            <Table.Column key="ip" title="IP" width="130" data-index="ip" />
            <Table.Column key="resource" title="资源" data-index="resource_slug">
              <template #default="{ record }">
                <span :class="displayMode === 'unique' ? 'font-mono text-xs' : ''">
                  {{ displayMode === 'name' ? (record.resource_name || record.resource_slug) : record.resource_slug }}
                </span>
              </template>
            </Table.Column>
            <Table.Column key="status" title="状态" width="90">
              <template #default="{ record }">
                <Badge :color="record.status === 'success' ? 'green' : 'red'"
                       :text="record.status === 'success' ? '成功' : '失败'" />
              </template>
            </Table.Column>
            <Table.Column key="reason" title="失败原因" width="130">
              <template #default="{ record }">{{ record.fail_reason || '—' }}</template>
            </Table.Column>
            <Table.Column key="time" title="时间" width="150">
              <template #default="{ record }">{{ dayjs(record.created_at).format('YYYY-MM-DD HH:mm:ss') }}</template>
            </Table.Column>
          </Table>

          <!-- <768：卡片（移动端易用性，与平台/订阅卡片风格一致；8 列精简展示） -->
          <div class="grid grid-cols-1 gap-2 md:hidden">
            <div v-for="log in list" :key="log.id" class="border rounded-lg p-3 bg-surface">
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-medium truncate">{{ typeText[log.download_type] ?? log.download_type }}</span>
                <Badge :color="log.status === 'success' ? 'green' : 'red'"
                       :text="log.status === 'success' ? '成功' : '失败'" />
              </div>
              <div class="text-xs text-text-secondary mt-1 space-y-0.5">
                <div>用户：{{ displayMode === 'name' ? (log.username || '—') : (log.user_email || log.username || '—') }} · 平台：{{ displayMode === 'name' ? (log.platform_name || log.platform || '—') : (log.platform || '—') }} · IP：{{ log.ip }}</div>
                <div :class="displayMode === 'unique' ? 'font-mono' : ''">资源：{{ displayMode === 'name' ? (log.resource_name || log.resource_slug) : log.resource_slug }}</div>
                <div v-if="log.fail_reason">原因：{{ log.fail_reason }}</div>
                <div>{{ dayjs(log.created_at).format('YYYY-MM-DD HH:mm:ss') }}</div>
              </div>
            </div>
          </div>
        </TriStateList>
        <div v-if="total > 0" class="flex justify-end mt-3">
          <Pagination v-model:current="page" :page-size="size" :total="total"
                      :show-total="(t: number) => `共 ${t} 条`" @change="loadAccess" />
        </div>
      </Tabs.TabPane>

      <!-- 邮件发送日志页签 -->
      <Tabs.TabPane key="mail" tab="邮件发送日志">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
          <Space :wrap="true">
            <AppSelect v-model:value="mailKind" style="width: 150px" allow-clear placeholder="类型筛选" @change="onMailFilterChange">
              <Select.Option v-for="opt in mailKindOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</Select.Option>
            </AppSelect>
            <AppSelect v-model:value="mailStatus" style="width: 150px" allow-clear placeholder="状态筛选" @change="onMailFilterChange">
              <Select.Option v-for="opt in mailStatusOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</Select.Option>
            </AppSelect>
            <Button @click="loadMail()">刷新</Button>
          </Space>
          <Button v-if="mailTotal > 0" danger @click="mailClearOpen = true">清空日志</Button>
        </div>
        <Alert type="info" show-icon class="mb-3"
               message="SMTP 已接受仅表示发件服务器接受邮件，不代表进入收件箱；本页为当前进程短期记录，服务重启后清空。" />
        <TriStateList :loading="mailLoading" :empty="mailList.length === 0 && mailTotal === 0"
                      :error="mailError || undefined" empty-text="暂无邮件发送记录" @retry="loadMail()">
          <!-- ≥768：表格 -->
          <Table :data-source="mailList" row-key="id" :pagination="false" size="small" class="hidden md:block">
            <Table.Column key="time" title="时间" width="160">
              <template #default="{ record }">{{ dayjs(record.created_at).format('YYYY-MM-DD HH:mm:ss') }}</template>
            </Table.Column>
            <Table.Column key="kind" title="类型" width="120">
              <template #default="{ record }">{{ mailKindLabel(record.kind) }}</template>
            </Table.Column>
            <Table.Column key="source" title="来源" width="120">
              <template #default="{ record }">{{ mailSourceLabel(record.source) }}</template>
            </Table.Column>
            <Table.Column key="recipient" title="收件人" width="160">
              <template #default="{ record }"><span class="font-mono text-xs">{{ record.recipient_masked }}</span></template>
            </Table.Column>
            <Table.Column key="status" title="状态" width="110">
              <template #default="{ record }">
                <Badge :color="mailStatusColor(record.status)" :text="mailStatusLabel(record.status)" />
              </template>
            </Table.Column>
            <Table.Column key="stage" title="失败阶段" width="120">
              <template #default="{ record }">{{ mailStageLabel(record.failure_stage) }}</template>
            </Table.Column>
            <Table.Column key="queue" title="队列耗时" width="100">
              <template #default="{ record }">{{ mailDuration(record.queue_duration_ms) }}</template>
            </Table.Column>
            <Table.Column key="send" title="发送耗时" width="100">
              <template #default="{ record }">{{ mailDuration(record.send_duration_ms) }}</template>
            </Table.Column>
          </Table>

          <!-- <768：移动端卡片 -->
          <div class="grid grid-cols-1 gap-2 md:hidden">
            <div v-for="rec in mailList" :key="rec.id" class="border rounded-lg p-3 bg-surface">
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-medium truncate">{{ mailKindLabel(rec.kind) }} · {{ mailSourceLabel(rec.source) }}</span>
                <Badge :color="mailStatusColor(rec.status)" :text="mailStatusLabel(rec.status)" />
              </div>
              <div class="text-xs text-text-secondary mt-1 space-y-0.5">
                <div>收件人：<span class="font-mono">{{ rec.recipient_masked }}</span></div>
                <div v-if="rec.failure_stage">失败阶段：{{ mailStageLabel(rec.failure_stage) }}</div>
                <div>队列：{{ mailDuration(rec.queue_duration_ms) }} · 发送：{{ mailDuration(rec.send_duration_ms) }}</div>
                <div>{{ dayjs(rec.created_at).format('YYYY-MM-DD HH:mm:ss') }}</div>
              </div>
            </div>
          </div>
        </TriStateList>
        <div v-if="mailTotal > 0" class="flex justify-end mt-3">
          <Pagination v-model:current="mailPage" :page-size="mailSize" :total="mailTotal"
                      :show-total="(t: number) => `共 ${t} 条`" @change="loadMail" />
        </div>
      </Tabs.TabPane>

      <!-- 实时日志流页签 -->
      <Tabs.TabPane key="stream" tab="实时日志流">
        <div class="flex flex-wrap items-center gap-2 mb-3">
          <AppSelect v-model:value="levelFilter" style="width: 130px" allow-clear placeholder="级别过滤">
            <Select.Option value="info">info</Select.Option>
            <Select.Option value="warn">warn</Select.Option>
            <Select.Option value="error">error</Select.Option>
            <Select.Option value="debug">debug</Select.Option>
          </AppSelect>
          <Button @click="paused = !paused">{{ paused ? '继续' : '暂停' }}</Button>
          <Button @click="clearScreen">清屏</Button>
          <span class="text-xs" :class="connected ? 'text-green-500' : 'text-text-tertiary'">
            {{ connected ? '已连接' : '未连接' }}
          </span>
        </div>
        <!-- 终端风深色底，不随主题变化；等宽字体；级别色块 -->
        <div ref="containerRef" class="log-terminal font-mono text-xs p-4 rounded h-[60vh] overflow-auto">
          <div v-if="lines.length === 0" class="text-text-secondary">等待日志输出…</div>
          <div v-for="(line, i) in lines" :key="i" :class="levelColor[line.level] ?? 'level-info'">
            [{{ dayjs(line.time).format('MM-DD HH:mm:ss') }}] [{{ line.level.toUpperCase() }}] {{ line.message }}
            <span v-if="line.attrs" class="text-text-tertiary">{{ line.attrs }}</span>
          </div>
        </div>
      </Tabs.TabPane>
    </Tabs>

    <!-- 清空日志确认 -->
    <ConfirmModal :open="clearOpen" title="清空访问日志" danger
                  content="将删除全部访问日志记录（不可恢复）。确定继续？"
                  :loading="clearing" @confirm="confirmClear" @update:open="clearOpen = false" />
    <ConfirmModal :open="mailClearOpen" title="清空邮件发送日志" danger
                  content="将删除当前全部邮件发送日志（不可恢复）；不会停止发送或取消排队任务。确定继续？"
                  :loading="mailClearing" @confirm="confirmMailClear" @update:open="mailClearOpen = false" />
  </div>
</template>

<style scoped>
/* 终端风深色底固定，不随暗色模式切换（UI §5.9） */
.log-terminal { background-color: #1a1a1a !important; }
.level-info { color: #e5e5e5; }
.level-warn { color: #facc15; }
.level-error { color: #f87171; }
.level-debug { color: #6b7280; }
</style>
