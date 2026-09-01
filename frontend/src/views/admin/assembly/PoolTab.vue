<!-- PoolTab.vue：规则素材池列表（Design2-UI §5.2.1）+ 新建/编辑弹窗 -->
<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { Badge, Button, Input, Switch, Table, Tooltip } from 'ant-design-vue'
import AppModal from '@/components/AppModal.vue'
import AppTimePicker from '@/components/AppTimePicker.vue'
import dayjs from 'dayjs'
import { listPools, createPool, updatePool, deletePool, submitSync, cancelSync, getSyncStatus, type PoolItem, type SyncTaskItem, type SourceMode } from '@/api/pool'
import { pollTask, ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import TriStateList from '@/components/TriStateList.vue'
import PoolDetail from './PoolDetail.vue'

const emit = defineEmits<{
  'pools-changed': [pools: PoolItem[]]
  'pool-content-changed': [poolID: number]
}>()

const loading = ref(false)
const pools = ref<PoolItem[]>([])
const detailID = ref(0)

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const next = await listPools()
    pools.value = next
    emit('pools-changed', next)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    if (!silent) loading.value = false
  }
}
onMounted(load)
onUnmounted(() => {
  pollHandles.forEach((h) => h.cancel())
  pollHandles.clear()
})

function currentDetail() {
  return pools.value.find((p) => p.id === detailID.value) ?? null
}

