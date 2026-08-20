<!-- PoolDetail.vue：素材池详情（条目分页 + 手动条目 CRUD + 同步历史） -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { Alert, Badge, Button, Input, Modal, Pagination, Select, Table, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import {
  listEntries, createEntry, updateEntry, deleteEntry,
  submitSync, getSyncStatus, listSyncTasks,
  type PoolItem, type PoolEntryItem, type SyncTaskItem,
} from '@/api/pool'
import { pollTask, ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'

const props = defineProps<{ pool: PoolItem }>()
const emit = defineEmits<{ back: []; changed: [] }>()

const RULE_TYPES = ['DOMAIN', 'DOMAIN-SUFFIX', 'DOMAIN-KEYWORD', 'IP-CIDR', 'IP-CIDR6', 'PROCESS-NAME', 'PROCESS-NAME-REGEX', 'USER-AGENT']

const loading = ref(false)
const entries = ref<PoolEntryItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const manualEntries = computed(() => entries.value.filter((e) => e.source === 'manual'))
const urlEntries = computed(() => entries.value.filter((e) => e.source === 'url'))

async function loadEntries() {
  loading.value = true
  try {
    const res = await listEntries(props.pool.id, page.value, pageSize)
    entries.value = res.list
    total.value = res.total
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(loadEntries)
watch(page, loadEntries)

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
    await loadEntries()
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
    await loadEntries()
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
    emit('changed')
    await loadEntries()
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

const statusMeta: Record<string, { color: string; text: string }> = {
  running: { color: 'processing', text: '同步中' },
  succeeded: { color: 'success', text: '成功' },
  failed: { color: 'error', text: '失败' },
  partial: { color: 'warning', text: '部分失败' },
}
const fmtTime = (t?: string | null) => (t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—')
</script>

<template>
  <div>
    <div class="flex items-center gap-2 mb-3 flex-wrap">
      <Button size="small" @click="emit('back')">返回素材池</Button>
      <h3 class="font-medium m-0">{{ pool.name }}</h3>
      <span class="text-xs text-gray-400">URL {{ pool.urls.length }} · 条目 {{ pool.entry_count }}</span>
      <Button size="small" type="primary" :loading="syncing" @click="doSync">同步</Button>
    </div>

    <!-- 同步回执 -->
    <Alert v-if="syncResult" :type="syncResult.status === 'succeeded' ? 'success' : 'warning'" show-icon class="mb-3">
      <template #message>同步{{ syncResult.status === 'succeeded' ? '成功' : '存在失败项' }}</template>
      <div v-for="(u, i) in syncResult.per_url" :key="i" class="text-xs">
        <span :class="u.ok ? 'text-green-600' : 'text-red-500'">{{ u.url }}</span>
        ：{{ u.ok ? `新增 ${u.added} · 删除 ${u.removed} · 跳过 ${u.skipped}` : (u.error || '失败') }}
        <span v-if="u.skip_reasons?.length" class="text-gray-400 ml-1">{{ u.skip_reasons.join('；') }}</span>
      </div>
    </Alert>

    <div class="flex items-center justify-between mb-2 flex-wrap gap-2">
      <div class="text-sm text-gray-500">
        手动条目（前段）与 URL 同步条目（后段）按渲染顺序展示；顺序由系统维护
      </div>
      <Button size="small" @click="openCreateEntry">新增条目</Button>
    </div>

    <div v-if="loading" class="py-8 text-center text-gray-400">加载中…</div>
    <div v-else-if="entries.length === 0" class="py-8 text-center text-gray-400">池内暂无条目</div>
    <template v-else>
      <div v-if="manualEntries.length" class="text-sm font-medium text-gray-600 mt-2 mb-1">手动条目（前段）</div>
      <Table v-if="manualEntries.length" :data-source="manualEntries" :pagination="false" row-key="id" size="small">
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
      <div v-if="urlEntries.length" class="text-sm font-medium text-gray-600 mt-4 mb-1">URL 同步条目（后段）</div>
      <Table v-if="urlEntries.length" :data-source="urlEntries" :pagination="false" row-key="id" size="small">
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
          <template #default><span class="text-xs text-gray-400">系统维护</span></template>
        </Table.Column>
      </Table>
    </template>
    <Pagination v-if="total > pageSize" class="mt-3" v-model:current="page" :page-size="pageSize"
                :total="total" show-size-changer :page-size-options="['20', '50', '100']" />

    <div class="mt-4">
      <h4 class="text-sm font-medium mb-2">同步历史</h4>
      <div v-if="tasks.length === 0" class="text-xs text-gray-400">暂无同步任务</div>
      <div v-for="t in tasks" :key="t.task_id" class="border rounded p-2 mb-2 text-sm">
        <Badge :status="(statusMeta[t.status]?.color ?? 'default') as any" :text="statusMeta[t.status]?.text ?? t.status" />
        <span class="text-xs text-gray-400 ml-2">{{ fmtTime(t.started_at) }} → {{ fmtTime(t.finished_at) }}</span>
        <Tooltip v-if="t.error" :title="t.error">
          <span class="text-xs text-red-500 ml-2">原因</span>
        </Tooltip>
        <div v-for="(u, i) in t.per_url" :key="i" class="text-xs mt-1 border-t pt-1">
          <span :class="u.ok ? 'text-green-600' : 'text-red-500'">{{ u.url }}</span>
          <span v-if="u.ok" class="text-gray-500 ml-2">新增 {{ u.added }} · 删除 {{ u.removed }} · 跳过 {{ u.skipped }}</span>
          <span v-else class="text-red-500 ml-2">{{ u.error || '失败' }}</span>
          <div v-if="u.skip_reasons?.length" class="text-gray-400 ml-2">{{ u.skip_reasons.join('；') }}</div>
        </div>
      </div>
      <Pagination v-if="taskTotal > 20" class="mt-2" v-model:current="taskPage" :page-size="20" :total="taskTotal" />
    </div>

    <Modal v-model:open="entryOpen" :title="editingEntry ? '编辑条目' : '新增条目'" :footer="null" :width="480"
           destroy-on-close>
      <div class="space-y-3">
        <Select v-model:value="entryForm.rule_type" :options="RULE_TYPES.map((t) => ({ label: t, value: t }))" class="w-full" />
        <Input v-model:value="entryForm.match_value" placeholder="匹配值（按规则类型白名单校验）" @press-enter="saveEntry" />
        <div class="flex justify-end gap-2">
          <Button @click="entryOpen = false">取消</Button>
          <Button type="primary" :loading="entrySaving" @click="saveEntry">保存</Button>
        </div>
      </div>
    </Modal>
  </div>
</template>
