<!-- ProxyGroupsView.vue：代理组管理页（Design2-UI §7）——预设/自建双态列表 + DAG 校验 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Checkbox, Form, Input, InputNumber, Select, Space, Switch, Table, Tag } from 'ant-design-vue'
import { listProxyGroups, createProxyGroup, updateProxyGroup, deleteProxyGroup, togglePresetGroup, type ProxyGroupItem, type ProxyGroupDefinition } from '@/api/proxyGroup'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const loading = ref(false)
const groups = ref<ProxyGroupItem[]>([])
const editing = ref<ProxyGroupItem | null>(null)
const creating = ref(false)
const toDelete = ref<ProxyGroupItem | null>(null)
const deleting = ref(false)
const saving = ref(false)

const form = reactive({
  name: '',
  group_type: 'select' as ProxyGroupDefinition['type'],
  group_names: [] as string[],
  use: '', url: '', expected_status: '', interval: 0, timeout: 0, max_failed_times: 0,
  lazy: false, disable_udp: false, interface_name: '', routing_mark: 0,
  filter: '', exclude_filter: '', exclude_type: '', include_all: false,
  include_all_proxies: false, include_all_providers: false, hidden: false, icon: '',
})

function resetAdvanced(def?: ProxyGroupDefinition) {
  form.use = (def?.use ?? []).join(', ')
  form.url = def?.url ?? ''
  form.expected_status = def?.['expected-status'] ?? ''
  form.interval = def?.interval ?? 0
  form.timeout = def?.timeout ?? 0
  form.max_failed_times = def?.['max-failed-times'] ?? 0
  form.lazy = def?.lazy ?? false
  form.disable_udp = def?.['disable-udp'] ?? false
  form.interface_name = def?.['interface-name'] ?? ''
  form.routing_mark = def?.['routing-mark'] ?? 0
  form.filter = def?.filter ?? ''
  form.exclude_filter = def?.['exclude-filter'] ?? ''
  form.exclude_type = def?.['exclude-type'] ?? ''
  form.include_all = def?.['include-all'] ?? false
  form.include_all_proxies = def?.['include-all-proxies'] ?? false
  form.include_all_providers = def?.['include-all-providers'] ?? false
  form.hidden = def?.hidden ?? false
  form.icon = def?.icon ?? ''
}

const FORCE_SUBGROUPS = ['🚀直接连接', '🌎国外流量']
const availableGroupNames = computed(() => new Set(groups.value.filter((g) => g.id !== editing.value?.id).map((g) => g.name)))
const staleGroupNames = computed(() => form.group_names.filter((n) => !FORCE_SUBGROUPS.includes(n) && !availableGroupNames.value.has(n)))
const staleRefCount = computed(() => staleGroupNames.value.length)

