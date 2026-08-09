<!-- GroupsView.vue：用户组与订阅分发管理（UI §5.2）——双态列表 + 编辑弹窗（改名/关联/每平台选定） -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Input, Modal, Select, Space, Table, Tag } from 'ant-design-vue'
import { listGroups, getGroup, createGroup, updateGroup, deleteGroup, type GroupDetail, type GroupItem, type SelectionItem } from '@/api/group'
import { listPlatforms, type PlatformItem } from '@/api/platform'
import { listSubscriptions, type PlatformSubs, type SubscriptionItem } from '@/api/subscription'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const loading = ref(false)
const groups = ref<GroupItem[]>([])
const platforms = ref<PlatformItem[]>([])
const subsByPlatform = ref<PlatformSubs[]>([])

async function load() {
  loading.value = true
  try {
    const [g, p, s] = await Promise.all([listGroups(), listPlatforms(), listSubscriptions()])
    groups.value = g
    platforms.value = p
    subsByPlatform.value = s
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 首管理员首次登录一次性引导条（localStorage 键记录关闭）
const showGuide = computed(() => localStorage.getItem('first_admin_guide_dismissed') !== '1')
function dismissGuide() {
  localStorage.setItem('first_admin_guide_dismissed', '1')
}

// 该平台订阅选项（供选定区 Select）
function subOptions(platformId: number) {
  const pg = subsByPlatform.value.find((x) => x.platform_id === platformId)
  return (pg?.subscriptions ?? []).map((s: SubscriptionItem) => ({ label: s.name, value: s.id }))
}

// --- 编辑弹窗（改名 + 关联订阅多选 + 每平台选定区） ---
const editOpen = ref(false)
const editing = ref<GroupDetail | null>(null)
const saving = ref(false)
const form = reactive({ name: '', sub_ids: [] as number[], selections: [] as SelectionItem[] })
// 初始选定快照：变更时提示影响用户数
const initialSelections = ref<SelectionItem[]>([])
const selectionChanged = computed(() => {
  const a = JSON.stringify(form.selections)
  const b = JSON.stringify(initialSelections.value)
  return a !== b
})

async function openEdit(g: GroupItem) {
  try {
    const detail = await getGroup(g.id)
    editing.value = detail
    form.name = detail.name
    form.sub_ids = []
    form.selections = []
    initialSelections.value = JSON.parse(JSON.stringify(detail.selections ?? []))
    form.selections = JSON.parse(JSON.stringify(detail.selections ?? []))
    await loadRel(detail.id)
    editOpen.value = true
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

// 关联订阅回显：订阅列表已带 groups 字段，反查该组关联的订阅
async function loadRel(groupId: number) {
  const all = subsByPlatform.value.flatMap((pg) => pg.subscriptions)
  const rel = all.filter((s) => s.groups?.some((g) => g.id === groupId))
  form.sub_ids = rel.map((s) => s.id)
}

// 每平台选定更新（subscription_id=0 表示取消选定）
function setSelection(platformId: number, subscriptionId: number) {
  const idx = form.selections.findIndex((s) => s.platform_id === platformId)
  if (idx >= 0) {
    if (subscriptionId === 0) {
      form.selections.splice(idx, 1)
    } else {
      form.selections[idx].subscription_id = subscriptionId
    }
  } else if (subscriptionId !== 0) {
    form.selections.push({ platform_id: platformId, subscription_id: subscriptionId })
  }
}
function onSelectionChange(platformId: number, value: unknown) {
  setSelection(platformId, Number(value))
}

// 选定变更影响提示：影响 N 名用户
const affectedUsers = computed(() => editing.value?.user_count ?? 0)

async function saveEdit() {
  if (!editing.value) return
  if (!form.name.trim()) {
    Notify.error('组名不能为空')
    return
  }
  saving.value = true
  try {
    await updateGroup(editing.value.id, {
      name: form.name.trim(),
      sub_ids: form.sub_ids,
      selections: form.selections,
    })
    Notify.success('组已更新')
    editOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message) // 「该组正在选定此订阅，请先在选定区改选」等
  } finally {
    saving.value = false
  }
}

// --- 新建组 ---
const createOpen = ref(false)
const newName = ref('')
const creating = ref(false)
async function doCreate() {
  if (!newName.value.trim()) {
    Notify.error('组名不能为空')
    return
  }
  creating.value = true
  try {
    await createGroup(newName.value.trim())
    Notify.success('组已创建')
    createOpen.value = false
    newName.value = ''
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}

// --- 删除组（迁入默认组） ---
const toDelete = ref<GroupItem | null>(null)
const deleting = ref(false)
const deleteContent = computed(() =>
  toDelete.value ? `组内 ${toDelete.value.user_count} 名用户将自动迁入默认组，关联与选定将被清理` : '',
)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteGroup(toDelete.value.id)
    Notify.success('组已删除，成员已迁入默认组')
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
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold m-0">用户组管理</h2>
      <Button type="primary" @click="createOpen = true">新建组</Button>
    </div>

    <!-- 分发引导：一次性 a-alert「创建第一份订阅」 -->
    <Alert v-if="showGuide" type="info" closable class="mb-4" message="还没有订阅内容？"
           description="前往订阅管理创建第一份订阅，再为各用户组选定分发" @close="dismissGuide" />

    <TriStateList :loading="loading" :empty="groups.length === 0" empty-text="暂无用户组">
      <!-- ≥768：表格 -->
      <Table :data-source="groups" row-key="id" :pagination="false" class="hidden md:block"
             :row-class-name="(r: GroupItem) => (r.needs_reselect ? 'row-warn' : '')">
        <Table.Column key="name" title="组名" data-index="name">
          <template #default="{ record }">
            <Space>
              {{ record.name }}
              <Tag v-if="record.is_default" color="gold">默认组</Tag>
              <Tag v-if="record.needs_reselect" color="orange">需要重新选定</Tag>
            </Space>
          </template>
        </Table.Column>
        <Table.Column key="subs" title="关联订阅数" width="110">
          <template #default="{ record }">{{ record.sub_count }}</template>
        </Table.Column>
        <Table.Column key="users" title="组内用户数" width="110">
          <template #default="{ record }">{{ record.user_count }}</template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="220">
          <template #default="{ record }">
            <Space>
              <Button size="small" type="primary" ghost @click="openEdit(record)">编辑</Button>
              <Button v-if="record.needs_reselect" size="small" @click="openEdit(record)">重新选定</Button>
              <Button v-if="!record.is_default" size="small" danger @click="toDelete = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <!-- <768：卡片（移动端易用性，与平台/订阅卡片风格一致） -->
      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="g in groups" :key="g.id"
             class="border rounded-lg p-3 bg-white dark:bg-gray-800"
             :class="g.needs_reselect ? 'border-orange-300' : ''">
          <div class="flex items-center justify-between gap-2">
            <span class="font-medium truncate">{{ g.name }}</span>
            <div class="flex gap-1 shrink-0">
              <Tag v-if="g.is_default" color="gold">默认组</Tag>
              <Tag v-if="g.needs_reselect" color="orange">需重选</Tag>
            </div>
          </div>
          <div class="text-xs text-gray-500 mt-1">关联订阅 {{ g.sub_count }} · 组内用户 {{ g.user_count }}</div>
          <div class="mt-2 flex flex-wrap gap-2">
            <Button size="small" type="primary" ghost @click="openEdit(g)">编辑</Button>
            <Button v-if="g.needs_reselect" size="small" @click="openEdit(g)">重新选定</Button>
            <Button v-if="!g.is_default" size="small" danger @click="toDelete = g">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <!-- 新建组弹窗 -->
    <Modal v-model:open="createOpen" title="新建用户组" :footer="null" :width="420" destroy-on-close>
      <Input v-model:value="newName" :maxlength="64" placeholder="组名（全局唯一）" @press-enter="doCreate" />
      <div class="flex justify-end mt-3">
        <Button type="primary" :loading="creating" @click="doCreate">创建</Button>
      </div>
    </Modal>

    <!-- 组编辑弹窗：改名 + 关联订阅多选 + 每平台选定区 -->
    <Modal v-model:open="editOpen" :title="`编辑组：${editing?.name ?? ''}`" :footer="null" :width="720"
           destroy-on-close>
      <div class="space-y-4">
        <div>
          <div class="mb-1 text-sm">组名</div>
          <Input v-model:value="form.name" :maxlength="64" />
        </div>
        <div>
          <div class="mb-1 text-sm">关联订阅（取消正被选定的订阅会被拒绝，请先在下方改选）</div>
          <Select v-model:value="form.sub_ids" mode="multiple" class="w-full" placeholder="选择关联的订阅">
            <Select.Option v-for="pg in subsByPlatform" :key="pg.platform_id" :label="pg.platform_name" disabled>
              {{ pg.platform_name }}
            </Select.Option>
            <template v-for="pg in subsByPlatform" :key="'opt-' + pg.platform_id">
              <Select.Option v-for="s in pg.subscriptions" :key="s.id" :value="s.id">
                {{ pg.platform_name }} / {{ s.name }}
              </Select.Option>
            </template>
          </Select>
        </div>
        <div>
          <div class="mb-1 text-sm flex items-center gap-2">
            每平台选定
            <span v-if="selectionChanged" class="text-xs text-orange-500">变更将影响 {{ affectedUsers }} 名用户</span>
          </div>
          <div v-for="p in platforms" :key="p.id" class="flex items-center gap-2 mb-2">
            <span class="w-32 text-sm truncate">{{ p.name }}</span>
            <Select class="flex-1" :value="form.selections.find((s) => s.platform_id === p.id)?.subscription_id ?? 0"
                    @change="onSelectionChange(p.id, $event)">
              <Select.Option :value="0">不选定</Select.Option>
              <Select.Option v-for="opt in subOptions(p.id)" :key="opt.value" :value="opt.value">{{ opt.label }}</Select.Option>
            </Select>
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <Button @click="editOpen = false">取消</Button>
          <Button type="primary" :loading="saving" @click="saveEdit">保存</Button>
        </div>
      </div>
    </Modal>

    <!-- 删除确认（迁入默认组） -->
    <ConfirmModal :open="toDelete !== null" title="删除用户组" danger :loading="deleting"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
