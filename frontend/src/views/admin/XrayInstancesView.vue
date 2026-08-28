<!-- XrayInstancesView.vue：Xray 实例与独立账号管理（Build7 Step3） -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Alert, Button, Checkbox, Empty, Input, InputNumber, Menu, Result, Switch, Table, Tabs, TabPane, Tag } from 'ant-design-vue'
import AppDropdown from '@/components/AppDropdown.vue'
import {
  listInstances, listExtAccounts, createInstance, updateInstance, deleteInstance, detectNodes, testConnection,
  runInit, reconcile, pushRepair, cleanOrphans, repairCredentials,
  createExtAccount, updateExtAccount, deleteExtAccount, retryExtSync, resetExtQuota, getExtCredentials,
  pushOne, repairCredentialsOne,
  type XrayInstance, type ExtAccount, type DetectResult, type ReconcileResult, type ReconcileItem, type ExtPushTarget,
} from '@/api/xray'
import { listNodes, setNodeDisplayName, type NodeItem } from '@/api/node'
import PageHeader from '@/components/PageHeader.vue'
import { Notify } from '@/components/Notify'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import { pollTask } from '@/api/request'
import { getAdminTask } from '@/api/settings'

const instances = ref<XrayInstance[]>([])
const extAccounts = ref<ExtAccount[]>([])
const xrayNodes = ref<NodeItem[]>([])
const loading = ref(false)
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onUnmounted(() => window.removeEventListener('resize', checkMobile))
const detectError = ref('')
const cleanOpen = ref(false)
const cleanEmails = ref<string[]>([])
const credentialsModal = ref(false)
const credentialsData = ref<{ title: string; uuid: string; secret: string } | null>(null)
const enabledCount = computed(() => instances.value.filter((item) => item.enabled).length)
const collectErrorCount = computed(() => instances.value.filter((item) => item.collect_status === 'error').length)