function removeGroupRef(name: string) {
  form.group_names = form.group_names.filter((n) => n !== name)
}
function removeAllStaleRefs() {
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
  loading.value = true
  try {
    groups.value = await listProxyGroups()
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
  form.group_names = []
  resetAdvanced()
}
function openEdit(g: ProxyGroupItem) {
  editing.value = g
  creating.value = true
  form.name = g.name
  form.group_type = g.definition.type
  form.group_names = [...(g.definition.groups ?? [])]
  resetAdvanced(g.definition)
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
      groups: form.group_names,
      use: form.use.split(',').map((v) => v.trim()).filter(Boolean),
      url: form.url.trim(),
      'expected-status': form.expected_status.trim(),
      interval: form.interval,
      timeout: form.timeout,
      'max-failed-times': form.max_failed_times,
      lazy: form.lazy,
      'disable-udp': form.disable_udp,
      'interface-name': form.interface_name.trim(),
      'routing-mark': form.routing_mark,
      filter: form.filter.trim(),
      'exclude-filter': form.exclude_filter.trim(),
      'exclude-type': form.exclude_type.trim(),
      'include-all': form.include_all,
      'include-all-proxies': form.include_all_proxies,
      'include-all-providers': form.include_all_providers,
      hidden: form.hidden,
      icon: form.icon.trim(),
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
  return [...(g.definition.groups ?? [])].join('、') || '空'
}
</script>

<template>
  <div>
    <PageHeader title="代理组管理">
      <template #actions>
        <Button type="primary" @click="openCreate">新建代理组</Button>
      </template>
    </PageHeader>

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
          <div class="text-xs text-text-secondary mt-1">{{ memberSummary(g) }}</div>
          <div class="mobile-actions mt-2 flex items-center gap-2 flex-wrap">
            <Checkbox v-if="g.type === 'preset'" :checked="g.enabled" @change="(e: any) => onTogglePreset(g, e.target.checked)" />
            <Button size="small" @click="openEdit(g)">编辑</Button>
            <Button size="small" danger :disabled="g.type === 'preset'" @click="toDelete = g">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <FormOverlay :open="creating" :title="editing ? '编辑代理组' : '新建代理组'" :width="720" :loading="saving"
                 @submit="save" @update:open="creating = false">
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
          <AppSelect v-model:value="form.group_type">
            <Select.Option value="select">select</Select.Option>
            <Select.Option value="url-test">url-test</Select.Option>
            <Select.Option value="fallback">fallback</Select.Option>
            <Select.Option value="load-balance">load-balance</Select.Option>
            <Select.Option value="relay">relay</Select.Option>
          </AppSelect>
        </Form.Item>

        <Form.Item label="子组引用">
          <AppSelect
            mode="multiple"
            :value="form.group_names"
            placeholder="可引用强制组或其它代理组"
            class="w-full"
            @change="onGroupRefsChange"
          >
            <Select.Option value="🚀直接连接">🚀直接连接</Select.Option>
            <Select.Option value="🌎国外流量">🌎国外流量</Select.Option>
            <Select.Option v-for="g in groups.filter((x) => x.id !== editing?.id)" :key="g.name" :value="g.name">{{ g.name }}</Select.Option>
          </AppSelect>
          <div v-for="name in form.group_names" :key="name" class="mt-1 flex items-center gap-2">
            <span class="text-sm">{{ name }}</span>
            <Tag v-if="staleGroupNames.includes(name)" color="red">已失效</Tag>
            <Button v-if="staleGroupNames.includes(name)" size="small" danger @click="removeGroupRef(name)">剔除</Button>
          </div>
        </Form.Item>

        <details class="border rounded-lg p-3">
          <summary class="cursor-pointer font-medium">高级字段</summary>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3 mt-3">
            <Form.Item label="Provider 引用（逗号分隔）"><Input v-model:value="form.use" /></Form.Item>
            <Form.Item label="健康检查 URL"><Input v-model:value="form.url" placeholder="https://www.gstatic.com/generate_204" /></Form.Item>
            <Form.Item label="期望状态码"><Input v-model:value="form.expected_status" placeholder="204" /></Form.Item>
            <Form.Item label="检查间隔（秒）"><InputNumber v-model:value="form.interval" :min="0" class="w-full" /></Form.Item>
            <Form.Item label="超时（毫秒）"><InputNumber v-model:value="form.timeout" :min="0" class="w-full" /></Form.Item>
            <Form.Item label="最大失败次数"><InputNumber v-model:value="form.max_failed_times" :min="0" class="w-full" /></Form.Item>
            <Form.Item label="出站网卡"><Input v-model:value="form.interface_name" /></Form.Item>
            <Form.Item label="路由标记"><InputNumber v-model:value="form.routing_mark" :min="0" class="w-full" /></Form.Item>
            <Form.Item label="过滤表达式"><Input v-model:value="form.filter" /></Form.Item>
            <Form.Item label="排除表达式"><Input v-model:value="form.exclude_filter" /></Form.Item>
            <Form.Item label="排除类型（| 分隔）"><Input v-model:value="form.exclude_type" placeholder="Direct|Reject" /></Form.Item>
            <Form.Item label="图标"><Input v-model:value="form.icon" /></Form.Item>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.lazy" /><span>懒检查</span></label>
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.disable_udp" /><span>禁用 UDP</span></label>
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.include_all" /><span>引入全部</span></label>
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.include_all_proxies" /><span>引入全部代理</span></label>
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.include_all_providers" /><span>引入全部 Provider</span></label>
            <label class="flex items-center gap-2 whitespace-nowrap text-sm"><Switch v-model:checked="form.hidden" /><span>隐藏</span></label>
          </div>
        </details>
      </Form>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除代理组" danger :loading="deleting" :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
