<!-- PoolDetail.vue：素材池详情（条目分页 + 手动条目 CRUD + 同步历史） -->
<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { Alert, Badge, Button, Input, Pagination, Space, Table, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import {
  listEntries, createEntry, updateEntry, deleteEntry,
  submitSync, getSyncStatus, listSyncTasks, clearSyncTasks,
  listSourceStatuses, activatePending, discardPending,
  type PoolItem, type PoolEntryItem, type SyncTaskItem,
  type SourceStatusItem, type SourceSnapshot,
} from '@/api/pool'
import { listCapabilityMeta } from '@/api/rulespec'
import { pollTask, ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'

const props = defineProps<{ pool: PoolItem }>()
const emit = defineEmits<{ back: []; changed: [poolID: number]; edit: [] }>()

const manualRuleTypes = ref<string[]>([])

const manualLoading = ref(false)
const manualEntries = ref<PoolEntryItem[]>([])
const manualTotal = ref(0)
const manualPage = ref(1)
const urlExpanded = ref(false)
const urlLoading = ref(false)
const urlLoaded = ref(false)
const urlError = ref('')
const urlEntries = ref<PoolEntryItem[]>([])
const urlTotal = ref(0)
const urlPage = ref(1)
const pageSize = 20

async function loadManualEntries() {
  manualLoading.value = true
  try {
    const res = await listEntries(props.pool.id, manualPage.value, pageSize, 'manual')
    manualEntries.value = res.list
    manualTotal.value = res.total
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    manualLoading.value = false
  }
}

async function loadURLEntries() {
  urlLoading.value = true
  urlError.value = ''
  try {
    const res = await listEntries(props.pool.id, urlPage.value, pageSize, 'url')
    urlEntries.value = res.list
    urlTotal.value = res.total
    urlLoaded.value = true
  } catch (err) {
    urlError.value = (err as Error).message
  } finally {
    urlLoading.value = false
  }
}

function toggleURLEntries() {
  urlExpanded.value = !urlExpanded.value
  if (urlExpanded.value && !urlLoaded.value && !urlLoading.value) void loadURLEntries()
}

async function refreshEntries() {
  await loadManualEntries()
  if (urlExpanded.value && urlLoaded.value) await loadURLEntries()
}

// 每 URL 来源状态（Step 8）：主状态只读取服务端 latest_attempt，display_url 仅用于展示。
const sourceStatuses = ref<SourceStatusItem[]>([])
const sourceStatusesLoading = ref(false)
const sourceStatusesError = ref('')
async function loadSourceStatuses() {
  sourceStatusesLoading.value = true
  sourceStatusesError.value = ''
  try {
    sourceStatuses.value = await listSourceStatuses(props.pool.id)
  } catch (err) {
    sourceStatusesError.value = (err as Error).message
    Notify.error((err as Error).message)
  } finally {
    sourceStatusesLoading.value = false
  }
}

async function refreshAfterPendingAction() {
  emit('changed', props.pool.id)
  await Promise.all([loadSourceStatuses(), refreshEntries()])
}

async function loadRuleTypes() {
  try {
    const meta = await listCapabilityMeta()
    manualRuleTypes.value = meta.legacy.filter((m) => m.material_pool).map((m) => m.rule_type)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

onMounted(() => {
  void loadRuleTypes()
  void loadManualEntries()
  void loadSourceStatuses()
})
watch(manualPage, () => { void loadManualEntries() })
watch(urlPage, () => {
  if (urlExpanded.value && urlLoaded.value) void loadURLEntries()
})

// 手动条目编辑弹窗
const entryOpen = ref(false)
const editingEntry = ref<PoolEntryItem | null>(null)
const entryForm = reactive({ rule_type: 'DOMAIN-SUFFIX', match_value: '' })
const entrySaving = ref(false)
function openCreateEntry() {
  editingEntry.value = null
  entryForm.rule_type = 'DOMAIN-SUFFIX'
  entryForm.match_value = ''
  entryOpen.value = true
}
function openEditEntry(e: PoolEntryItem) {
  editingEntry.value = e
  entryForm.rule_type = e.rule_type
  entryForm.match_value = e.match_value
  entryOpen.value = true
}
async function saveEntry() {
  if (!entryForm.match_value.trim()) { Notify.error('匹配值不能为空'); return }
  entrySaving.value = true
  try {
    if (editingEntry.value) {
      await updateEntry(props.pool.id, editingEntry.value.id, {
        rule_type: entryForm.rule_type, match_value: entryForm.match_value.trim(),
      })
      Notify.success('条目已更新')
    } else {
      await createEntry(props.pool.id, {
        rule_type: entryForm.rule_type, match_value: entryForm.match_value.trim(),
      })
      Notify.success('条目已添加')
    }
    entryOpen.value = false
    await loadManualEntries()
    emit('changed', props.pool.id)
  } catch (err) {
    Notify.error((err as Error).message) // 409 去重冲突文案
  } finally {
    entrySaving.value = false
  }
}
async function removeEntry(e: PoolEntryItem) {
  try {
    await deleteEntry(props.pool.id, e.id)
    Notify.success('条目已删除')
    await loadManualEntries()
    emit('changed', props.pool.id)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

// 同步（复用 pollTask：组件卸载仅停前端轮询，后端任务继续）
const syncing = ref(false)
const syncResult = ref<SyncTaskItem | null>(null)
let pollHandle: { run: () => Promise<SyncTaskItem>; cancel: () => void } | null = null
async function doSync() {
  if (syncing.value) { Notify.warning('同步进行中，请等待完成'); return }
  syncing.value = true
  syncResult.value = null
  pollHandle = pollTask<SyncTaskItem>({
    submit: () => submitSync(props.pool.id),
    query: () => getSyncStatus(props.pool.id),
    isDone: (r) => ['succeeded', 'failed', 'partial'].includes(r.status),
  })
  try {
    syncResult.value = await pollHandle.run()
    if (syncResult.value.status === 'succeeded') Notify.success('同步完成')
    else Notify.warning('同步完成（存在失败项，详情见回执）')
    emit('changed', props.pool.id)
    await Promise.all([refreshEntries(), loadSourceStatuses()])
  } catch (err) {
    if (err instanceof Error && err.message === '轮询已取消') return
    if (err instanceof ApiError && err.status === 409) {
      Notify.warning('同步进行中，请等待完成')
    } else {
      Notify.error((err as Error).message)
    }
  } finally {
    syncing.value = false
    pollHandle = null
  }
}
onUnmounted(() => pollHandle?.cancel())

// 同步历史
const tasks = ref<SyncTaskItem[]>([])
const taskTotal = ref(0)
const taskPage = ref(1)
async function loadTasks() {
  try {
    const res = await listSyncTasks(props.pool.id, taskPage.value, 20)
    tasks.value = res.list
    taskTotal.value = res.total
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
onMounted(loadTasks)
watch(taskPage, loadTasks)

// 手动清理当前池已完成同步历史
const clearOpen = ref(false)
const clearing = ref(false)
async function confirmClearTasks() {
  clearing.value = true
  try {
    const res = await clearSyncTasks(props.pool.id)
    Notify.success(`已清理 ${res.cleared} 条已完成历史`)
    taskPage.value = 1
    await loadTasks()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    clearing.value = false
    clearOpen.value = false
  }
}


const statusMeta: Record<string, { color: string; text: string }> = {
  running: { color: 'processing', text: '同步中' },
  succeeded: { color: 'success', text: '成功' },
  failed: { color: 'error', text: '失败' },
  partial: { color: 'warning', text: '部分失败' },
}
const fmtTime = (t?: string | null) => (t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—')

const sourceModeLabels: Record<string, string> = { clash: 'Clash 规则源', shadowrocket: 'SR 规则源', auto: '自动识别' }
function sourceModeLabel(mode: string) { return sourceModeLabels[mode] ?? mode }

// 主状态严格由 latest_attempt.status 决定，latest_failed 只作为历史信息。
function mainStatusMeta(s: SourceStatusItem) {
  const key = s.latest_attempt?.status || 'never_synced'
  const known: Record<string, { color: string; text: string }> = {
    active: { color: 'success', text: 'active' },
    pending: { color: 'warning', text: 'pending' },
    failed: { color: 'error', text: 'failed' },
    staging: { color: 'processing', text: 'staging' },
    never_synced: { color: 'default', text: '待同步' },
  }
  return { key, ...(known[key] ?? { color: 'default', text: key }) }
}

const reasonLabels: Record<string, string> = {
  first_success: '首次成功',
  normal: '常规同步',
  format_changed: '格式变化',
  profile_changed: '平台变化',
  accepted_below_threshold: '接受量低于阈值',
  request_invalid: '请求构造无效',
  network_error: '网络错误',
  http_status_error: 'HTTP 状态错误',
  body_read_error: '响应读取失败',
  body_too_large: '响应体超限',
  html_source: 'HTML 来源',
  unrecognized_source: '来源无法识别',
  ambiguous_format: '格式不明确',
  conflicting_format: '格式冲突',
  mixed_platform: '平台混合',
  no_accepted_rules: '无接受规则',
  recognition_threshold_not_met: '识别率未达门槛',
  parse_error: '解析失败',
}
function reasonLabel(code: string) { return reasonLabels[code] ? `${reasonLabels[code]}（${code}）` : code }

function statsUnavailable(s: SourceSnapshot | null | undefined) { return !s?.stats || s.stats.schema_version === 0 }
function detectionOf(s: SourceSnapshot | null | undefined) { return s?.stats?.detection ?? null }
function countsOf(s: SourceSnapshot | null | undefined) { return s?.stats?.rule_counts ?? [] }
function comparisonOf(s: SourceSnapshot | null | undefined) { return s?.stats?.comparison ?? null }
function decisionOf(s: SourceSnapshot | null | undefined) { return s?.stats?.decision ?? null }

// pending 激活/丢弃：成功后刷新来源状态与素材条目。
const activateOpen = ref(false)
const activating = ref(false)
const activateSource = ref<SourceStatusItem | null>(null)
const discardOpen = ref(false)
const discarding = ref(false)
const discardSource = ref<SourceStatusItem | null>(null)

function openActivate(s: SourceStatusItem) {
  activateSource.value = s
  activateOpen.value = true
}
function closeActivate() {
  if (activating.value) return
  activateOpen.value = false
  activateSource.value = null
}
function openDiscard(s: SourceStatusItem) {
  discardSource.value = s
  discardOpen.value = true
}
function closeDiscard() {
  if (discarding.value) return
  discardOpen.value = false
  discardSource.value = null
}
async function confirmActivate() {
  const source = activateSource.value
  const pending = source?.pending
  if (!source || !pending) return
  activating.value = true
  try {
    await activatePending(props.pool.id, source.source_id, pending.id)
    Notify.success('pending 快照已激活')
    activateOpen.value = false
    activateSource.value = null
    await refreshAfterPendingAction()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    activating.value = false
  }
}
async function confirmDiscard() {
  const source = discardSource.value
  const pending = source?.pending
  if (!source || !pending) return
  discarding.value = true
  try {
    await discardPending(props.pool.id, source.source_id, pending.id)
    Notify.success('pending 快照已丢弃')
    discardOpen.value = false
    discardSource.value = null
    await refreshAfterPendingAction()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    discarding.value = false
  }
}
</script>

<template>
  <div>
    <div class="flex items-center gap-2 mb-3 flex-wrap">
      <div class="text-sm font-medium text-text">{{ pool.name }}</div>
      <span class="text-xs text-text-tertiary">URL {{ pool.urls.length }} · 条目 {{ pool.entry_count }} · 上次同步 {{ fmtTime(pool.last_synced_at) }}</span>
      <Badge v-if="pool.sync_status" :status="(statusMeta[pool.sync_status]?.color ?? 'default') as any"
             :text="statusMeta[pool.sync_status]?.text ?? pool.sync_status" />
      <div class="flex-1" />
      <Button size="small" :loading="syncing" @click="doSync">同步</Button>
      <Button size="small" @click="emit('edit')">编辑</Button>
    </div>

    <!-- 同步回执 -->
    <Alert v-if="syncResult" :type="syncResult.status === 'succeeded' ? 'success' : 'warning'" show-icon class="mb-3">
      <template #message>同步{{ syncResult.status === 'succeeded' ? '成功' : '存在失败项' }}</template>
      <div v-for="(u, i) in syncResult.per_url" :key="i" class="text-xs">
        <span :class="u.ok ? 'text-green-600' : 'text-red-500'">{{ u.url }}</span>
        ：{{ u.ok ? `接受 ${u.accepted ?? 0} · 排除 ${u.excluded ?? 0} · 拒绝 ${u.rejected ?? 0} · 重复 ${u.duplicates ?? 0}${u.pending ? ' · 待激活' : ''}` : (u.error || '失败') }}
      </div>
    </Alert>

    <!-- 每 URL 来源状态（Step 8）：display_url 仅用于展示 -->
    <section class="mb-4">
      <div class="flex items-center justify-between mb-2 gap-2 flex-wrap">
        <div class="text-sm font-medium">URL 来源状态</div>
        <span class="text-xs text-text-tertiary">主状态由最近一次同步尝试决定</span>
      </div>
      <div v-if="sourceStatusesLoading" class="py-6 text-center text-text-tertiary">来源状态加载中…</div>
      <Alert v-else-if="sourceStatusesError" type="error" show-icon :message="sourceStatusesError">
        <template #action>
          <Button size="small" data-testid="source-status-retry" @click="loadSourceStatuses">重试</Button>
        </template>
      </Alert>
      <div v-else-if="sourceStatuses.length === 0" class="py-6 text-center text-text-tertiary">暂无 URL 来源</div>
      <div v-else class="space-y-3">
        <div v-for="s in sourceStatuses" :key="s.source_id" data-testid="source-status-card"
             class="border rounded-lg p-3 min-w-0">
          <div class="flex flex-wrap items-center gap-2 min-w-0">
            <span class="text-sm font-medium break-all min-w-0">{{ s.display_url || '（无展示 URL）' }}</span>
            <Tag>{{ sourceModeLabel(s.source_mode) }}（{{ s.source_mode }}）</Tag>
            <span data-testid="source-main-status" :data-status="mainStatusMeta(s).key">
              <Badge :status="mainStatusMeta(s).color as any" :text="mainStatusMeta(s).text" />
            </span>
            <Tag v-if="s.latest_attempt?.format">{{ s.latest_attempt.format }}</Tag>
            <Tag v-if="s.latest_attempt?.profile">{{ s.latest_attempt.profile }}</Tag>
          </div>

          <Alert v-if="s.latest_attempt?.status === 'failed' && s.active" class="mt-2" type="warning" show-icon
                 message="同步失败，继续使用旧活动快照" />

          <div v-if="s.latest_attempt" class="mt-2 text-xs text-text-secondary break-words">
            <span>最近尝试 #{{ s.latest_attempt.id }}</span>
            <span class="ml-2">输入 {{ s.latest_attempt.input }} · 识别 {{ s.latest_attempt.recognized }} · 接受 {{ s.latest_attempt.accepted }} · 排除 {{ s.latest_attempt.excluded }} · 拒绝 {{ s.latest_attempt.rejected }} · 重复 {{ s.latest_attempt.duplicates }}</span>
            <span v-if="s.latest_attempt.error" class="ml-2 text-red-500">{{ s.latest_attempt.error }}</span>
          </div>
          <div v-else class="mt-2 text-xs text-text-tertiary">从未同步</div>

          <div v-if="s.latest_attempt?.diagnostics?.length" class="mt-2 min-w-0">
            <div class="text-xs text-text-secondary mb-1">诊断（后端已脱敏并限长，最多 20 条）</div>
            <div v-for="(d, i) in s.latest_attempt.diagnostics" :key="i" data-testid="source-diagnostic"
                 class="text-xs font-mono break-all whitespace-pre-wrap bg-surface-subtle rounded px-2 py-1 mt-1 min-w-0">
              [{{ d.kind }}] 行 {{ d.line }}：{{ d.message }}<template v-if="d.raw">（原始：{{ d.raw }}）</template>
            </div>
          </div>

          <div v-if="s.latest_attempt" class="mt-2 border-t pt-2 min-w-0 text-xs text-text-secondary">
            <div v-if="statsUnavailable(s.latest_attempt)" class="text-text-tertiary">历史统计不可用（schema_version 0）</div>
            <div v-else class="space-y-1 min-w-0">
              <div v-if="detectionOf(s.latest_attempt)" class="break-words">
                检测依据：<span class="font-mono break-all">{{ detectionOf(s.latest_attempt)?.evidence_codes.join('、') || '（无）' }}</span>
                <template v-if="detectionOf(s.latest_attempt)?.recognition_required_percent != null"> · 识别率要求 {{ detectionOf(s.latest_attempt)?.recognition_required_percent }}%</template>
              </div>
              <div v-for="(rc, idx) in countsOf(s.latest_attempt)" :key="idx" class="break-all">
                分项 {{ rc.family }}/{{ rc.matcher }}/{{ rc.scope }}：接受 {{ rc.accepted }} · 排除 {{ rc.excluded }} · 拒绝 {{ rc.rejected }} · 重复 {{ rc.duplicates }}
              </div>
              <div v-if="s.latest_attempt.stats?.unclassified_rejected">未分类拒绝 {{ s.latest_attempt.stats.unclassified_rejected }}</div>
              <div class="break-words">
                <template v-if="comparisonOf(s.latest_attempt)?.previous_active">
                  旧 active #{{ comparisonOf(s.latest_attempt)?.previous_active?.snapshot_id }}：{{ comparisonOf(s.latest_attempt)?.previous_active?.format || '—' }} · {{ comparisonOf(s.latest_attempt)?.previous_active?.profile || '—' }} · 接受 {{ comparisonOf(s.latest_attempt)?.previous_active?.accepted }}；
                </template>
                <template v-else-if="comparisonOf(s.latest_attempt)">首次同步，无旧 active 对比；</template>
                <template v-if="comparisonOf(s.latest_attempt)">
                  格式变化 {{ comparisonOf(s.latest_attempt)?.format_changed ? '是' : '否' }} · 平台变化 {{ comparisonOf(s.latest_attempt)?.profile_changed ? '是' : '否' }} · 阈值 {{ comparisonOf(s.latest_attempt)?.accepted_drop_threshold_percent }}% · 缩量触发 {{ comparisonOf(s.latest_attempt)?.accepted_drop_triggered ? '是' : '否' }}
                </template>
              </div>
              <div v-if="decisionOf(s.latest_attempt)" class="break-words">
                初始决策 {{ decisionOf(s.latest_attempt)?.initial_status }} · 原因：{{ decisionOf(s.latest_attempt)?.reason_codes.map(reasonLabel).join('、') || '—' }}
              </div>
            </div>
          </div>

          <div v-if="s.active" class="mt-2 border-t pt-2 text-xs text-text-secondary min-w-0">
            <div>当前活动快照 #{{ s.active.id }}：输入 {{ s.active.input }} · 识别 {{ s.active.recognized }} · 接受 {{ s.active.accepted }} · 排除 {{ s.active.excluded }} · 拒绝 {{ s.active.rejected }} · 重复 {{ s.active.duplicates }}</div>
            <div>格式 {{ s.active.format || '—' }} · 平台 {{ s.active.profile || '—' }} · 激活时间
              <span data-testid="active-activated-at" :data-raw="s.active.activated_at || ''">{{ fmtTime(s.active.activated_at) }}</span>
            </div>
            <div v-if="s.active.diagnostics?.length">当前 active 诊断 {{ s.active.diagnostics.length }} 条</div>
          </div>

          <div v-if="s.latest_failed && s.latest_attempt?.id !== s.latest_failed.id" class="mt-2 text-xs text-text-tertiary break-words">
            历史失败 #{{ s.latest_failed.id }}：{{ s.latest_failed.error || s.latest_failed.diagnostics[0]?.message || '（无摘要）' }}
          </div>

          <div v-if="s.pending" class="mt-3 flex items-center gap-2 flex-wrap">
            <Tag color="warning">待激活快照 #{{ s.pending.id }}</Tag>
            <Button size="small" type="primary" @click="openActivate(s)">激活</Button>
            <Button size="small" danger @click="openDiscard(s)">丢弃</Button>
          </div>
        </div>
      </div>
    </section>

    <div class="flex items-center justify-between mb-2 flex-wrap gap-2">
      <div class="text-sm text-text-secondary">
        手动条目（前段）与 URL 同步条目（后段）按渲染顺序展示；顺序由系统维护
      </div>
      <Button size="small" @click="openCreateEntry">新增条目</Button>
    </div>

    <div class="text-sm font-medium text-text-secondary mt-2 mb-1">手动条目（前段）</div>
    <div v-if="manualLoading" class="py-8 text-center text-text-tertiary">加载中…</div>
    <template v-else>
      <div v-if="manualEntries.length">
      <Table :data-source="manualEntries" :pagination="false" row-key="id" size="small" class="hidden md:block">
        <Table.Column title="规则类型" key="type" width="170">
          <template #default="{ record }"><Tag>{{ record.rule_type }}</Tag></template>
        </Table.Column>
        <Table.Column title="匹配值" key="value">
          <template #default="{ record }"><span class="font-mono text-xs">{{ record.match_value }}</span></template>
        </Table.Column>
        <Table.Column title="来源" key="source" width="90">
          <template #default><Tag color="green">manual</Tag></template>
        </Table.Column>
        <Table.Column title="操作" key="actions" width="140">
          <template #default="{ record }">
            <Button size="small" @click="openEditEntry(record)">编辑</Button>
            <Button size="small" class="ml-1" @click="removeEntry(record)">删除</Button>
          </template>
        </Table.Column>
      </Table>
      <!-- <768 卡片态：手动条目 -->
      <div class="mobile-actions md:hidden space-y-2 mt-2">
        <div v-for="e in manualEntries" :key="e.id" class="border rounded-lg p-2">
          <div class="flex items-center justify-between gap-2">
            <div>
              <Tag>{{ e.rule_type }}</Tag>
              <span class="font-mono text-xs">{{ e.match_value }}</span>
            </div>
            <Space :size="4">
              <Button size="small" @click="openEditEntry(e)">编辑</Button>
              <Button size="small" danger @click="removeEntry(e)">删除</Button>
            </Space>
          </div>
        </div>
      </div>
      </div>
      <div v-else class="py-8 text-center text-text-tertiary">暂无手动条目</div>
    </template>
    <Pagination v-if="manualTotal > pageSize" class="mt-3" v-model:current="manualPage" :page-size="pageSize"
                :total="manualTotal" show-size-changer :page-size-options="['20', '50', '100']" />

    <section class="mt-4 border rounded-lg p-3">
      <div class="flex items-center justify-between gap-3">
        <div>
          <div class="text-sm font-medium">URL 同步条目（后段）</div>
          <div class="text-xs text-text-tertiary">展开后才查询，条目由系统维护</div>
        </div>
        <Button size="small" data-testid="toggle-url-entries" :aria-expanded="urlExpanded" @click="toggleURLEntries">
          {{ urlExpanded ? '收起' : '展开' }}
        </Button>
      </div>

      <div v-if="urlExpanded" class="mt-3">
        <div v-if="urlLoading" class="py-8 text-center text-text-tertiary">加载中…</div>
        <Alert v-else-if="urlError" type="error" show-icon :message="urlError">
          <template #action><Button size="small" @click="loadURLEntries">重试</Button></template>
        </Alert>
        <template v-else-if="urlEntries.length">
          <Table :data-source="urlEntries" :pagination="false" row-key="id" size="small" class="hidden md:block">
            <Table.Column title="规则类型" key="type" width="170">
              <template #default="{ record }"><Tag>{{ record.rule_type }}</Tag></template>
            </Table.Column>
            <Table.Column title="匹配值" key="value">
              <template #default="{ record }"><span class="font-mono text-xs">{{ record.match_value }}</span></template>
            </Table.Column>
            <Table.Column title="来源" key="source" width="90">
              <template #default><Tag color="blue">url</Tag></template>
            </Table.Column>
            <Table.Column title="操作" key="actions" width="140">
              <template #default><span class="text-xs text-text-tertiary">系统维护</span></template>
            </Table.Column>
          </Table>
          <div class="mobile-actions md:hidden space-y-2 mt-2">
            <div v-for="e in urlEntries" :key="e.id" class="border rounded-lg p-2">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <Tag>{{ e.rule_type }}</Tag>
                  <span class="font-mono text-xs">{{ e.match_value }}</span>
                </div>
                <span class="text-xs text-text-tertiary">系统维护</span>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="py-8 text-center text-text-tertiary">暂无 URL 同步条目</div>
        <Pagination v-if="urlTotal > pageSize" class="mt-3" v-model:current="urlPage" :page-size="pageSize"
                    :total="urlTotal" show-size-changer :page-size-options="['20', '50', '100']" />
      </div>
    </section>

    <div class="mt-4">
      <div class="flex items-center justify-between mb-2">
        <h4 class="text-sm font-medium">同步历史</h4>
        <Button size="small" :disabled="taskTotal === 0 || clearing" @click="clearOpen = true">清理已完成历史</Button>
      </div>
      <div v-if="tasks.length === 0" class="text-xs text-text-tertiary">暂无同步任务</div>
      <div v-for="t in tasks" :key="t.task_id" class="border rounded p-2 mb-2 text-sm">
        <Badge :status="(statusMeta[t.status]?.color ?? 'default') as any" :text="statusMeta[t.status]?.text ?? t.status" />
        <span class="text-xs text-text-tertiary ml-2">{{ fmtTime(t.started_at) }} → {{ fmtTime(t.finished_at) }}</span>
        <Tooltip v-if="t.error" :title="t.error">
          <span class="text-xs text-red-500 ml-2">原因</span>
        </Tooltip>
        <div v-for="(u, i) in t.per_url" :key="i" class="text-xs mt-1 border-t pt-1">
          <span :class="u.ok ? 'text-green-600' : 'text-red-500'">{{ u.url }}</span>
          <span v-if="u.ok" class="text-text-secondary ml-2">接受 {{ u.accepted ?? 0 }} · 排除 {{ u.excluded ?? 0 }} · 拒绝 {{ u.rejected ?? 0 }} · 重复 {{ u.duplicates ?? 0 }}<template v-if="u.pending"> · 待激活</template></span>
          <span v-else class="text-red-500 ml-2">{{ u.error || '失败' }}</span>
        </div>
      </div>
      <Pagination v-if="taskTotal > 20" class="mt-2" v-model:current="taskPage" :page-size="20" :total="taskTotal" />
    </div>

    <ConfirmModal :open="clearOpen" title="清理已完成历史" danger :loading="clearing"
                  content="将删除当前素材池中所有已完成的同步历史（含成功、部分成功、失败），不影响 active/pending 快照和运行中任务。"
                  @confirm="confirmClearTasks" @update:open="clearOpen = false" />

    <ConfirmModal :open="activateOpen" title="激活 pending 快照" :loading="activating"
                  content="激活后当前 active 将被 pending 替换。请确认旧 active 与新 pending 的输入、接受数、格式、平台和诊断差异。"
                  @confirm="confirmActivate" @update:open="closeActivate">
      <div class="space-y-2 text-sm min-w-0">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div class="border rounded p-2 min-w-0">
            <div class="font-medium">旧 active{{ activateSource?.active ? ` #${activateSource.active.id}` : '' }}</div>
            <template v-if="activateSource?.active">
              <div>输入 {{ activateSource.active.input }} · 接受 {{ activateSource.active.accepted }}</div>
              <div class="break-all">格式 {{ activateSource.active.format || '—' }} · 平台 {{ activateSource.active.profile || '—' }}</div>
            </template>
            <div v-else class="text-text-tertiary">无旧活动快照</div>
          </div>
          <div class="border rounded p-2 min-w-0">
            <div class="font-medium">新 pending #{{ activateSource?.pending?.id }}</div>
            <div>输入 {{ activateSource?.pending?.input }} · 接受 {{ activateSource?.pending?.accepted }}</div>
            <div class="break-all">格式 {{ activateSource?.pending?.format || '—' }} · 平台 {{ activateSource?.pending?.profile || '—' }}</div>
          </div>
        </div>
        <div>诊断差异：旧 active {{ activateSource?.active?.diagnostics?.length ?? 0 }} 条 / 新 pending {{ activateSource?.pending?.diagnostics?.length ?? 0 }} 条</div>
        <div v-for="(d, i) in (activateSource?.pending?.diagnostics ?? [])" :key="'pending-' + i"
             class="text-xs font-mono break-all whitespace-pre-wrap">[新] {{ d.message }}</div>
        <div v-for="(d, i) in (activateSource?.active?.diagnostics ?? [])" :key="'active-' + i"
             class="text-xs font-mono break-all whitespace-pre-wrap">[旧] {{ d.message }}</div>
      </div>
    </ConfirmModal>

    <ConfirmModal :open="discardOpen" title="丢弃 pending 快照" danger :loading="discarding"
                  content="丢弃后该 pending 快照将被删除，当前 active 不受影响。"
                  @confirm="confirmDiscard" @update:open="closeDiscard" />

    <FormOverlay v-model:open="entryOpen" :title="editingEntry ? '编辑条目' : '新增条目'" :width="480"
                 :loading="entrySaving" destroy-on-close @submit="saveEntry">
      <div class="space-y-3">
        <AppSelect v-model:value="entryForm.rule_type" :options="manualRuleTypes.map((t) => ({ label: t, value: t }))" class="w-full" />
        <Input v-model:value="entryForm.match_value" placeholder="匹配值（按规则类型白名单校验）" @press-enter="saveEntry" />
      </div>
    </FormOverlay>
  </div>
</template>
