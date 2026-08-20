<!-- XrayInstancesView.vue：Xray 实例与独立账号管理（Build7 Step3） -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Button, Empty, Modal, Table, Tabs, TabPane, Tag } from 'ant-design-vue'
import {
  listInstances, listExtAccounts, createInstance, deleteInstance, detectNodes,
  runInit, reconcile, pushRepair, cleanOrphans, repairCredentials,
  createExtAccount, deleteExtAccount, retryExtSync, resetExtQuota, getExtCredentials,
  type XrayInstance, type ExtAccount, type DetectResult, type ReconcileResult,
} from '@/api/xray'
import PageHeader from '@/components/PageHeader.vue'
import { Notify } from '@/components/Notify'
import ConfirmModal from '@/components/ConfirmModal.vue'
import { pollTask } from '@/api/request'
import { getAdminTask } from '@/api/settings'

const instances = ref<XrayInstance[]>([])
const extAccounts = ref<ExtAccount[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const [ins, exts] = await Promise.all([listInstances(), listExtAccounts()])
    instances.value = ins
    extAccounts.value = exts
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

const createOpen = ref(false)
const createForm = ref({ name: '', api_addr: '', api_tag: '' })
const creating = ref(false)
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
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}

const detectTarget = ref<XrayInstance | null>(null)
const detectResult = ref<DetectResult | null>(null)
const detecting = ref(false)
async function doDetect(inst: XrayInstance) {
  detectTarget.value = inst
  detecting.value = true
  detectResult.value = null
  try {
    detectResult.value = await detectNodes(inst.id)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    detecting.value = false
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

const initLoading = ref(false)
async function doInit() {
  initLoading.value = true
  try {
    const res = await runInit()
    await pollTask({
      submit: () => Promise.resolve(),
      query: () => getAdminTask(res.task_id),
      isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
    }).run()
    Notify.success('初始化完成')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    initLoading.value = false
  }
}

const reconcileResult = ref<ReconcileResult | null>(null)
const reconcileTarget = ref<XrayInstance | null>(null)
async function doReconcile(inst: XrayInstance) {
  reconcileTarget.value = inst
  reconcileResult.value = null
  try {
    reconcileResult.value = await reconcile(inst.id)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doPushRepair() {
  if (!reconcileTarget.value) return
  try {
    await pushRepair(reconcileTarget.value.id)
    Notify.success('补推任务已提交')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doClean() {
  if (!reconcileTarget.value || !reconcileResult.value) return
  try {
    await cleanOrphans(reconcileTarget.value.id, reconcileResult.value.orphans.map((o) => o.email))
    Notify.success('清理任务已提交')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doRepairCredentials() {
  if (!reconcileTarget.value || !reconcileResult.value) return
  try {
    await repairCredentials(reconcileTarget.value.id, reconcileResult.value.credential_mismatches)
    Notify.success('凭据修复任务已提交')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

const extCreateOpen = ref(false)
const extForm = ref({ name: '', credential_mode: 'generate' as 'generate' | 'manual', uuid: '', proxy_secret: '', quota: null as number | null, push_targets: [] as { instance_id: number; inbound_tag: string }[] })
const extCreating = ref(false)
async function doExtCreate() {
  if (!extForm.value.name) {
    Notify.error('名称必填')
    return
  }
  extCreating.value = true
  try {
    const res = await createExtAccount({ ...extForm.value, push_targets: extForm.value.push_targets })
    Notify.success(`独立账号已创建${res.credentials ? '，请复制一次性凭据' : ''}`)
    extCreateOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    extCreating.value = false
  }
}

async function doExtRetry(acc: ExtAccount) {
  try {
    await retryExtSync(acc.id)
    Notify.success('重试任务已执行')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doExtReset(acc: ExtAccount) {
  try {
    await resetExtQuota(acc.id)
    Notify.success('配额已重置并重新推送')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doExtDelete(acc: ExtAccount) {
  try {
    await deleteExtAccount(acc.id)
    Notify.success('独立账号已删除')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doExtCredentials(acc: ExtAccount) {
  try {
    const creds = await getExtCredentials(acc.id)
    Modal.info({ title: `${acc.name} 凭据`, content: `UUID: ${creds.uuid}\n密码: ${creds.proxy_secret}` })
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
</script>

<template>
  <div>
    <PageHeader title="Xray 实例">
      <template #actions>
        <Button :loading="initLoading" @click="doInit">开始初始化</Button>
        <Button type="primary" @click="createOpen = true">新增实例</Button>
      </template>
    </PageHeader>

    <Tabs>
      <TabPane key="instances" tab="Xray 实例">
        <div v-if="instances.length === 0 && !loading" class="py-16">
          <Empty description="还没有 Xray 实例">
            <Button type="primary" @click="createOpen = true">新增实例</Button>
          </Empty>
        </div>
        <Table v-else :data-source="instances" row-key="id" :loading="loading" :pagination="false">
          <Table.Column title="名称" data-index="name" />
          <Table.Column title="API 地址" data-index="api_addr" />
          <Table.Column title="状态">
            <template #default="{ record }">
              <Tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</Tag>
            </template>
          </Table.Column>
          <Table.Column title="操作" width="260">
            <template #default="{ record }">
              <Button size="small" class="mr-1" @click="doDetect(record)">刷新节点</Button>
              <Button size="small" class="mr-1" @click="doReconcile(record)">对账</Button>
              <Button size="small" danger @click="deleting = record">删除</Button>
            </template>
          </Table.Column>
        </Table>
      </TabPane>
      <TabPane key="ext" tab="独立账号">
        <div v-if="extAccounts.length === 0 && !loading" class="py-16">
          <Empty description="还没有独立账号">
            <Button type="primary" @click="extCreateOpen = true">创建独立账号</Button>
          </Empty>
        </div>
        <Table v-else :data-source="extAccounts" row-key="id" :loading="loading" :pagination="false">
          <Table.Column title="名称" data-index="name" />
          <Table.Column title="Email" data-index="email" />
          <Table.Column title="超限">
            <template #default="{ record }">
              <Tag v-if="record.quota_exceeded" color="red">已超限</Tag>
              <span v-else>—</span>
            </template>
          </Table.Column>
          <Table.Column title="操作" width="320">
            <template #default="{ record }">
              <Button size="small" class="mr-1" @click="doExtCredentials(record)">复制凭据</Button>
              <Button size="small" class="mr-1" @click="doExtRetry(record)">重试</Button>
              <Button size="small" class="mr-1" @click="doExtReset(record)">重置配额</Button>
              <Button size="small" danger @click="doExtDelete(record)">删除</Button>
            </template>
          </Table.Column>
        </Table>
      </TabPane>
    </Tabs>

    <Modal v-model:open="createOpen" title="新增实例" :footer="null" width="480" destroy-on-close>
      <div class="space-y-3">
        <input v-model="createForm.name" placeholder="名称" class="w-full border rounded px-3 py-2" />
        <input v-model="createForm.api_addr" placeholder="api_addr (host:port)" class="w-full border rounded px-3 py-2" />
        <input v-model="createForm.api_tag" placeholder="api_tag（可空）" class="w-full border rounded px-3 py-2" />
        <Button type="primary" :loading="creating" @click="doCreate">创建</Button>
      </div>
    </Modal>

    <Modal :open="detectTarget !== null" title="刷新节点结果" :footer="null" width="520" @update:open="detectTarget = null">
      <div v-if="detectResult">
        <p>新增 {{ detectResult.added }} / 更新 {{ detectResult.updated }} / 缺失 {{ detectResult.missing }}</p>
        <p v-if="detectResult.skipped.length">跳过：{{ detectResult.skipped.map((s) => `${s.tag}: ${s.reason}`).join('；') }}</p>
      </div>
    </Modal>

    <Modal :open="reconcileTarget !== null" title="实例对账" :footer="null" width="720" @update:open="reconcileTarget = null">
      <div v-if="reconcileResult">
        <p>待补推 {{ reconcileResult.to_push.length }} / 无头 {{ reconcileResult.orphans.length }} / 疑似残留 {{ reconcileResult.ext_orphans.length }} / 凭据不一致 {{ reconcileResult.credential_mismatches.length }}</p>
        <div class="mt-3 space-x-2">
          <Button type="primary" @click="doPushRepair">一键补推</Button>
          <Button @click="doClean">清理无头</Button>
          <Button @click="doRepairCredentials">修复凭据</Button>
        </div>
      </div>
    </Modal>

    <Modal v-model:open="extCreateOpen" title="创建独立账号" :footer="null" width="720" destroy-on-close>
      <div class="space-y-3">
        <input v-model="extForm.name" placeholder="名称" class="w-full border rounded px-3 py-2" />
        <select v-model="extForm.credential_mode" class="w-full border rounded px-3 py-2">
          <option value="generate">自动生成</option>
          <option value="manual">手填接管</option>
        </select>
        <input v-if="extForm.credential_mode === 'manual'" v-model="extForm.uuid" placeholder="UUID" class="w-full border rounded px-3 py-2" />
        <input v-if="extForm.credential_mode === 'manual'" v-model="extForm.proxy_secret" placeholder="代理密码" class="w-full border rounded px-3 py-2" />
        <Button type="primary" :loading="extCreating" @click="doExtCreate">创建</Button>
      </div>
    </Modal>

    <ConfirmModal :open="deleting !== null" title="删除实例" danger :loading="deleteLoading"
                  content="将级联删除该实例下 Xray 节点、组分配与推送记录；实例不可达时 Xray 侧残留账号需手动清理。"
                  @confirm="confirmDelete" @update:open="deleting = null" />
  </div>
</template>
