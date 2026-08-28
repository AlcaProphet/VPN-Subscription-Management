<!-- GroupsView.vue：用户组管理（Build7 高级：节点分配 + 默认配额 + 候选集引导） -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Button, Checkbox, Empty, Input, InputNumber, Table, Tag } from 'ant-design-vue'
import {
  listGroups, getGroup, createGroup, updateGroup, deleteGroup, updateGroupNodes, updateGroupQuota,
  type GroupItem, type CandidateNode, type GroupDetail,
} from '@/api/group'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import FormSection from '@/components/FormSection.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const loading = ref(false)
const groups = ref<GroupItem[]>([])

async function load() {
  loading.value = true
  try {
    groups.value = await listGroups()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

const quotaText = (g: GroupItem) =>
  g.default_quota == null ? '不限流量' : `${g.default_quota} GB`

// --- 新建组 ---
const createOpen = ref(false)
const newName = ref('')
const creating = ref(false)
async function doCreate() {
  if (!newName.value.trim()) { Notify.error('组名不能为空'); return }
  creating.value = true
  try {
    await createGroup(newName.value.trim())
    Notify.success('组已创建')
    createOpen.value = false
    newName.value = ''
    await load()
  } catch (err) { Notify.error((err as Error).message) } finally { creating.value = false }
}

// --- 编辑：名称 + 节点分配 + 默认配额 ---
const editOpen = ref(false)
const editing = ref<GroupDetail | null>(null)
const editName = ref('')
const selectedNodeIDs = ref<number[]>([])
const candidateNodes = ref<CandidateNode[]>([])
const editQuota = ref<number | undefined>(undefined)
const saving = ref(false)
const assignedNonCandidate = computed(() =>
  (editing.value?.nodes ?? []).filter((n) => !(candidateNodes.value ?? []).some((c) => c.node_id === n.node_id)),
)
const selectedNodes = computed(() =>
  selectedNodeIDs.value
    .map((id) => {
      const n = editing.value?.nodes?.find((x) => x.node_id === id)
      const c = candidateNodes.value.find((x) => x.node_id === id)
      return {
        node_id: id,
        name: n?.node_name || c?.name || String(id),
        render_name: n?.render_name || c?.render_name || c?.name || '',
        is_public: !!n?.is_public,
        in_candidate: !!c,
      }
    })
    .filter(Boolean),
)
function moveSelected(index: number, dir: -1 | 1) {
  const target = index + dir
  if (target < 0 || target >= selectedNodeIDs.value.length) return
  const arr = [...selectedNodeIDs.value]
  ;[arr[index], arr[target]] = [arr[target], arr[index]]
  selectedNodeIDs.value = arr
}
function removeSelected(nodeId: number) {
  selectedNodeIDs.value = selectedNodeIDs.value.filter((id) => id !== nodeId)
}
async function openEdit(g: GroupItem) {
  try {
    const detail = await getGroup(g.id)
    editing.value = detail
    editName.value = detail.name
    selectedNodeIDs.value = (detail.nodes ?? []).map((n) => n.node_id)
    candidateNodes.value = detail.candidate_nodes ?? []
    editQuota.value = detail.default_quota ?? undefined
    editOpen.value = true
  } catch (err) { Notify.error((err as Error).message) }
}
async function doSaveEdit() {
  if (!editing.value || !editName.value.trim()) return
  saving.value = true
  try {
    await updateGroup(editing.value.id, { name: editName.value.trim() })
    await updateGroupNodes(editing.value.id, { node_ids: selectedNodeIDs.value })
    await updateGroupQuota(editing.value.id, { default_quota: editQuota.value })
    Notify.success('已保存，节点变更将同步至 Xray')
    editOpen.value = false
    await load()
  } catch (err) { Notify.error((err as Error).message) } finally { saving.value = false }
}

const toDelete = ref<GroupItem | null>(null)
const deleting = ref(false)
const deleteContent = computed(() =>
  toDelete.value ? `组内 ${toDelete.value.user_count} 名用户将自动迁入默认组` : '',
)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteGroup(toDelete.value.id)
    Notify.success('组已删除，成员已迁入默认组')
    toDelete.value = null
    await load()
  } catch (err) { Notify.error((err as Error).message) } finally { deleting.value = false }
}
</script>

