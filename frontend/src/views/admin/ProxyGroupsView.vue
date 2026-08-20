<!-- ProxyGroupsView.vue：代理组管理页（Design2-UI §7）——预设/自建双态列表 + DAG 校验 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Checkbox, Form, Input, Modal, Select, Space, Table, Tag } from 'ant-design-vue'
import { HolderOutlined } from '@ant-design/icons-vue'
import { listProxyGroups, createProxyGroup, updateProxyGroup, deleteProxyGroup, togglePresetGroup, type ProxyGroupItem, type ProxyGroupDefinition } from '@/api/proxyGroup'
import { listNodes, type NodeItem } from '@/api/node'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { useSortableList } from '@/composables/useSortableList'
import { Notify } from '@/components/Notify'

const loading = ref(false)
const groups = ref<ProxyGroupItem[]>([])
const nodes = ref<NodeItem[]>([])
const isMobile = ref(false)
const editing = ref<ProxyGroupItem | null>(null)
const creating = ref(false)
const toDelete = ref<ProxyGroupItem | null>(null)
const deleting = ref(false)
const saving = ref(false)

const form = reactive({
  name: '',
  group_type: 'select' as 'select' | 'url-test' | 'fallback',
  node_names: [] as string[],
  group_names: [] as string[],
})
const addNodeName = ref<string | undefined>(undefined)
const nodeList = computed<string[]>({
  get: () => form.node_names,
  set: (v) => { form.node_names = v },
})
const { move: nodeMove, up: nodeUp, down: nodeDown } = useSortableList(nodeList)
const dragIndex = ref<number | null>(null)
function onDragStart(idx: number) { dragIndex.value = idx }
function onDrop(idx: number) {
  if (dragIndex.value !== null && dragIndex.value !== idx) nodeMove(dragIndex.value, idx)
  dragIndex.value = null
}

const FORCE_SUBGROUPS = ['🚀直接连接', '🌎国外流量']
const availableNodeNames = computed(() => new Set(nodes.value.map((n) => n.name)))
const availableGroupNames = computed(() => new Set(groups.value.filter((g) => g.id !== editing.value?.id).map((g) => g.name)))
const staleNodeNames = computed(() => form.node_names.filter((n) => !availableNodeNames.value.has(n)))
const staleGroupNames = computed(() => form.group_names.filter((n) => !FORCE_SUBGROUPS.includes(n) && !availableGroupNames.value.has(n)))
const staleRefCount = computed(() => staleNodeNames.value.length + staleGroupNames.value.length)

function removeNodeRef(name: string) {
  form.node_names = form.node_names.filter((n) => n !== name)
}
function removeGroupRef(name: string) {
  form.group_names = form.group_names.filter((n) => n !== name)
}
function removeAllStaleRefs() {
  staleNodeNames.value.forEach(removeNodeRef)
  staleGroupNames.value.forEach(removeGroupRef)
}

// 前端 DAG 环检测：与后端同口径，保存前拦截。
function detectCycle(): string | null {
  const currentName = editing.value?.name || form.name || '（新建）'
  const adj = new Map<string, string[]>()
  for (const g of groups.value) {
    if (editing.value && g.id === editing.value.id) {
      adj.set(g.name, [...form.group_names])
    } else {
      adj.set(g.name, [...(g.definition.groups ?? [])])
    }
  }
  if (!editing.value) adj.set(currentName, [...form.group_names])
  const color = new Map<string, 0 | 1 | 2>()
  const dfs = (name: string): string | null => {
    color.set(name, 1)
    for (const next of adj.get(name) ?? []) {
      if (FORCE_SUBGROUPS.includes(next)) continue
      const nextColor = color.get(next)
      if (nextColor === 1) return `${name} → ${next}`
      if (!nextColor) {
        const cycle = dfs(next)
        if (cycle) return cycle
      }
    }
    color.set(name, 2)
    return null
  }
  for (const name of adj.keys()) {
    if (!color.get(name)) {
      const cycle = dfs(name)
      if (cycle) return cycle
    }
  }
  return null
}

