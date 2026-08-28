<!-- PoolTab.vue：规则素材池列表（Design2-UI §5.2.1）+ 新建/编辑弹窗 -->
<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { Badge, Button, Modal, Input, Switch, Table, TimePicker, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import { listPools, createPool, updatePool, deletePool, submitSync, cancelSync, getSyncStatus, type PoolItem, type SyncTaskItem } from '@/api/pool'
import { pollTask, ApiError } from '@/api/request'
import { Notify } from '@/components/Notify'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import TriStateList from '@/components/TriStateList.vue'
import PoolDetail from './PoolDetail.vue'

const loading = ref(false)
const pools = ref<PoolItem[]>([])
const detailID = ref(0)

async function load() {
  loading.value = true
  try {
    pools.value = await listPools()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
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
const form = reactive({ name: '', urls: [''] as string[], auto_sync: false, sync_time: '04:00' })
const saving = ref(false)
function openCreate() {
  editing.value = null
  form.name = ''
  form.urls = ['']
  form.auto_sync = false
  form.sync_time = '04:00'
  formOpen.value = true
}
function openEdit(p: PoolItem) {
  editing.value = p
  form.name = p.name
  form.urls = p.urls.length ? [...p.urls] : ['']
  form.auto_sync = p.auto_sync
  form.sync_time = p.sync_time
  formOpen.value = true
}
async function save() {
  if (!form.name.trim()) { Notify.error('请填写名称'); return }
  const urls = form.urls.map((u) => u.trim()).filter(Boolean)
  for (const u of urls) {
    if (!/^https?:\/\//i.test(u)) {
      Notify.error(`URL 仅支持 http/https 地址：${u}`)
      return
    }
  }
  saving.value = true
  try {
    const data = { name: form.name.trim(), urls, auto_sync: form.auto_sync, sync_time: form.sync_time }
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
    await updatePool(p.id, { name: p.name, urls: p.urls, auto_sync: value, sync_time: p.sync_time })
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
    await load()
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
const fmtTime = (t?: string | null) => (t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—')
</script>

<template>
  <div>
    <!-- 素材池详情弹窗 -->
    <Modal :open="detailID !== 0" :footer="null" :width="760" :centered="true"
           :title="currentDetail() ? `素材池详情 · ${currentDetail()!.name}` : ''"
           :body-style="{ maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' }"
           @cancel="detailID = 0">
      <PoolDetail v-if="detailID" :pool="currentDetail()!" @changed="load" @edit="openEdit(currentDetail()!)" />
    </Modal>

    <div class="flex items-center justify-between mb-3">
        <div class="text-sm text-gray-500">维护规则素材池，供订阅装配时勾选拼接</div>
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
              <Tooltip :title="record.sync_error || ''">
                <Badge :status="(statusMeta[record.sync_status]?.color ?? 'default') as any"
                       :text="statusMeta[record.sync_status]?.text ?? '未同步'" />
              </Tooltip>
            </template>
          </Table.Column>
          <Table.Column title="定时同步" key="auto" width="150">
            <template #default="{ record }">
              <div class="flex items-center gap-2">
                <Switch :checked="record.auto_sync" :loading="toggling === record.id"
                        size="small" @change="(v: boolean | string | number) => toggleAuto(record, Boolean(v))" />
                <span class="text-xs text-gray-400">每日 {{ record.sync_time }} UTC</span>
              </div>
            </template>
          </Table.Column>
          <Table.Column title="操作" key="actions" width="230">
            <template #default="{ record }">
              <div class="flex items-center gap-1">
                <Button size="small" @click="detailID = record.id">详情</Button>
                <Button size="small" :loading="syncingID === record.id" @click="doSync(record)">同步</Button>
                <Button v-if="syncingID === record.id" size="small" danger @click="doCancelSync(record)">取消</Button>
                <Button size="small" @click="openEdit(record)">编辑</Button>
                <Button size="small" danger @click="toDelete = record">删除</Button>
              </div>
            </template>
          </Table.Column>
        </Table>

        <div class="grid grid-cols-1 gap-3 md:hidden">
          <div v-for="p in pools" :key="p.id" class="mobile-actions border rounded-lg p-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <a class="text-blue-500 font-medium" @click="detailID = p.id">{{ p.name }}</a>
              <Tooltip :title="p.sync_error || ''">
                <Badge :status="(statusMeta[p.sync_status]?.color ?? 'default') as any"
                       :text="statusMeta[p.sync_status]?.text ?? '未同步'" />
              </Tooltip>
            </div>
            <div class="text-xs text-gray-500 mt-1">URL {{ p.urls.length }} · 条目 {{ p.entry_count }} · 上次同步 {{ fmtTime(p.last_synced_at) }}</div>
            <div class="mt-2 flex items-center gap-2 flex-wrap">
              <Switch :checked="p.auto_sync" :loading="toggling === p.id" size="small"
                      @change="(v: boolean | string | number) => toggleAuto(p, Boolean(v))" />
              <span class="text-xs text-gray-400">每日 {{ p.sync_time }} UTC</span>
              <Button size="small" @click="detailID = p.id">详情</Button>
              <Button size="small" :loading="syncingID === p.id" @click="doSync(p)">同步</Button>
              <Button v-if="syncingID === p.id" size="small" danger @click="doCancelSync(p)">取消</Button>
              <Button size="small" @click="openEdit(p)">编辑</Button>
              <Button size="small" danger @click="toDelete = p">删除</Button>
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
          <div v-for="(_, i) in form.urls" :key="i" class="flex gap-2 mb-1">
            <Input v-model:value="form.urls[i]" placeholder="https://example.com/rules.txt" />
            <Button size="small" danger :disabled="form.urls.length <= 1" @click="form.urls.splice(i, 1)">删除</Button>
          </div>
          <Button size="small" @click="form.urls.push('')">添加 URL</Button>
        </div>
        <div class="flex items-center gap-3 flex-wrap">
          <span class="text-sm">定时同步</span>
          <Switch v-model:checked="form.auto_sync" size="small" />
          <TimePicker :value="form.sync_time ? dayjs(form.sync_time, 'HH:mm') : undefined" format="HH:mm" :minute-step="1"
                      @change="(t: any) => form.sync_time = t ? t.format('HH:mm') : '04:00'" />
          <span class="text-xs text-gray-400">HH:MM，按 UTC 每日执行，停机错过不补跑</span>
        </div>
      </div>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除素材池" danger :loading="deleting"
                  :content="`池内 ${toDelete?.entry_count ?? 0} 条条目将级联删除；已装配版本为快照不受影响`"
                  @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