<template>
  <div>
    <PageHeader title="用户组管理" subtitle="用户组决定成员可用的 Xray 节点与默认流量配额；节点变更会同步到受影响账号。">
      <template #actions>
        <Button type="primary" @click="createOpen = true">新建组</Button>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="groups.length === 0" empty-text="暂无用户组">
      <Table :data-source="groups" row-key="id" :pagination="false">
        <Table.Column key="name" title="组名">
          <template #default="{ record }">
            {{ record.name }}
            <Tag v-if="record.is_default" color="blue">默认组</Tag>
          </template>
        </Table.Column>
        <Table.Column key="quota" title="默认配额">
          <template #default="{ record }">{{ quotaText(record) }}</template>
        </Table.Column>
        <Table.Column key="nodes" title="分配节点数" data-index="node_count" width="120" />
        <Table.Column key="users" title="用户数" data-index="user_count" width="100" />
        <Table.Column key="actions" title="操作" width="160">
          <template #default="{ record }">
            <Button size="small" @click="openEdit(record)">编辑</Button>
            <Button v-if="!record.is_default" size="small" danger class="ml-1" @click="toDelete = record">删除</Button>
          </template>
        </Table.Column>
      </Table>
    </TriStateList>

    <FormOverlay v-model:open="createOpen" title="新建组" :width="420" :loading="creating" destroy-on-close @submit="doCreate">
      <Input v-model:value="newName" :maxlength="64" placeholder="组名（全局唯一）" @press-enter="doCreate" />
      <template #footer>
        <Button class="touch-target" @click="createOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="creating" @click="doCreate">创建</Button>
      </template>
    </FormOverlay>

    <FormOverlay :open="editOpen" title="编辑组" :width="560" :loading="saving" destroy-on-close
                 @submit="doSaveEdit" @update:open="editOpen = false">
      <div class="space-y-4">
        <FormSection title="基础信息" help="名称在全局范围内唯一。">
          <Input v-model:value="editName" :maxlength="64" />
        </FormSection>
        <FormSection title="可用节点" help="仅候选集节点会注入到下载内容；公共节点对全部用户组自动可见。">
          <div class="text-xs text-gray-400 mb-1">节点分配（候选集）</div>
          <Empty v-if="candidateNodes.length === 0 && selectedNodes.length === 0" description="请先装配并激活 Clash YAML / SR 节点订阅 / 通用节点订阅模板">
            <Button type="primary" @click="$router.push('/admin/assembly')">前往装配</Button>
          </Empty>
          <div v-if="assignedNonCandidate.length > 0" class="mb-2">
            <Tag color="red">存在非候选集已分配节点</Tag>
            <div class="text-xs text-red-500">{{ assignedNonCandidate.map((n) => n.node_name).join('、') }} 不在当前候选集，保存后可能被候选集重算摘除。</div>
          </div>
          <div v-if="candidateNodes.length > 0" class="space-y-1 mb-3">
            <div v-for="c in candidateNodes" :key="c.name" class="flex items-center gap-2">
              <Checkbox :checked="selectedNodeIDs.includes(c.node_id)" :disabled="c.is_public" @change="() => {
                if (c.is_public) return
                const id = c.node_id
                if (selectedNodeIDs.includes(id)) selectedNodeIDs = selectedNodeIDs.filter((x) => x !== id)
                else selectedNodeIDs = [...selectedNodeIDs, id]
              }">
                <span>{{ c.render_name || c.name }}</span>
                <span v-if="c.render_name && c.render_name !== c.name" class="ml-1 text-xs text-gray-400">{{ c.name }}</span>
              </Checkbox>
              <Tag v-if="c.is_public" color="default">公共·免分配</Tag>
              <Tag v-if="c.in_partial_blueprint" color="orange">仅部分模板</Tag>
            </div>
          </div>
          <div v-if="selectedNodes.length > 0">
            <div class="text-xs text-gray-400 mb-1">分配排序（顺序将写入 sort_order）</div>
            <div v-for="(n, i) in selectedNodes" :key="n.node_id" class="flex items-center gap-2 py-1">
              <span class="w-6 text-gray-400">{{ i + 1 }}</span>
              <span :class="n.is_public ? 'text-gray-400' : ''" class="flex-1 text-sm">{{ n.render_name || n.name }}</span>
              <Tag v-if="n.is_public" color="default">公共·免分配</Tag>
              <Tag v-if="!n.in_candidate" color="red">非候选</Tag>
              <Button size="small" :disabled="i === 0" @click="moveSelected(i, -1)">↑</Button>
              <Button size="small" :disabled="i === selectedNodes.length - 1" @click="moveSelected(i, 1)">↓</Button>
              <Button size="small" danger @click="removeSelected(n.node_id)">移除</Button>
            </div>
          </div>
        </FormSection>
        <FormSection title="流量" help="设置该组的默认配额，用户级覆盖优先于此配置。">
          <div class="text-xs text-gray-400 mb-1">默认配额（GB，0/留空不限）</div>
          <InputNumber v-model:value="editQuota" :min="0" class="w-40" />
        </FormSection>
      </div>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除用户组" danger :loading="deleting"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