function formatBytes(v?: number) {
  if (v == null) return '—'
  if (v < 1024) return `${v} B`
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`
  if (v < 1024 * 1024 * 1024) return `${(v / 1024 / 1024).toFixed(1)} MB`
  return `${(v / 1024 / 1024 / 1024).toFixed(2)} GB`
}
async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Notify.success('已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

async function load() {
  loading.value = true
  try {
    const [ins, exts, nodes] = await Promise.all([listInstances(), listExtAccounts(), listNodes('xray')])
    instances.value = ins
    extAccounts.value = exts
    xrayNodes.value = nodes
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 独立账号推送目标选项：按实例分组展示 xray 节点（仅启用/可分配/未缺失，且实例启用）。
const targetOptions = computed(() => {
  const instEnabled = new Map<number, boolean>()
  for (const inst of instances.value) instEnabled.set(inst.id, inst.enabled)
  const map = new Map<number, { label: string; options: { label: string; value: string }[] }>()
  for (const n of xrayNodes.value) {
    if (n.source !== 'xray' || !n.enabled || !n.allocatable || n.missing) continue
    if (instEnabled.get(n.instance_id ?? 0) === false) continue
    if (!map.has(n.instance_id ?? 0)) {
      map.set(n.instance_id ?? 0, { label: n.instance_slug || `实例 ${n.instance_id}`, options: [] })
    }
    map.get(n.instance_id ?? 0)!.options.push({ label: `${n.render_name}（${n.tag}）`, value: `${n.instance_id}/${n.tag}` })
  }
  return Array.from(map.values())
})

const createOpen = ref(false)
const createForm = ref({ name: '', api_addr: '', api_tag: '' })
const creating = ref(false)
const testing = ref(false)
const testResult = ref('')
async function doTestConnection() {
  if (!createForm.value.api_addr) {
    Notify.error('请先填写 api_addr')
    return
  }
  testing.value = true
  testResult.value = ''
  try {
    await testConnection(createForm.value.api_addr)
    testResult.value = '连接成功'
  } catch (err) {
    testResult.value = `连接失败：${(err as Error).message}`
  } finally {
    testing.value = false
  }
}
async function doCreate() {
  if (!createForm.value.name || !createForm.value.api_addr) {
    Notify.error('名称与 api_addr 必填')
    return
  }
  creating.value = true
  try {
    await createInstance({ ...createForm.value, enabled: true })
    Notify.success('实例已创建')
    createOpen.value = false
    createForm.value = { name: '', api_addr: '', api_tag: '' }
    testResult.value = ''
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}

const editOpen = ref(false)
const editing = ref<XrayInstance | null>(null)
const editForm = ref({ name: '', api_addr: '', api_tag: '' })
const saving = ref(false)
function openEdit(inst: XrayInstance) {
  editing.value = inst
  editForm.value = { name: inst.name, api_addr: inst.api_addr, api_tag: inst.api_tag ?? '' }
  editOpen.value = true
}
async function doSaveEdit() {
  if (!editing.value || !editForm.value.name || !editForm.value.api_addr) {
    Notify.error('名称与 api_addr 必填')
    return
  }
  saving.value = true
  try {
    await updateInstance(editing.value.id, { ...editForm.value, enabled: editing.value.enabled })
    Notify.success('已保存，建议执行「刷新节点」以同步 api_addr 变化后的节点信息')
    editOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

const editTestResult = ref('')
const editTesting = ref(false)
async function doEditTestConnection() {
  if (!editForm.value.api_addr) return
  editTesting.value = true
  editTestResult.value = ''
  try {
    await testConnection(editForm.value.api_addr)
    editTestResult.value = '连接成功'
  } catch (err) {
    editTestResult.value = `连接失败：${(err as Error).message}`
  } finally {
    editTesting.value = false
  }
}

async function doToggleInstance(inst: XrayInstance) {
  try {
    await updateInstance(inst.id, { name: inst.name, api_addr: inst.api_addr, api_tag: inst.api_tag ?? '', enabled: !inst.enabled })
    Notify.success(inst.enabled ? '实例已停用（暂停管理）' : '实例已启用')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

const detectTarget = ref<XrayInstance | null>(null)
const detectResult = ref<DetectResult | null>(null)
const detecting = ref(false)
const addedNodeNames = ref<Record<number, string>>({})
const namingNodeID = ref<number | null>(null)
async function doDetect(inst: XrayInstance) {
  detectTarget.value = inst
  detecting.value = true
  detectResult.value = null
  detectError.value = ''
  try {
    const res = await detectNodes(inst.id)
    if (res.added === 0 && res.updated === 0 && res.missing === 0) {
      Notify.info('节点无变化')
      detectTarget.value = null
      return
    }
    detectResult.value = res
    addedNodeNames.value = {}
    for (const n of res.added_nodes) addedNodeNames.value[n.node_id] = n.name
  } catch (err) {
    detectError.value = (err as Error).message
  } finally {
    detecting.value = false
  }
}
async function doSetNodeDisplayName(nodeId: number) {
  const name = (addedNodeNames.value[nodeId] ?? '').trim()
  if (!name) return
  namingNodeID.value = nodeId
  try {
    await setNodeDisplayName(nodeId, name)
    Notify.success('节点显示名已设置')
    if (detectResult.value) {
      detectResult.value.added_nodes = detectResult.value.added_nodes.filter((n) => n.node_id !== nodeId)
    }
    delete addedNodeNames.value[nodeId]
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    namingNodeID.value = null
  }
}

const deleting = ref<XrayInstance | null>(null)
const deleteLoading = ref(false)
async function confirmDelete() {
  if (!deleting.value) return
  deleteLoading.value = true
  try {
    const res = await deleteInstance(deleting.value.id)
    await pollTask({
      submit: () => Promise.resolve(),
      query: () => getAdminTask(res.task_id),
      isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
    }).run()
    Notify.success('实例已删除')
    deleting.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleteLoading.value = false
  }
}

const initOpen = ref(false)
const initLoading = ref(false)
async function doInit() {
  initOpen.value = false
  initLoading.value = true
  try {
    const res = await runInit()
    const task = await pollTask({
      submit: () => Promise.resolve(),
      query: () => getAdminTask(res.task_id),
      isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
    }).run()
    if (task.status === 'failed') throw new Error(task.error || '初始化失败')
    const r = (task.result ?? {}) as { synced?: number; failed?: number }
    Notify.success(`初始化完成：成功 ${r.synced ?? 0}，失败 ${r.failed ?? 0}`)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    initLoading.value = false
  }
}

const reconcileResult = ref<ReconcileResult | null>(null)
const reconcileTarget = ref<XrayInstance | null>(null)
const cleanOrphanEmails = ref<string[]>([])
const cleanExtEmails = ref<string[]>([])
const reconcileBusy = ref(false)
const reconcileEmpty = computed(() => {
  const r = reconcileResult.value
  return !!r && r.to_push.length === 0 && r.orphans.length === 0 && r.ext_orphans.length === 0 && r.credential_mismatches.length === 0 && r.to_remove.length === 0
})
async function doReconcile(inst: XrayInstance) {
  reconcileTarget.value = inst
  reconcileResult.value = null
  cleanOrphanEmails.value = []
  cleanExtEmails.value = []
  try {
    reconcileResult.value = await reconcile(inst.id)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function runTask(taskID: string, successText: string) {
  const task = await pollTask({
    submit: () => Promise.resolve(),
    query: () => getAdminTask(taskID),
    isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
  }).run()
  if (task.status === 'failed') throw new Error(task.error || '任务失败')
  Notify.success(successText)
  await load()
}
async function doPushRepair() {
  if (!reconcileTarget.value) return
  reconcileBusy.value = true
  try {
    const res = await pushRepair(reconcileTarget.value.id)
    const task = await pollTask({
      submit: () => Promise.resolve(),
      query: () => getAdminTask(res.task_id),
      isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
    }).run()
    if (task.status === 'failed') throw new Error(task.error || '补推失败')
    const r = (task.result ?? {}) as { pushed?: number; skipped?: number; failed?: number }
    Notify.success(`补推完成：成功 ${r.pushed ?? 0}，跳过 ${r.skipped ?? 0}，失败 ${r.failed ?? 0}`)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    reconcileBusy.value = false
  }
}
async function doClean() {
  if (!reconcileTarget.value || !reconcileResult.value) return
  const emails = [...cleanOrphanEmails.value, ...cleanExtEmails.value]
  if (emails.length === 0) {
    Notify.error('请先勾选要清理的残留账号')
    return
  }
  cleanEmails.value = emails
  cleanOpen.value = true
}
async function confirmClean() {
  if (!reconcileTarget.value) return
  cleanOpen.value = false
  reconcileBusy.value = true
  try {
    const res = await cleanOrphans(reconcileTarget.value.id, cleanEmails.value)
    await runTask(res.task_id, '清理任务已完成')
    reconcileResult.value = await reconcile(reconcileTarget.value.id)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    reconcileBusy.value = false
  }
}
async function doRepairCredentials() {
  if (!reconcileTarget.value || !reconcileResult.value || reconcileResult.value.credential_mismatches.length === 0) return
  reconcileBusy.value = true
  try {
    const res = await repairCredentials(reconcileTarget.value.id, reconcileResult.value.credential_mismatches)
    await runTask(res.task_id, '凭据修复任务已完成')
    reconcileResult.value = await reconcile(reconcileTarget.value.id)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    reconcileBusy.value = false
  }
}
async function doPushOne(item: ReconcileItem) {
  if (!reconcileTarget.value) return
  try {
    await pushOne(reconcileTarget.value.id, item)
    Notify.success('已补推：' + item.email)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doCredentialsOne(item: ReconcileItem) {
  if (!reconcileTarget.value) return
  try {
    await repairCredentialsOne(reconcileTarget.value.id, item)
    Notify.success('凭据已修复：' + item.email)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doRetryExtRemove(item: ReconcileItem) {
  if (item.ext_account_id == null) return
  try {
    const res = await retryExtSync(item.ext_account_id)
    Notify.success(`移除重试完成：移除 ${res.removed ?? 0}，失败 ${res.remove_failed ?? 0}`)
    await load()
    if (reconcileTarget.value) reconcileResult.value = await reconcile(reconcileTarget.value.id)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

const extCreateOpen = ref(false)
const extCreateConfirmOpen = ref(false)
const extForm = ref({ name: '', credential_mode: 'generate' as 'generate' | 'manual', uuid: '', proxy_secret: '', quota: undefined as number | undefined })
const extSelectedTargets = ref<string[]>([])
const extCreating = ref(false)
async function doExtCreate() {
  if (!extForm.value.name) {
    Notify.error('名称必填')
    return
  }
  extCreating.value = true
  try {
    const push_targets: ExtPushTarget[] = extSelectedTargets.value.map((v) => {
      const [instance_id, inbound_tag] = v.split('/')
      return { instance_id: Number(instance_id), inbound_tag }
    })
    const res = await createExtAccount({ ...extForm.value, push_targets })
    if (res.credentials) {
      credentialsData.value = { title: `${res.account.name} 一次性凭据`, uuid: res.credentials.uuid, secret: res.credentials.proxy_secret }
        credentialsModal.value = true
    } else {
      Notify.success('独立账号已创建')
    }
    extCreateOpen.value = false
    extForm.value = { name: '', credential_mode: 'generate', uuid: '', proxy_secret: '', quota: undefined }
    extSelectedTargets.value = []
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    extCreating.value = false
  }
}

const extEditOpen = ref(false)
const extEditing = ref<ExtAccount | null>(null)
const extEditForm = ref({ name: '', quota: undefined as number | undefined, uuid: '', proxy_secret: '' })
const extEditSelectedTargets = ref<string[]>([])
const extSaving = ref(false)
function openExtEdit(acc: ExtAccount) {
  extEditing.value = acc
  extEditForm.value = { name: acc.name, quota: acc.quota ?? undefined, uuid: '', proxy_secret: '' }
  extEditSelectedTargets.value = (acc.push_targets ?? []).map((t) => `${t.instance_id}/${t.inbound_tag}`)
  extEditOpen.value = true
}
async function doExtUpdate() {
  if (!extEditing.value || !extEditForm.value.name) return
  extSaving.value = true
  try {
    const push_targets: ExtPushTarget[] = extEditSelectedTargets.value.map((v) => {
      const [instance_id, inbound_tag] = v.split('/')
      return { instance_id: Number(instance_id), inbound_tag }
    })
    await updateExtAccount(extEditing.value.id, {
      name: extEditForm.value.name,
      quota: extEditForm.value.quota,
      uuid: extEditForm.value.uuid || undefined,
      proxy_secret: extEditForm.value.proxy_secret || undefined,
      push_targets,
    })
    Notify.success('独立账号已更新')
    extEditOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    extSaving.value = false
  }
}

async function doExtRetry(acc: ExtAccount) {
  try {
    const res = await retryExtSync(acc.id)
    Notify.success(`重试完成：新增 ${res.added ?? 0}，失败 ${res.add_failed ?? 0}，移除 ${res.removed ?? 0}`)
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
const extResetTarget = ref<ExtAccount | null>(null)
const extResetting = ref(false)
async function confirmExtReset() {
  if (!extResetTarget.value) return
  extResetting.value = true
  try {
    await resetExtQuota(extResetTarget.value.id)
    Notify.success('配额已重置并重新推送')
    extResetTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    extResetting.value = false
  }
}
const extDeleteTarget = ref<ExtAccount | null>(null)
const extDeleting = ref(false)
async function confirmExtDelete() {
  if (!extDeleteTarget.value) return
  extDeleting.value = true
  try {
    await deleteExtAccount(extDeleteTarget.value.id)
    Notify.success('独立账号已删除')
    extDeleteTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    extDeleting.value = false
  }
}
async function doExtCredentials(acc: ExtAccount) {
  try {
    const creds = await getExtCredentials(acc.id)
    credentialsData.value = { title: `${acc.name} 凭据`, uuid: creds.uuid, secret: creds.proxy_secret }
    credentialsModal.value = true
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
</script>

<template>
  <div>
    <PageHeader title="Xray 实例" subtitle="按实例完成检测、对账与初始化；危险删除操作收进更多菜单，避免误触。">
      <template #actions>
        <Button type="primary" :loading="initLoading" @click="initOpen = true">开始初始化</Button>
        <Button @click="createOpen = true">新增实例</Button>
      </template>
    </PageHeader>

    <Tabs>
      <TabPane key="instances" tab="Xray 实例">
        <div class="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="xray-summary"><span>实例总数</span><strong>{{ instances.length }}</strong></div>
          <div class="xray-summary"><span>启用实例</span><strong>{{ enabledCount }}</strong></div>
          <div class="xray-summary"><span>Xray 节点</span><strong>{{ xrayNodes.length }}</strong></div>
          <div class="xray-summary"><span>采集异常</span><strong :class="collectErrorCount ? 'text-red-500' : ''">{{ collectErrorCount }}</strong></div>
        </div>
        <Alert v-if="instances.length" class="mb-3" type="info" show-icon
               message="实例列表" description="先新增或编辑实例，再按行执行节点检测与账号对账；初始化只作用于面板用户。" />
        <div v-if="instances.length === 0 && !loading" class="py-16">
          <Alert type="info" show-icon class="mb-3" message="需先在 Xray 服务器开启 gRPC API 与流量统计（policy.stats）" />
          <Empty description="还没有 Xray 实例">
            <Button type="primary" @click="createOpen = true">新增实例</Button>
          </Empty>
        </div>
        <template v-else-if="!isMobile">
          <Table :data-source="instances" row-key="id" :loading="loading" :pagination="false">
            <Table.Column title="名称" data-index="name" />
            <Table.Column title="slug" data-index="slug" />
            <Table.Column title="API 地址" data-index="api_addr" />
              <Table.Column title="API Tag">
                <template #default="{ record }">{{ record.api_tag || '—' }}</template>
              </Table.Column>
            <Table.Column title="最近采集">
              <template #default="{ record }">{{ record.last_collect_at ? new Date(record.last_collect_at).toLocaleString() : '—' }}</template>
            </Table.Column>
            <Table.Column title="状态">
              <template #default="{ record }">
                <div class="flex items-center gap-2">
                  <Switch :checked="record.enabled" @change="doToggleInstance(record)" />
                  <Tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</Tag>
                  <Tag v-if="record.collect_status === 'error'" color="red" class="ml-1" :title="record.collect_error || '采集异常'">采集异常</Tag>
                </div>
              </template>
            </Table.Column>
            <Table.Column title="操作" width="300">
              <template #default="{ record }">
                <Button size="small" class="mr-1" @click="openEdit(record)">编辑</Button>
                <Button size="small" class="mr-1" @click="doDetect(record)">刷新节点</Button>
                <Button size="small" class="mr-1" @click="doReconcile(record)">对账</Button>
                <AppDropdown>
                  <Button size="small">更多 ▾</Button>
                  <template #overlay><Menu><Menu.Item danger @click="deleting = record">删除实例</Menu.Item></Menu></template>
                </AppDropdown>
              </template>
            </Table.Column>
          </Table>
        </template>
        <div v-else class="space-y-3 py-2">
          <div v-for="inst in instances" :key="inst.id" class="mobile-actions border rounded-lg p-3">
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ inst.name }}</span>
              <Switch :checked="inst.enabled" size="small" @change="doToggleInstance(inst)" />
            </div>
            <div class="text-sm text-text-secondary mt-1">{{ inst.slug }} · {{ inst.api_addr }}</div>
            <div class="flex flex-wrap gap-2 mt-2">
              <Button size="small" @click="openEdit(inst)">编辑</Button>
              <Button size="small" @click="doDetect(inst)">刷新节点</Button>
              <Button size="small" @click="doReconcile(inst)">对账</Button>
              <AppDropdown>
                <Button size="small">更多 ▾</Button>
                <template #overlay><Menu><Menu.Item danger @click="deleting = inst">删除实例</Menu.Item></Menu></template>
              </AppDropdown>
            </div>
          </div>
        </div>
      </TabPane>
      <TabPane key="ext" tab="独立账号">
        <div v-if="extAccounts.length === 0 && !loading" class="py-16">
          <Alert type="info" show-icon class="mb-3" message="用于向面板账号体系之外的人员/场景分发凭据（可手写入自定义订阅内容）" />
          <Empty description="还没有独立账号">
            <Button type="primary" @click="extCreateOpen = true">创建独立账号</Button>
          </Empty>
        </div>
        <template v-else-if="!isMobile">
          <Table :data-source="extAccounts" row-key="id" :loading="loading" :pagination="false">
            <Table.Column title="名称" data-index="name" />
            <Table.Column title="Email" data-index="email" />
            <Table.Column title="配额">
              <template #default="{ record }">{{ record.quota == null ? '不限' : `${record.quota} GB` }}</template>
            </Table.Column>
            <Table.Column title="本月用量">
              <template #default="{ record }">{{ formatBytes(record.used_bytes) }}</template>
            </Table.Column>
            <Table.Column title="推送摘要">
              <template #default="{ record }">
                <span v-if="!record.push_targets?.length">—</span>
                <template v-else>
                  <Tag v-if="record.push_targets.some((t: any) => t.sync_status === 'failed')" color="red">{{ record.push_targets.filter((t: any) => t.sync_status === 'failed').length }} 条失败</Tag>
                  <Tag v-else color="green">{{ record.push_targets.length }} 个已同步</Tag>
                </template>
              </template>
            </Table.Column>
            <Table.Column title="超限">
              <template #default="{ record }">
                <Tag v-if="record.quota_exceeded" color="red">已超限</Tag>
                <span v-else>—</span>
              </template>
            </Table.Column>
            <Table.Column title="操作" width="360">
              <template #default="{ record }">
                <Button size="small" class="mr-1" @click="openExtEdit(record)">编辑</Button>
                <Button size="small" class="mr-1" @click="doExtCredentials(record)">复制凭据</Button>
                <Button size="small" class="mr-1" @click="doExtRetry(record)">重试</Button>
                <Button size="small" class="mr-1" @click="extResetTarget = record">重置配额</Button>
                <Button size="small" danger @click="extDeleteTarget = record">删除</Button>
              </template>
            </Table.Column>
          </Table>
        </template>
        <div v-else class="space-y-3 py-2">
          <div v-for="acc in extAccounts" :key="acc.id" class="mobile-actions border rounded-lg p-3">
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ acc.name }}</span>
              <Tag v-if="acc.quota_exceeded" color="red">已超限</Tag>
            </div>
            <div class="text-sm text-text-secondary mt-1">{{ acc.email }} · {{ acc.quota == null ? '不限流量' : `${acc.quota} GB` }} · 本月 {{ formatBytes(acc.used_bytes) }}</div>
            <div class="flex flex-wrap gap-2 mt-2">
              <Button size="small" @click="openExtEdit(acc)">编辑</Button>
              <Button size="small" @click="doExtCredentials(acc)">复制凭据</Button>
              <Button size="small" @click="doExtRetry(acc)">重试</Button>
              <Button size="small" @click="extResetTarget = acc">重置配额</Button>
              <Button size="small" danger @click="extDeleteTarget = acc">删除</Button>
            </div>
          </div>
        </div>
      </TabPane>
    </Tabs>

    <FormOverlay v-model:open="createOpen" title="新增实例" width="480" :loading="creating" destroy-on-close>
      <div class="space-y-3">
        <input v-model="createForm.name" placeholder="名称" class="w-full border rounded px-3 py-2" />
        <input v-model="createForm.api_addr" placeholder="api_addr (host:port)" class="w-full border rounded px-3 py-2" />
        <input v-model="createForm.api_tag" placeholder="api_tag（可空）" class="w-full border rounded px-3 py-2" />
        <Button :loading="testing" @click="doTestConnection">测试连接</Button>
        <Alert v-if="testResult" :type="testResult.startsWith('连接成功') ? 'success' : 'error'" show-icon :message="testResult" />
      </div>
      <template #footer>
        <Button class="touch-target" @click="createOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="creating" @click="doCreate">创建</Button>
      </template>
    </FormOverlay>

    <FormOverlay :open="editOpen" title="编辑实例" width="480" :loading="saving" destroy-on-close
                 @submit="doSaveEdit" @update:open="editOpen = false">
      <div class="space-y-3">
        <input v-model="editForm.name" placeholder="名称" class="w-full border rounded px-3 py-2" />
        <input v-model="editForm.api_addr" placeholder="api_addr (host:port)" class="w-full border rounded px-3 py-2" />
        <input v-model="editForm.api_tag" placeholder="api_tag（可空）" class="w-full border rounded px-3 py-2" />
        <div class="flex items-center gap-2">
          <Button :loading="editTesting" @click="doEditTestConnection">测试连接</Button>
          <Alert v-if="editTestResult" :type="editTestResult.startsWith('连接成功') ? 'success' : 'error'" show-icon :message="editTestResult" class="flex-1" />
        </div>
      </div>
    </FormOverlay>

    <FormOverlay :open="detectTarget !== null" title="刷新节点结果" width="560" @update:open="detectTarget = null">
      <Alert v-if="detectError" type="error" show-icon class="mb-3" :message="detectError" description="请检查 api_addr / 实例状态后重试" />
      <div v-if="detectResult" class="space-y-3">
        <p>新增 {{ detectResult.added }} / 更新 {{ detectResult.updated }} / 缺失 {{ detectResult.missing }}</p>
        <p v-if="detectResult.skipped.length">跳过：{{ detectResult.skipped.map((s) => `${s.tag}: ${s.reason}`).join('；') }}</p>
        <div v-if="detectResult.added_nodes.length">
          <div class="text-sm font-medium mb-2">为新增节点设置显示名（可选，留空则使用默认名）</div>
          <div v-for="n in detectResult.added_nodes" :key="n.node_id" class="flex items-center gap-2 mb-2">
            <span class="text-xs w-40 truncate">{{ n.name }}</span>
            <Input v-model:value="addedNodeNames[n.node_id]" :placeholder="n.name" class="flex-1" />
            <Button size="small" type="primary" :loading="namingNodeID === n.node_id" @click="doSetNodeDisplayName(n.node_id)">设置</Button>
          </div>
        </div>
      </div>
      <template #footer>
        <Button type="primary" class="touch-target" @click="detectTarget = null">关闭</Button>
      </template>
    </FormOverlay>

    <FormOverlay :open="reconcileTarget !== null" title="实例对账" width="820" @update:open="reconcileTarget = null">
      <div v-if="reconcileResult" class="space-y-4">
        <Result v-if="reconcileEmpty" status="success" title="账号已一致，无需处理" />
        <p v-if="!reconcileEmpty">待补推 {{ reconcileResult.to_push.length }} / 无头 {{ reconcileResult.orphans.length }} / 疑似残留 {{ reconcileResult.ext_orphans.length }} / 凭据不一致 {{ reconcileResult.credential_mismatches.length }} / 待移除 {{ reconcileResult.to_remove.length }}</p>
        <div>
          <div class="text-sm font-medium mb-2">待移除（移除失败，确认后可从 Xray 清理）</div>
          <div v-if="reconcileResult.to_remove.length === 0" class="text-text-tertiary text-sm">无</div>
          <div v-for="item in reconcileResult.to_remove" :key="item.email" class="flex items-center justify-between py-1">
            <span class="text-sm">{{ item.email }}（{{ item.inbound_tag }}）</span>
            <Button size="small" @click="doRetryExtRemove(item)">重试移除</Button>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-2">待补推</div>
          <div v-if="reconcileResult.to_push.length === 0" class="text-text-tertiary text-sm">无</div>
          <div v-for="item in reconcileResult.to_push" :key="item.email" class="flex items-center justify-between py-1">
            <span class="text-sm">{{ item.email }}（{{ item.inbound_tag }}）</span>
            <Button size="small" @click="doPushOne(item)">单条补推</Button>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-2">无头用户（勾选后清理）</div>
          <div v-if="reconcileResult.orphans.length === 0" class="text-text-tertiary text-sm">无</div>
          <div v-for="item in reconcileResult.orphans" :key="item.email" class="flex items-center gap-2 py-1">
            <Checkbox :checked="cleanOrphanEmails.includes(item.email)" @change="cleanOrphanEmails = cleanOrphanEmails.includes(item.email) ? cleanOrphanEmails.filter((x) => x !== item.email) : [...cleanOrphanEmails, item.email]" />
            <span class="text-sm">{{ item.email }}（{{ item.inbound_tag }}）</span>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-2">疑似独立账号残留（勾选后清理）</div>
          <div v-if="reconcileResult.ext_orphans.length === 0" class="text-text-tertiary text-sm">无</div>
          <div v-for="item in reconcileResult.ext_orphans" :key="item.email" class="flex items-center gap-2 py-1">
            <Checkbox :checked="cleanExtEmails.includes(item.email)" @change="cleanExtEmails = cleanExtEmails.includes(item.email) ? cleanExtEmails.filter((x) => x !== item.email) : [...cleanExtEmails, item.email]" />
            <span class="text-sm">{{ item.email }}（{{ item.inbound_tag }}）</span>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-2">凭据不一致</div>
          <div v-if="reconcileResult.credential_mismatches.length === 0" class="text-text-tertiary text-sm">无</div>
          <div v-for="item in reconcileResult.credential_mismatches" :key="item.email" class="flex items-center justify-between py-1">
            <span class="text-sm">{{ item.email }}（{{ item.inbound_tag }}）</span>
            <Button size="small" @click="doCredentialsOne(item)">修复凭据</Button>
          </div>
        </div>
        <div class="flex flex-wrap gap-2 pt-2 border-t">
          <Button type="primary" :loading="reconcileBusy" @click="doPushRepair">一键补推</Button>
          <Button :loading="reconcileBusy" @click="doClean">清理勾选残留</Button>
          <Button :loading="reconcileBusy" @click="doRepairCredentials">修复全部凭据</Button>
        </div>
      </div>
      <template #footer>
        <Button type="primary" class="touch-target" @click="reconcileTarget = null">关闭</Button>
      </template>
    </FormOverlay>

    <FormOverlay v-model:open="extCreateOpen" title="创建独立账号" width="720" :loading="extCreating" destroy-on-close>
      <div class="space-y-3">
        <Input v-model:value="extForm.name" placeholder="名称" class="w-full" />
        <AppSelect v-model:value="extForm.credential_mode" class="w-full" :options="[{ value: 'generate', label: '自动生成' }, { value: 'manual', label: '手填接管' }]" />
        <template v-if="extForm.credential_mode === 'manual'">
          <Input.Password v-model:value="extForm.uuid" placeholder="UUID" class="w-full" />
          <Input.Password v-model:value="extForm.proxy_secret" placeholder="代理密码" class="w-full" />
        </template>
        <InputNumber v-model:value="extForm.quota" :min="0" class="w-48" placeholder="配额（GB，0/空=不限）" />
        <div>
          <div class="text-sm text-text-tertiary mb-1">推送目标（可多选）</div>
          <AppSelect v-model:value="extSelectedTargets" mode="multiple" class="w-full" :options="targetOptions" placeholder="选择 Xray 节点" />
          <div v-if="targetOptions.length === 0" class="text-xs text-text-tertiary mt-1">请先在实例页检测节点</div>
        </div>
      </div>
      <template #footer>
        <Button class="touch-target" @click="extCreateOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="extCreating"
                @click="extForm.credential_mode === 'generate' ? extCreateConfirmOpen = true : doExtCreate()">创建</Button>
      </template>
    </FormOverlay>

    <ConfirmModal :open="extCreateConfirmOpen" title="创建独立账号（自动生成）" danger :loading="extCreating"
                  content="若 Xray 侧已存在同 email 账号，将先移除旧账号并以新生成凭据重新推送（覆盖接管，Xray 侧旧账号被踢除）。"
                  @confirm="extCreateConfirmOpen = false; doExtCreate()" @update:open="extCreateConfirmOpen = false" />

    <FormOverlay :open="extEditOpen" title="编辑独立账号" width="720" :loading="extSaving" destroy-on-close
                 @submit="doExtUpdate" @update:open="extEditOpen = false">
      <div class="space-y-3">
        <Input v-model:value="extEditForm.name" placeholder="名称" class="w-full" />
        <InputNumber v-model:value="extEditForm.quota" :min="0" class="w-48" placeholder="配额（GB，0/空=不限）" />
        <div>
          <div class="text-sm text-text-tertiary mb-1">凭据（留空=保留原凭据；修改后将对保留目标重推）</div>
          <Input.Password v-model:value="extEditForm.uuid" placeholder="UUID（留空保留）" class="w-full" />
          <Input.Password v-model:value="extEditForm.proxy_secret" placeholder="代理密码（留空保留）" class="w-full" />
        </div>
        <div>
          <div class="text-sm text-text-tertiary mb-1">推送目标（可多选，移除已选目标会同步删除 Xray 账号）</div>
          <AppSelect v-model:value="extEditSelectedTargets" mode="multiple" class="w-full" :options="targetOptions" placeholder="选择 Xray 节点" />
        </div>
      </div>
    </FormOverlay>

    <ConfirmModal :open="initOpen" title="开始初始化" danger :loading="initLoading"
                  content="初始化将：1）为所有 active 用户生成/补齐 UUID 与代理密码；2）按当前候选集向全部 Xray 实例推送账号；3）写入 xray_users 同步状态。请确认已配置实例并完成节点检测。"
                  @confirm="doInit" @update:open="initOpen = false" />

    <ConfirmModal :open="extResetTarget !== null" title="重置独立账号配额" :loading="extResetting"
                  content="将清空该账号当月流量并重新推送全部目标；若已超限，重置后恢复推送。"
                  @confirm="confirmExtReset" @update:open="extResetTarget = null" />

    <ConfirmModal :open="extDeleteTarget !== null" title="删除独立账号" danger :loading="extDeleting"
                  content="将删除独立账号、推送目标并尝试从 Xray 侧移除账号；不可达时残留需手动清理。"
                  @confirm="confirmExtDelete" @update:open="extDeleteTarget = null" />

    <ConfirmModal :open="deleting !== null" title="删除实例" danger :loading="deleteLoading"
                  content="将级联删除该实例下 Xray 节点、组分配与推送记录；实例不可达时 Xray 侧残留账号需手动清理。"
                  @confirm="confirmDelete" @update:open="deleting = null" />

    <ConfirmModal :open="cleanOpen" title="清理勾选残留" danger :loading="reconcileBusy"
                  :content="`将从 Xray 删除 ${cleanEmails.length} 个无主账号，不可恢复。`"
                  @confirm="confirmClean" @update:open="cleanOpen = false" />

    <FormOverlay :open="credentialsModal" :title="credentialsData?.title ?? '独立账号凭据'" width="520" @update:open="credentialsModal = false">
      <Alert type="warning" show-icon class="mb-3" message="凭据即该账号的唯一凭证，请妥善保管；关闭后将不再展示。" />
      <div class="space-y-2">
        <div class="flex items-center gap-2">
          <span class="w-20 text-text-secondary">UUID</span>
          <code class="flex-1 break-all bg-surface-subtle rounded px-2 py-1">{{ credentialsData?.uuid }}</code>
          <Button size="small" @click="copyText(credentialsData?.uuid ?? '')">复制</Button>
        </div>
        <div class="flex items-center gap-2">
          <span class="w-20 text-text-secondary">代理密码</span>
          <code class="flex-1 break-all bg-surface-subtle rounded px-2 py-1">{{ credentialsData?.secret }}</code>
          <Button size="small" @click="copyText(credentialsData?.secret ?? '')">复制</Button>
        </div>
      </div>
      <template #footer>
        <Button type="primary" class="touch-target" @click="credentialsModal = false">关闭</Button>
      </template>
    </FormOverlay>
  </div>
</template>

<style scoped>
.xray-summary { border: 1px solid rgb(229 231 235); border-radius: .5rem; padding: .65rem .75rem; display: flex; flex-direction: column; gap: .15rem; }
.xray-summary span { font-size: .75rem; color: rgb(107 114 128); }
.xray-summary strong { font-size: 1.25rem; line-height: 1.2; }
</style>