// 新建/编辑
const formOpen = ref(false)
const editing = ref<PoolItem | null>(null)
const form = reactive({ name: '', sources: [{ url: '', source_mode: 'auto' as SourceMode }] as { url: string; source_mode: SourceMode }[], auto_sync: false, sync_time: '04:00' })
const saving = ref(false)
function openCreate() {
  editing.value = null
  form.name = ''
  form.sources = [{ url: '', source_mode: 'auto' }]
  form.auto_sync = false
  form.sync_time = '04:00'
  formOpen.value = true
}
function openEdit(p: PoolItem) {
  editing.value = p
  form.name = p.name
  form.sources = (p.sources || []).filter((s) => s.kind === 'url').map((s) => ({ url: s.url || '', source_mode: s.source_mode }))
  if (!form.sources.length) form.sources = [{ url: '', source_mode: 'auto' }]
  form.auto_sync = p.auto_sync
  form.sync_time = p.sync_time
  formOpen.value = true
}
async function save() {
  if (!form.name.trim()) { Notify.error('请填写名称'); return }
  const sources = form.sources.map((s) => ({ url: s.url.trim(), source_mode: s.source_mode })).filter((s) => s.url)
  for (const u of sources.map((s) => s.url)) {
    if (!/^https?:\/\//i.test(u)) {
      Notify.error(`URL 仅支持 http/https 地址：${u}`)
      return
    }
  }
  saving.value = true
  try {
    const data = { name: form.name.trim(), sources, auto_sync: form.auto_sync, sync_time: form.sync_time }
    if (editing.value) {
      await updatePool(editing.value.id, data)
      Notify.success('素材池已更新')
    } else {
      await createPool(data)
      Notify.success('素材池已创建')
    }
    formOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// 行内定时开关
const toggling = ref(0)
async function toggleAuto(p: PoolItem, value: boolean) {
  toggling.value = p.id
  try {
    const sources = (p.sources || []).filter((s) => s.kind === 'url').map((s) => ({ url: s.url || '', source_mode: s.source_mode }))
    await updatePool(p.id, { name: p.name, sources, auto_sync: value, sync_time: p.sync_time })
    p.auto_sync = value
    Notify.success(value ? '已开启定时同步' : '已关闭定时同步')
  } catch (err) {
    Notify.error((err as Error).message)
    await load() // 失败回滚
  } finally {
    toggling.value = 0
  }
}

// 行内同步（pollTask 轮询 + 可取消）
const syncingID = ref(0)
const syncTaskID = ref(0)
const pollHandles = new Map<number, { cancel: () => void }>()
async function doSync(p: PoolItem) {
  if (syncingID.value) { Notify.warning('同步进行中，请等待完成'); return }
  syncingID.value = p.id
  syncTaskID.value = 0
  const handle = pollTask<SyncTaskItem>({
    submit: async () => {
      const r = await submitSync(p.id)
      syncTaskID.value = r.task_id
    },
    query: () => getSyncStatus(p.id),
    isDone: (r) => ['succeeded', 'failed', 'partial'].includes(r.status),
  })
  pollHandles.set(p.id, handle)
  try {
    const result = await handle.run()
    if (result.status === 'succeeded') Notify.success('同步完成')
    else if (result.status === 'partial') Notify.warning('同步完成（存在失败项，请进入详情查看回执）')
    else Notify.error(result.error || '同步失败')
    emit('pool-content-changed', p.id)
    await load(true)
  } catch (err) {
    if (err instanceof Error && err.message === '轮询已取消') return
    if (err instanceof ApiError && err.status === 409) Notify.warning('同步进行中，请等待完成')
    else Notify.error((err as Error).message)
  } finally {
    syncingID.value = 0
    syncTaskID.value = 0
    pollHandles.delete(p.id)
  }
}
async function doCancelSync(p: PoolItem) {
  if (!syncTaskID.value) { Notify.warning('任务尚未开始，无法取消'); return }
  try {
    await cancelSync(p.id, syncTaskID.value)
    Notify.success('已请求取消，任务将尽快结束')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

async function onDetailChanged(poolID: number) {
  emit('pool-content-changed', poolID)
  await load()
}

const toDelete = ref<PoolItem | null>(null)
const deleting = ref(false)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deletePool(toDelete.value.id)
    Notify.success('素材池已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

const statusMeta: Record<string, { color: string; text: string }> = {
  running: { color: 'processing', text: '同步中' },
  succeeded: { color: 'success', text: '成功' },
  failed: { color: 'error', text: '失败' },
  partial: { color: 'warning', text: '部分失败' },
}
const defaultStatusMeta = { color: 'default', text: '未同步' }
function displaySyncStatus(p: PoolItem) {
  if (syncingID.value === p.id) return statusMeta.running
  return statusMeta[p.sync_status] ?? defaultStatusMeta
}
const fmtTime = (t?: string | null) => (t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—')
</script>

<template>
  <div>
    <!-- 素材池详情弹窗 -->
    <AppModal :open="detailID !== 0" :footer="null" :width="760" :centered="true"
              :title="currentDetail() ? `素材池详情 · ${currentDetail()!.name}` : ''"
              @update:open="detailID = $event ? detailID : 0">
      <PoolDetail v-if="detailID" :pool="currentDetail()!" @changed="onDetailChanged" @edit="openEdit(currentDetail()!)" />
    </AppModal>

    <div class="flex items-center justify-between mb-3">
        <div class="text-sm text-text-secondary">维护规则素材池，供订阅装配时勾选拼接</div>
        <Button type="primary" class="touch-target" @click="openCreate">新建素材池</Button>
      </div>

      <TriStateList :loading="loading" :empty="pools.length === 0" empty-text="还没有规则素材池">
        <Table :data-source="pools" :pagination="false" row-key="id" size="middle" class="hidden md:block">
          <Table.Column title="池名称" key="name">
            <template #default="{ record }">
              <a class="text-blue-500" @click="detailID = record.id">{{ record.name }}</a>
            </template>
          </Table.Column>
          <Table.Column title="URL" key="urls" width="80">
            <template #default="{ record }">{{ record.urls.length }}</template>
          </Table.Column>
          <Table.Column title="条目" key="entries" width="80">
            <template #default="{ record }">{{ record.entry_count }}</template>
          </Table.Column>
          <Table.Column title="上次同步" key="last" width="150">
            <template #default="{ record }">{{ fmtTime(record.last_synced_at) }}</template>
          </Table.Column>
          <Table.Column title="同步状态" key="status" width="120">
            <template #default="{ record }">
              <Tooltip :title="syncingID === record.id ? '' : (record.sync_error || '')">
                <span class="inline-flex min-w-16">
                  <Badge :status="displaySyncStatus(record).color as any"
                         :text="displaySyncStatus(record).text" />
                </span>
              </Tooltip>
            </template>
          </Table.Column>
          <Table.Column title="定时同步" key="auto" width="150">
            <template #default="{ record }">
              <div class="flex items-center gap-2">
                <Switch :checked="record.auto_sync" :loading="toggling === record.id"
                        size="small" @change="(v: boolean | string | number) => toggleAuto(record, Boolean(v))" />
                <span class="text-xs text-text-tertiary">每日 {{ record.sync_time }} UTC</span>
              </div>
            </template>
          </Table.Column>
          <Table.Column title="操作" key="actions" width="200">
            <template #default="{ record }">
              <div class="flex items-center gap-2">
                <Button class="pool-sync-action w-11 shrink-0" size="small"
                        :danger="syncingID === record.id"
                        :disabled="syncingID === record.id && syncTaskID === 0"
                        @click="syncingID === record.id ? doCancelSync(record) : doSync(record)">
                  {{ syncingID === record.id ? '取消' : '同步' }}
                </Button>
                <Button class="pool-edit-action w-11 shrink-0" size="small" @click="openEdit(record)">编辑</Button>
                <Button class="pool-delete-action w-11 shrink-0" size="small" danger @click="toDelete = record">删除</Button>
              </div>
            </template>
          </Table.Column>
        </Table>

        <div class="grid grid-cols-1 gap-3 md:hidden">
          <div v-for="p in pools" :key="p.id" class="mobile-actions border rounded-lg p-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <a class="text-blue-500 font-medium" @click="detailID = p.id">{{ p.name }}</a>
              <Tooltip :title="syncingID === p.id ? '' : (p.sync_error || '')">
                <span class="inline-flex min-w-16 justify-end">
                  <Badge :status="displaySyncStatus(p).color as any"
                         :text="displaySyncStatus(p).text" />
                </span>
              </Tooltip>
            </div>
            <div class="text-xs text-text-secondary mt-1">URL {{ p.urls.length }} · 条目 {{ p.entry_count }} · 上次同步 {{ fmtTime(p.last_synced_at) }}</div>
            <div class="mt-2 flex items-center gap-2">
              <label class="switch-hit">
                <Switch :checked="p.auto_sync" :loading="toggling === p.id" size="small"
                        @change="(v: boolean | string | number) => toggleAuto(p, Boolean(v))" />
              </label>
              <span class="text-xs text-text-tertiary">每日 {{ p.sync_time }} UTC</span>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <Button class="pool-sync-action w-11 shrink-0" size="small"
                      :danger="syncingID === p.id"
                      :disabled="syncingID === p.id && syncTaskID === 0"
                      @click="syncingID === p.id ? doCancelSync(p) : doSync(p)">
                {{ syncingID === p.id ? '取消' : '同步' }}
              </Button>
              <Button class="pool-edit-action w-11 shrink-0" size="small" @click="openEdit(p)">编辑</Button>
              <Button class="pool-delete-action w-11 shrink-0" size="small" danger @click="toDelete = p">删除</Button>
            </div>
          </div>
        </div>
      </TriStateList>
    <!-- 新建/编辑弹窗 -->
    <FormOverlay v-model:open="formOpen" :title="editing ? '编辑素材池' : '新建素材池'" :width="480"
                 :loading="saving" destroy-on-close @submit="save">
      <div class="space-y-3">
        <div>
          <div class="text-sm mb-1">名称（全局唯一）</div>
          <Input v-model:value="form.name" :maxlength="100" placeholder="如 苹果域名" />
        </div>
        <div>
          <div class="text-sm mb-1">订阅 URL（http/https）</div>
          <div v-for="(_, i) in form.sources" :key="i" class="flex gap-2 mb-1">
            <Input v-model:value="form.sources[i].url" placeholder="https://example.com/rules.txt" class="flex-1" />
            <select v-model="form.sources[i].source_mode" class="px-2 py-1 rounded border text-sm">
              <option value="auto">我不确定</option>
              <option value="clash">Clash 规则源</option>
              <option value="shadowrocket">SR 规则源</option>
            </select>
            <Button size="small" danger :disabled="form.sources.length <= 1" @click="form.sources.splice(i, 1)">删除</Button>
          </div>
          <Button size="small" @click="form.sources.push({ url: '', source_mode: 'auto' })">添加 URL</Button>
        </div>
        <div class="flex items-center gap-3 flex-wrap">
          <span class="text-sm">定时同步</span>
          <Switch v-model:checked="form.auto_sync" size="small" />
          <AppTimePicker :value="form.sync_time ? dayjs(form.sync_time, 'HH:mm') : undefined" format="HH:mm" :minute-step="1"
                         @change="(t: any) => form.sync_time = t ? t.format('HH:mm') : '04:00'" />
          <span class="text-xs text-text-tertiary">HH:MM，按 UTC 每日执行，停机错过不补跑</span>
        </div>
      </div>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除素材池" danger :loading="deleting"
                  :content="`池内 ${toDelete?.entry_count ?? 0} 条条目将级联删除；已装配版本为快照不受影响`"
                  @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