// R14-14：选择子组时即时 DAG 检测（保存时仍会二次兜底）
function onGroupRefsChange(v: any) {
  form.group_names = v as string[] ?? []
  const cycle = detectCycle()
  if (cycle) Notify.warning(`代理组存在环：${cycle}`)
}

async function load() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
  loading.value = true
  try {
    groups.value = await listProxyGroups()
    nodes.value = await listNodes('manual')
    const xray = await listNodes('xray')
    nodes.value = nodes.value.concat(xray)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(() => void load())

function openCreate() {
  creating.value = true
  editing.value = null
  form.name = ''
  form.group_type = 'select'
  form.node_names = []
  form.group_names = []
}
function openEdit(g: ProxyGroupItem) {
  editing.value = g
  creating.value = true
  form.name = g.name
  form.group_type = g.definition.type
  form.node_names = [...(g.definition.nodes ?? [])]
  form.group_names = [...(g.definition.groups ?? [])]
}
async function save() {
  if (staleRefCount.value > 0) {
    Notify.error('存在失效引用，请先剔除后再保存')
    return
  }
  const cycle = detectCycle()
  if (cycle) {
    Notify.error(`代理组存在环：${cycle}`)
    return
  }
  saving.value = true
  try {
    const definition: ProxyGroupDefinition = {
      type: form.group_type,
      nodes: form.node_names,
      groups: form.group_names,
    }
    if (editing.value) {
      await updateProxyGroup(editing.value.id, { group_type: form.group_type, definition })
    } else {
      await createProxyGroup({ name: form.name, group_type: form.group_type, definition })
    }
    Notify.success(editing.value ? '代理组已更新' : '代理组已创建')
    creating.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}
async function onTogglePreset(g: ProxyGroupItem, enabled: boolean) {
  try {
    await togglePresetGroup(g.id, enabled)
    g.enabled = enabled
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  }
}
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteProxyGroup(toDelete.value.id)
    Notify.success('代理组已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}
const deleteContent = computed(() => {
  const g = toDelete.value
  if (!g) return ''
  return `将删除代理组「${g.name}」。若被其他代理组引用为子组，将产生悬空引用（编辑时红标提示）。\n删除后不可恢复。`
})

function memberSummary(g: ProxyGroupItem): string {
  return [...(g.definition.nodes ?? []), ...(g.definition.groups ?? [])].join('、') || '空'
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold m-0">代理组管理</h2>
      <Button type="primary" @click="openCreate">新建代理组</Button>
    </div>

    <TriStateList :loading="loading" :empty="groups.length === 0" empty-text="暂无代理组">
      <Table :data-source="groups" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="name" title="名称">
          <template #default="{ record }">
            <Space>
              <Checkbox v-if="record.type === 'preset'" :checked="record.enabled" @change="(e: any) => onTogglePreset(record, e.target.checked)" />
              <span>{{ record.name }}</span>
            </Space>
          </template>
        </Table.Column>
        <Table.Column key="type" title="类型" width="110">
          <template #default="{ record }">
            <Tag :color="record.type === 'preset' ? 'blue' : 'green'">{{ record.type }}</Tag>
            <Tag>{{ record.definition.type }}</Tag>
          </template>
        </Table.Column>
        <Table.Column key="members" title="成员">
          <template #default="{ record }">{{ memberSummary(record) }}</template>
        </Table.Column>
        <Table.Column key="enabled" title="启用" width="90">
          <template #default="{ record }">
            <Checkbox v-if="record.type === 'preset'" :checked="record.enabled" @change="(e: any) => onTogglePreset(record, e.target.checked)" />
          </template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="180">
          <template #default="{ record }">
            <Space>
              <Button size="small" @click="openEdit(record)">编辑</Button>
              <Button size="small" danger :disabled="record.type === 'preset'" @click="toDelete = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="g in groups" :key="g.id" class="border rounded-lg p-3">
          <div class="flex items-center justify-between">
            <Space>
              <Checkbox v-if="g.type === 'preset'" :checked="g.enabled" @change="(e: any) => onTogglePreset(g, e.target.checked)" />
              <span class="font-medium">{{ g.name }}</span>
            </Space>
            <Tag :color="g.type === 'preset' ? 'blue' : 'green'">{{ g.type }}</Tag>
          </div>
          <div class="text-xs text-gray-500 mt-1">{{ memberSummary(g) }}</div>
          <div class="mt-2 flex items-center gap-2">
            <Checkbox v-if="g.type === 'preset'" :checked="g.enabled" @change="(e: any) => onTogglePreset(g, e.target.checked)" />
            <Button size="small" @click="openEdit(g)">编辑</Button>
            <Button size="small" danger :disabled="g.type === 'preset'" @click="toDelete = g">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <Modal :open="creating" :title="editing ? '编辑代理组' : '新建代理组'" :width="720" :confirm-loading="saving" @ok="save" @cancel="creating = false">
      <Form layout="vertical">
        <Alert v-if="staleRefCount > 0" type="error" show-icon class="mb-3"
               :message="`${staleRefCount} 项引用已失效，请剔除或替换后保存`">
          <template #action>
            <Button size="small" danger @click="removeAllStaleRefs">一键剔除全部失效项</Button>
          </template>
        </Alert>
        <Form.Item label="名称" required>
          <Input v-model:value="form.name" :disabled="editing !== null" placeholder="允许空格，禁止逗号与首尾空白" />
        </Form.Item>
        <Form.Item label="组类型" required>
          <Select v-model:value="form.group_type">
            <Select.Option value="select">select</Select.Option>
            <Select.Option value="url-test">url-test</Select.Option>
            <Select.Option value="fallback">fallback</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item label="节点引用（有序）">
          <div class="space-y-2">
            <div v-for="(name, idx) in form.node_names" :key="name" :draggable="!isMobile" class="flex items-center gap-2"
                 @dragstart="onDragStart(idx)" @dragover.prevent @drop="onDrop(idx)">
              <HolderOutlined v-if="!isMobile" class="cursor-move text-gray-400" />
              <span class="flex-1">{{ name }}</span>
              <Tag v-if="staleNodeNames.includes(name)" color="red">已失效</Tag>
              <Button v-if="staleNodeNames.includes(name)" size="small" danger @click="removeNodeRef(name)">剔除</Button>
              <Button size="small" @click="nodeUp(idx)">上移</Button>
              <Button size="small" @click="nodeDown(idx)">下移</Button>
              <Button size="small" danger @click="form.node_names.splice(idx, 1)">移除</Button>
            </div>
            <Select
              v-model:value="addNodeName"
              placeholder="添加节点（提交 nodes.name 稳定键）"
              class="w-full"
              @change="(v: any) => { if (v && !form.node_names.includes(v)) form.node_names.push(v as string); addNodeName = undefined }"
            >
              <Select.Option v-for="n in nodes" :key="n.name" :value="n.name">{{ n.render_name }}<span v-if="n.source === 'xray' && n.display_name" class="text-xs text-gray-400">（{{ n.name }}）</span></Select.Option>
            </Select>
          </div>
        </Form.Item>
        <Form.Item label="子组引用">
          <Select
            mode="multiple"
            :value="form.group_names"
            placeholder="可引用强制组或其它代理组"
            class="w-full"
            @change="onGroupRefsChange"
          >
            <Select.Option value="🚀直接连接">🚀直接连接</Select.Option>
            <Select.Option value="🌎国外流量">🌎国外流量</Select.Option>
            <Select.Option v-for="g in groups.filter((x) => x.id !== editing?.id)" :key="g.name" :value="g.name">{{ g.name }}</Select.Option>
          </Select>
          <div v-for="name in form.group_names" :key="name" class="mt-1 flex items-center gap-2">
            <span class="text-sm">{{ name }}</span>
            <Tag v-if="staleGroupNames.includes(name)" color="red">已失效</Tag>
            <Button v-if="staleGroupNames.includes(name)" size="small" danger @click="removeGroupRef(name)">剔除</Button>
          </div>
        </Form.Item>
      </Form>
    </Modal>

    <ConfirmModal :open="toDelete !== null" title="删除代理组" danger :loading="deleting" :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
