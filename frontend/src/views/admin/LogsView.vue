<!-- LogsView.vue：日志查看（UI §5.9，Design1 §3.4.9）——a-tabs 双页签：访问日志 / 实时日志流（SSE） -->
<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import dayjs from 'dayjs'
import { Badge, Button, DatePicker, Pagination, Select, Space, Table, Tabs } from 'ant-design-vue'
import { queryAccessLogs, clearAccessLogs, issueStreamToken, type AccessLog, type LogEntry } from '@/api/log'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

// --- 访问日志页签 ---
const loading = ref(false)
const list = ref<AccessLog[]>([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const range = ref<any>(null)

async function loadAccess() {
  loading.value = true
  try {
    const q: { from: string; to: string; page: number; size: number } = { from: '', to: '', page: page.value, size: size.value }
    if (range.value && range.value[0] && range.value[1]) {
      q.from = range.value[0].format('YYYY-MM-DD')
      q.to = range.value[1].format('YYYY-MM-DD')
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

// --- 实时日志流页签（SSE：短期 Token + 8 连接上限） ---
const activeTab = ref('access')
const paused = ref(false)
const levelFilter = ref('')
const lines = ref<LogEntry[]>([])
const connected = ref(false)
let eventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reconnectCount = 0

const levelColor: Record<string, string> = {
  info: 'level-info', warn: 'level-warn', error: 'level-error', debug: 'level-debug',
}

// SSE 短期 Token 换取过程对 UI 透明：自动先换 Token 再建 EventSource（Design1 §4.8）
async function connect() {
  try {
    const { token } = await issueStreamToken() // POST /admin/logs/stream/token（Bearer 会话）
    eventSource = new EventSource(`/api/admin/logs/stream?token=${token}`)
    eventSource.onopen = () => {
      connected.value = true
      reconnectCount = 0
    }
    eventSource.onmessage = (ev) => {
      if (paused.value) return // 暂停：停止渲染（后端缓冲继续滚动）
      try {
        const entry = JSON.parse(ev.data) as LogEntry
        if (levelFilter.value && entry.level !== levelFilter.value) return
        lines.value.push(entry)
        if (lines.value.length > 1000) lines.value = lines.value.slice(-1000) // 前端渲染上限
        void nextTick(() => scrollToBottom())
      } catch { /* 忽略畸形帧 */ }
    }
    eventSource.onerror = () => {
      connected.value = false
      eventSource?.close()
      if (activeTab.value !== 'stream') return // 离开页签不重连
      Notify.warning('日志流连接断开，正在重连…')
      reconnectCount++
      // 重连重新换取 Token（严格一次性）；连接数达上限时提示
      if (reconnectCount > 3) {
        Notify.warning('多次重连失败：可能已达连接数上限，请关闭其他日志页后重试')
        return
      }
      reconnectTimer = setTimeout(connect, 3000)
    }
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

function disconnect() {
  eventSource?.close()
  eventSource = null
  connected.value = false
  if (reconnectTimer) clearTimeout(reconnectTimer)
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

// 页签切换：进入实时流页签时连接，离开时断开
function onTabChange(key: any) {
  activeTab.value = key
  if (key === 'stream') {
    lines.value = [] // 重新连接后先推缓冲历史
    void connect()
  } else {
    disconnect()
  }
}

onUnmounted(disconnect)
</script>

<template>
  <div>
    <h2 class="text-lg font-semibold mb-4">日志查看</h2>
    <Tabs :active-key="activeTab" @change="onTabChange">
      <!-- 访问日志页签 -->
      <Tabs.TabPane key="access" tab="访问日志">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
          <Space>
            <DatePicker.RangePicker v-model:value="range" @change="page = 1; loadAccess()" />
            <Button @click="page = 1; loadAccess()">查询</Button>
          </Space>
          <Button danger @click="clearOpen = true">清空日志</Button>
        </div>
        <TriStateList :loading="loading" :empty="list.length === 0 && total === 0" empty-text="所选日期范围内无记录">
          <Table :data-source="list" row-key="id" :pagination="false" size="small">
            <Table.Column key="type" title="下载类型" width="120">
              <template #default="{ record }">{{ typeText[record.download_type] ?? record.download_type }}</template>
            </Table.Column>
            <Table.Column key="user" title="用户" width="110">
              <template #default="{ record }">{{ record.username || '—' }}</template>
            </Table.Column>
            <Table.Column key="platform" title="平台" width="120">
              <template #default="{ record }">{{ record.platform || '—' }}</template>
            </Table.Column>
            <Table.Column key="ip" title="IP" width="130" data-index="ip" />
            <Table.Column key="resource" title="资源标识" data-index="resource_slug">
              <template #default="{ record }">
                <span class="font-mono text-xs">{{ record.resource_slug }}</span>
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
        </TriStateList>
        <div class="flex justify-end mt-3">
          <Pagination v-model:current="page" :page-size="size" :total="total"
                      :show-total="(t: number) => `共 ${t} 条`" @change="loadAccess" />
        </div>
      </Tabs.TabPane>

      <!-- 实时日志流页签 -->
      <Tabs.TabPane key="stream" tab="实时日志流">
        <div class="flex flex-wrap items-center gap-2 mb-3">
          <Select v-model:value="levelFilter" style="width: 130px" allow-clear placeholder="级别过滤">
            <Select.Option value="info">info</Select.Option>
            <Select.Option value="warn">warn</Select.Option>
            <Select.Option value="error">error</Select.Option>
            <Select.Option value="debug">debug</Select.Option>
          </Select>
          <Button @click="paused = !paused">{{ paused ? '继续' : '暂停' }}</Button>
          <Button @click="clearScreen">清屏</Button>
          <span class="text-xs" :class="connected ? 'text-green-500' : 'text-gray-400'">
            {{ connected ? '已连接' : '未连接' }}
          </span>
        </div>
        <!-- 终端风深色底，不随主题变化；等宽字体；级别色块 -->
        <div ref="containerRef" class="log-terminal font-mono text-xs p-4 rounded h-[60vh] overflow-auto">
          <div v-if="lines.length === 0" class="text-gray-500">等待日志输出…</div>
          <div v-for="(line, i) in lines" :key="i" :class="levelColor[line.level] ?? 'level-info'">
            [{{ dayjs(line.time).format('MM-DD HH:mm:ss') }}] [{{ line.level.toUpperCase() }}] {{ line.message }}
            <span v-if="line.attrs" class="text-gray-400">{{ line.attrs }}</span>
          </div>
        </div>
      </Tabs.TabPane>
    </Tabs>

    <!-- 清空日志确认 -->
    <ConfirmModal :open="clearOpen" title="清空日志" danger
                  content="将删除全部访问日志记录（不可恢复）。确定继续？"
                  :loading="clearing" @confirm="confirmClear" @update:open="clearOpen = false" />
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
