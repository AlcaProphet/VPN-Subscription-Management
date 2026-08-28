<!-- SubscriptionsView.vue：订阅管理（Design2-UI §4.1）——平铺列表（每平台一份订阅条目） -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Input, Table, Tag, TypographyText } from 'ant-design-vue'
import {
  listSubscriptions,
  createSubscription,
  updateSubscription,
  deleteSubscription,
  type SubscriptionItem,
} from '@/api/subscription'
import { listPlatforms, type PlatformItem } from '@/api/platform'
import PageHeader from '@/components/PageHeader.vue'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const router = useRouter()
const loading = ref(false)
const subs = ref<SubscriptionItem[]>([])
const platforms = ref<PlatformItem[]>([])
const isMobile = ref(false)

const productTypeMeta: Record<string, { label: string; color: string }> = {
  yaml: { label: 'yaml', color: 'blue' },
  subs: { label: 'subs', color: 'cyan' },
  'generic-subs': { label: 'generic-subs', color: 'purple' },
}

function checkMobile() { isMobile.value = window.matchMedia('(max-width: 767px)').matches }
onMounted(() => { checkMobile(); window.addEventListener('resize', checkMobile) })

function pooled(id: number) {
  return sessionStorage.getItem(`pooled_sub_${id}`) === '1'
}

