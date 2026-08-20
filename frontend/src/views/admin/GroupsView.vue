<!-- GroupsView.vue：用户组管理（Build4 最小化：基础 CRUD；Build7 重构为节点分配 + 默认配额高级 UI） -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Button, Input, Modal, Table, Tag } from 'ant-design-vue'
import { listGroups, getGroup, createGroup, updateGroup, deleteGroup, type GroupItem } from '@/api/group'
import ConfirmModal from '@/components/ConfirmModal.vue'
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

// --- 新建/改名 ---
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

const editOpen = ref(false)
const editing = ref<GroupItem | null>(null)
const editName = ref('')
const saving = ref(false)
async function openEdit(g: GroupItem) {
  try {
    const detail = await getGroup(g.id)
    editing.value = detail
    editName.value = detail.name
    editOpen.value = true
  } catch (err) { Notify.error((err as Error).message) }
}
async function doSaveEdit() {
  if (!editing.value || !editName.value.trim()) return
  saving.value = true
  try {
    await updateGroup(editing.value.id, { name: editName.value.trim() })
    Notify.success('组已更新')
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
    <PageHeader title="用户组管理">
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
        <Table.Column key="users" title="用户数" data-index="user_count" width="100" />
        <Table.Column key="actions" title="操作" width="160">
          <template #default="{ record }">
            <Button size="small" @click="openEdit(record)">编辑</Button>
            <Button v-if="!record.is_default" size="small" danger class="ml-1" @click="toDelete = record">删除</Button>
          </template>
        </Table.Column>
      </Table>
    </TriStateList>

    <Modal v-model:open="createOpen" title="新建组" :footer="null" :width="420" destroy-on-close>
      <Input v-model:value="newName" :maxlength="64" placeholder="组名（全局唯一）" @press-enter="doCreate" />
      <div class="flex justify-end mt-3">
        <Button type="primary" :loading="creating" @click="doCreate">创建</Button>
      </div>
    </Modal>

    <Modal :open="editOpen" title="编辑组" :footer="null" :width="420" destroy-on-close
           @cancel="editOpen = false">
      <div class="text-xs text-gray-400 mb-2">名称（全局唯一校验）</div>
      <Input v-model:value="editName" :maxlength="64" @press-enter="doSaveEdit" />
      <div class="flex justify-end mt-3">
        <Button type="primary" :loading="saving" @click="doSaveEdit">保存</Button>
      </div>
    </Modal>

    <ConfirmModal :open="toDelete !== null" title="删除用户组" danger :loading="deleting"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