async function load() {
  loading.value = true
  try {
    const [s, p] = await Promise.all([listSubscriptions(), listPlatforms()])
    subs.value = s
    platforms.value = p
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

function goVersions(sub: SubscriptionItem) {
  sessionStorage.removeItem(`pooled_sub_${sub.id}`)
  void router.push(`/admin/subscriptions/${sub.id}/versions`)
}

function goAssembly(sub: SubscriptionItem) {
  const tab = sub.product_type === 'yaml' ? 'clash-yaml' : sub.product_type === 'subs' ? 'sr-subs' : 'generic-subs'
  void router.push(`/admin/assembly?tab=${tab}&platform_id=${sub.platform_id}`)
}

function goAssemblyHeader() {
  if (subs.value.length > 0) {
    goAssembly(subs.value[0])
  } else {
    void router.push('/admin/assembly')
  }
}

// 平台选项：已被订阅条目占用的平台禁用 + 后缀
const platformOptions = () =>
  platforms.value.map((p) => ({
    label: subs.value.some((s) => s.platform_id === p.id) ? `${p.name}（已有订阅）` : p.name,
    value: p.id,
    disabled: subs.value.some((s) => s.platform_id === p.id),
  }))

// --- 新建/编辑 ---
const modalOpen = ref(false)
const editing = ref<SubscriptionItem | null>(null)
const saving = ref(false)
const form = reactive({ platform_id: 0, name: '' })

function openCreate() {
  editing.value = null
  form.platform_id = platformOptions().find((o) => !o.disabled)?.value ?? 0
  form.name = ''
  modalOpen.value = true
}
function openEdit(sub: SubscriptionItem) {
  editing.value = sub
  form.platform_id = sub.platform_id
  form.name = sub.name
  modalOpen.value = true
}

async function save() {
  if (!form.name.trim() || form.platform_id <= 0) {
    Notify.error('请填写名称并选择平台')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateSubscription(editing.value.id, { name: form.name.trim() })
      Notify.success('订阅已更新')
    } else {
      await createSubscription({ platform_id: form.platform_id, name: form.name.trim() })
      Notify.success('订阅已创建，可上传内容或前往订阅装配生成模板')
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// --- 删除 ---
const toDelete = ref<SubscriptionItem | null>(null)
const deleting = ref(false)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteSubscription(toDelete.value.id)
    Notify.success('订阅已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div>
    <PageHeader title="订阅管理">
      <template #actions>
        <Button @click="goAssemblyHeader">前往装配</Button>
        <Button type="primary" @click="openCreate">新建订阅</Button>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="subs.length === 0" empty-text="还没有订阅">
      <!-- ≥768 表格 -->
      <Table v-if="!isMobile" :data-source="subs" :pagination="false" row-key="id" size="middle"
             :row-class-name="(r: SubscriptionItem) => pooled(r.id) ? 'bg-amber-50 dark:bg-amber-900/20' : ''">
        <Table.Column title="平台" data-index="platform_name" />
        <Table.Column title="订阅名称" data-index="name" />
        <Table.Column title="产物格式" key="product_type">
          <template #default="{ record }">
            <Tag :color="productTypeMeta[record.product_type]?.color">{{ record.product_type }}</Tag>
          </template>
        </Table.Column>
        <Table.Column title="内容形态" key="content_kind">
          <template #default="{ record }">
            <Tag v-if="record.content_kind === 'blueprint'" color="purple">装配模板</Tag>
            <Tag v-else-if="record.content_kind === 'upload'" color="default">直接上传</Tag>
            <span v-else class="text-text-tertiary">—</span>
          </template>
        </Table.Column>
        <Table.Column title="当前版本" key="current">
          <template #default="{ record }">
            <Tag v-if="record.current_version > 0" color="green">v{{ record.current_version }}</Tag>
            <span v-else class="text-text-tertiary">未激活</span>
          </template>
        </Table.Column>
        <Table.Column title="状态" key="status">
          <template #default="{ record }">
            <Alert v-if="pooled(record.id)" type="info" show-icon message="已入池未生效，请激活" />
          </template>
        </Table.Column>
        <Table.Column title="操作" key="actions">
          <template #default="{ record }">
            <div class="flex items-center gap-1">
              <Button size="small" @click="goVersions(record)">版本管理</Button>
              <Button size="small" @click="goAssembly(record)">装配生成</Button>
              <Button v-if="pooled(record.id)" size="small" type="primary" ghost @click="goVersions(record)">去激活</Button>
              <Button size="small" @click="openEdit(record)">编辑</Button>
              <Button size="small" danger @click="toDelete = record">删除</Button>
            </div>
          </template>
        </Table.Column>
      </Table>

      <!-- <768 卡片 -->
      <div v-else class="space-y-2">
        <div v-for="sub in subs" :key="sub.id"
             :class="pooled(sub.id) ? 'border rounded-lg p-3 bg-amber-50 dark:bg-amber-900/20 border-amber-300' : 'border rounded-lg p-3 bg-surface'">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div>
              <div class="font-medium">{{ sub.name }}</div>
              <div class="text-xs text-text-tertiary">{{ sub.platform_name }} · {{ sub.product_type }}</div>
            </div>
            <div class="mobile-actions flex flex-wrap gap-1">
              <Button size="small" @click="goVersions(sub)">版本管理</Button>
              <Button size="small" @click="goAssembly(sub)">装配生成</Button>
              <Button size="small" @click="openEdit(sub)">编辑</Button>
              <Button size="small" danger @click="toDelete = sub">删除</Button>
            </div>
          </div>
          <Alert v-if="pooled(sub.id)" type="info" show-icon class="mt-2"
                 message="已入池未生效，请激活">
            <template #action>
              <Button size="small" @click="goVersions(sub)">去激活</Button>
            </template>
          </Alert>
        </div>
      </div>
    </TriStateList>

    <FormOverlay v-model:open="modalOpen" :title="editing ? '编辑订阅' : '新建订阅'" :width="480"
                 :loading="saving" destroy-on-close @submit="save">
      <div class="space-y-4">
        <div>
          <div class="mb-1 text-sm">平台</div>
          <AppSelect v-model:value="form.platform_id" class="w-full" :disabled="!!editing" :options="platformOptions()"
                  placeholder="选择平台" />
          <div v-if="editing" class="text-xs text-text-tertiary mt-1">平台创建后不可修改</div>
        </div>
        <div>
          <div class="mb-1 text-sm">名称</div>
          <Input v-model:value="form.name" :maxlength="100" placeholder="订阅名称（不强制唯一）" />
        </div>
        <div v-if="editing">
          <div class="mb-1 text-sm">标识（系统自动生成，创建后不可修改）</div>
          <TypographyText code>{{ editing.slug }}</TypographyText>
        </div>
      </div>
      <template #footer>
        <Button class="touch-target" @click="modalOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="saving" @click="save">{{ editing ? '保存' : '创建' }}</Button>
      </template>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除订阅" danger :loading="deleting"
                  content="将删除该订阅的全部版本文件与指向它的下载 Token；装配蓝图级联删除将触发候选集重算（高级模式下可能摘除受影响的组节点分配并移除对应 Xray 账号）；不级联自定义订阅。删除后不可恢复。"
                  @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
