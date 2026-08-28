<!-- NodesView.vue：节点管理页（Design2-UI §6）——manual/xray 双态列表 + 动态表单 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Form, Input, InputNumber, Select, Space, Switch, Table, Tag, Tooltip } from 'ant-design-vue'
import { listNodes, getProtocols, createNode, updateNode, deleteNode, toggleNode, setNodeDisplayName, importNodes, type NodeItem, type ProtocolInfo, type NodeForm, type ImportLineResult } from '@/api/node'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'
import { ApiError } from '@/api/request'

const loading = ref(false)
const nodes = ref<NodeItem[]>([])
const protocols = ref<ProtocolInfo[]>([])
const editing = ref<NodeItem | null>(null)
const creating = ref(false)
const naming = ref<NodeItem | null>(null)
const namingError = ref('')
const displayNameInput = ref('')
const toDelete = ref<NodeItem | null>(null)
const deleting = ref(false)
const disableTarget = ref<NodeItem | null>(null)
const disabling = ref(false)
const publicTarget = ref<NodeItem | null>(null)
const publicChanging = ref(false)
const saving = ref(false)
const importOpen = ref(false)
const importText = ref('')
const importResults = ref<ImportLineResult[]>([])
const importing = ref(false)

const form = reactive<NodeForm>({
  name: '', protocol: 'vless', host: '', port: 443, protocol_json: {},
})
const nameReadonly = computed(() => editing.value !== null)
const nameSpaceError = computed(() => {
  if (!creating.value || nameReadonly.value) return ''
  return form.name.includes(' ') ? '名称禁止空格' : ''
})

async function load() {
  loading.value = true
  try {
    nodes.value = await listNodes()
    if (protocols.value.length === 0) protocols.value = await getProtocols()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(() => void load())

function currentSchema() {
  return protocols.value.find((p) => p.protocol === form.protocol) ?? null
}
function sensitiveFields() {
  return new Set(currentSchema()?.sensitive_fields ?? [])
}
function openCreate() {
  creating.value = true
  editing.value = null
  form.name = ''
  form.protocol = 'vless'
  form.host = ''
  form.port = 443
  form.protocol_json = {}
}
function openImport() {
  importOpen.value = true
  importText.value = ''
  importResults.value = []
}
async function doImport() {
  if (!importText.value.trim()) {
    Notify.warning('请先粘贴节点 URI')
    return
  }
  importing.value = true
  try {
    const res = await importNodes(importText.value)
    importResults.value = res.list
    await load()
    Notify.success(`导入完成：${res.list.filter((r) => r.ok).length} 成功，${res.list.filter((r) => !r.ok).length} 跳过`)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    importing.value = false
  }
}

function openEdit(n: NodeItem) {
  if (n.source !== 'manual') return
  editing.value = n
  creating.value = true
  form.name = n.name
  form.protocol = n.protocol
  form.host = n.host
  form.port = n.port
  form.protocol_json = { ...(n.protocol_json ?? {}) }
}
async function save() {
  saving.value = true
  try {
    if (editing.value) {
      await updateNode(editing.value.id, { ...form })
    } else {
      await createNode({ ...form })
    }
    Notify.success(editing.value ? '节点已更新' : '节点已创建')
    creating.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

async function onToggleEnabled(n: NodeItem, enabled: boolean) {
  if (!enabled) {
    disableTarget.value = n
    return
  }
  try {
    await toggleNode(n.id, { enabled })
    n.enabled = enabled
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  }
}
async function confirmDisable() {
  if (!disableTarget.value) return
  disabling.value = true
  try {
    await toggleNode(disableTarget.value.id, { enabled: false })
    disableTarget.value.enabled = false
    Notify.success('节点已停用')
    disableTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  } finally {
    disabling.value = false
  }
}
async function onTogglePublic(n: NodeItem, isPublic: boolean) {
  publicTarget.value = { ...n, is_public: isPublic }
}
async function confirmPublic() {
  if (!publicTarget.value) return
  publicChanging.value = true
  try {
    const target = publicTarget.value
    await toggleNode(target.id, { is_public: target.is_public })
    const row = nodes.value.find((x) => x.id === target.id)
    if (row) row.is_public = target.is_public
    Notify.success(target.is_public ? '已设为公共节点' : '已取消公共节点')
    publicTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  } finally {
    publicChanging.value = false
  }
}
function openNaming(n: NodeItem) {
  naming.value = { ...n }
  displayNameInput.value = n.display_name ?? ''
  namingError.value = ''
}
async function saveDisplayName() {
  if (!naming.value) return
  saving.value = true
  namingError.value = ''
  try {
    await setNodeDisplayName(naming.value.id, displayNameInput.value)
    Notify.success('显示名已更新')
    naming.value = null
    await load()
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      namingError.value = err.message // R14-14：显示名冲突字段级提示
    } else {
      Notify.error((err as Error).message)
    }
  } finally {
    saving.value = false
  }
}
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteNode(toDelete.value.id)
    Notify.success('节点已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

const deleteContent = computed(() => {
  const n = toDelete.value
  if (!n) return ''
  return `将删除节点「${n.render_name}」。装配快照与代理组定义中的引用将按悬空容错处理。\n删除后不可恢复。`
})

function fieldValue(key: string): unknown {
  return form.protocol_json[key]
}
function setField(key: string, val: unknown) {
  form.protocol_json = { ...form.protocol_json, [key]: val }
}

// 对象字段（如旧版 REALITY JSON）改为失焦时解析，避免逐字符报错
const objectError = ref('')
const objectErrorField = ref('')
function handleObjectFieldBlur(name: string, e: any) {
  try {
    setField(name, JSON.parse(e.target.value || '{}'))
    objectError.value = ''
    objectErrorField.value = ''
  } catch {
    objectError.value = '对象字段 JSON 格式错误'
    objectErrorField.value = name
  }
}
</script>

<template>
  <div>
    <PageHeader title="节点管理">
      <template #actions>
        <Button @click="openImport">批量导入</Button>
        <Button type="primary" @click="openCreate">新建节点</Button>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="nodes.length === 0" empty-text="暂无节点">
      <Table :data-source="nodes" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="render_name" title="节点">
          <template #default="{ record }">
            <div class="font-medium">{{ record.render_name }}</div>
            <div v-if="record.source === 'xray' && record.display_name" class="text-xs text-text-secondary">{{ record.name }}</div>
          </template>
        </Table.Column>
        <Table.Column key="source" title="来源" width="100">
          <template #default="{ record }">
            <Tag :color="record.source === 'manual' ? 'default' : 'purple'">{{ record.source }}</Tag>
          </template>
        </Table.Column>
        <Table.Column key="protocol" title="协议" data-index="protocol" width="110" />
        <Table.Column key="addr" title="地址">
          <template #default="{ record }">{{ record.host }}:{{ record.port }}</template>
        </Table.Column>
        <Table.Column key="enabled" title="启用" width="90">
          <template #default="{ record }">
            <Switch :checked="record.enabled" @change="(v: any) => onToggleEnabled(record, v)" />
          </template>
        </Table.Column>
        <Table.Column key="public" title="公共" width="90">
          <template #default="{ record }">
            <Switch v-if="record.source === 'xray' && record.allocatable && !record.missing" :checked="record.is_public" @change="(v: any) => onTogglePublic(record, v)" />
          </template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="200">
          <template #default="{ record }">
            <Space>
              <Button v-if="record.source === 'manual'" size="small" @click="openEdit(record)">编辑</Button>
              <Button v-if="record.source === 'xray'" size="small" @click="openNaming(record)">命名</Button>
              <Tooltip :title="record.source === 'xray' && !record.missing ? '该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测' : ''">
                <Button size="small" danger :disabled="record.source === 'xray' && !record.missing" @click="toDelete = record">删除</Button>
              </Tooltip>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="n in nodes" :key="n.id" class="border rounded-lg p-3">
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ n.render_name }}</span>
            <Tag :color="n.source === 'manual' ? 'default' : 'purple'">{{ n.source }}</Tag>
          </div>
          <div v-if="n.source === 'xray' && n.display_name" class="text-xs text-text-secondary">{{ n.name }}</div>
          <div class="text-xs text-text-secondary mt-1">{{ n.protocol }} · {{ n.host }}:{{ n.port }}</div>
          <div class="mobile-actions mt-2 flex items-center gap-2 flex-wrap">
            <Switch :checked="n.enabled" size="small" @change="(v: any) => onToggleEnabled(n, v)" />
            <Button v-if="n.source === 'manual'" size="small" @click="openEdit(n)">编辑</Button>
            <Button v-if="n.source === 'xray'" size="small" @click="openNaming(n)">命名</Button>
            <Tooltip :title="n.source === 'xray' && !n.missing ? '该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测' : ''">
              <Button size="small" danger :disabled="n.source === 'xray' && !n.missing" @click="toDelete = n">删除</Button>
            </Tooltip>
          </div>
        </div>
      </div>
    </TriStateList>

    <FormOverlay :open="creating" :title="editing ? '编辑节点' : '新建节点'" :width="720" :loading="saving"
                 @submit="save" @update:open="creating = false">
      <Form layout="vertical">
        <Form.Item label="协议" required>
          <AppSelect v-model:value="form.protocol">
            <Select.Option v-for="p in protocols" :key="p.protocol" :value="p.protocol">{{ p.label }}</Select.Option>
          </AppSelect>
        </Form.Item>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
          <Form.Item label="名称" required :validate-status="nameSpaceError ? 'error' : undefined" :help="nameSpaceError || undefined">
            <Input v-model:value="form.name" :disabled="nameReadonly" placeholder="禁止空格/逗号，支持中文与 emoji" />
          </Form.Item>
          <Form.Item label="服务器" required>
            <Input v-model:value="form.host" placeholder="域名或 IP" />
          </Form.Item>
          <Form.Item label="端口" required>
            <InputNumber v-model:value="form.port" :min="1" :max="65535" class="w-full" />
          </Form.Item>
        </div>
        <div v-if="currentSchema()" class="grid grid-cols-1 md:grid-cols-2 gap-x-3">
          <div v-for="f in currentSchema()!.form_schema" :key="f.name" :class="f.type === 'object' ? 'col-span-2 mb-3' : 'mb-3'">
            <label class="text-sm text-text-secondary">{{ f.label }}<span v-if="f.required" class="text-red-500"> *</span></label>
            <Input.Password v-if="f.type === 'password'" :value="String(fieldValue(f.name) ?? '')" :placeholder="sensitiveFields().has(f.name) ? '留空 = 保留原凭据' : ''" @change="(e: any) => setField(f.name, e.target.value)" />
            <InputNumber v-else-if="f.type === 'number'" :value="Number(fieldValue(f.name) ?? f.default ?? 0)" class="w-full" @change="(v: any) => setField(f.name, v ?? 0)" />
            <AppSelect v-else-if="f.type === 'select'" :value="String(fieldValue(f.name) ?? f.default ?? '')" class="w-full" @change="(v: any) => setField(f.name, v)">
              <Select.Option v-for="opt in f.options" :key="opt" :value="opt">{{ opt }}</Select.Option>
            </AppSelect>
            <Switch v-else-if="f.type === 'bool'" :checked="Boolean(fieldValue(f.name) ?? f.default ?? false)" @change="(v: any) => setField(f.name, v)" />
            <Input.TextArea v-else-if="f.type === 'object'" :value="JSON.stringify(fieldValue(f.name) ?? f.default ?? {}, null, 2)" :rows="3"
              @blur="(e: any) => handleObjectFieldBlur(f.name, e)" />
            <Input v-else-if="f.type === 'text-list' || f.type === 'int-list'" :value="Array.isArray(fieldValue(f.name)) ? (fieldValue(f.name) as unknown[]).join(', ') : String(fieldValue(f.name) ?? '')" @change="(e: any) => setField(f.name, e.target.value)" />
            <Input v-else :value="String(fieldValue(f.name) ?? '')" @change="(e: any) => setField(f.name, e.target.value)" />
            <div v-if="objectError && objectErrorField === f.name" class="text-xs text-red-500 mt-1">{{ objectError }}</div>
            <div v-else-if="f.type === 'text-list' || f.type === 'int-list'" class="text-xs text-text-tertiary mt-1">逗号分隔</div>
            <div v-else-if="f.help" class="text-xs text-text-tertiary mt-1">{{ f.help }}</div>
          </div>
        </div>
      </Form>
    </FormOverlay>

    <FormOverlay :open="naming !== null" title="节点显示名" :loading="saving"
                 @submit="saveDisplayName" @update:open="naming = null">
      <p class="text-sm text-text-secondary">仅 Xray 节点可设置显示名；留空保存将清空并恢复系统名。</p>
      <Alert v-if="namingError" type="error" show-icon class="mb-2" :message="namingError" />
      <Input v-model:value="displayNameInput" placeholder="系统名" />
    </FormOverlay>

    <ConfirmModal :open="disableTarget !== null" title="停用节点" danger :loading="disabling"
                  content="停用该节点将移除受影响用户的 Xray 账号（重新启用后需重新分配）。确定停用吗？"
                  @confirm="confirmDisable" @update:open="disableTarget = null" />

    <ConfirmModal :open="publicTarget !== null" title="切换公共标记" :loading="publicChanging"
                  :content="`确定${publicTarget?.is_public ? '设为' : '取消'}公共节点吗？公共节点将进入候选集供用户分配。`"
                  @confirm="confirmPublic" @update:open="publicTarget = null" />

    <ConfirmModal :open="toDelete !== null" title="删除节点" danger :loading="deleting" :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />

    <FormOverlay :open="importOpen" title="批量导入节点" :width="760" :loading="importing"
                 @submit="doImport" @update:open="importOpen = false">
      <p class="text-sm text-text-secondary mb-2">支持 ss / vmess / vless / trojan / hysteria2 / hysteria / tuic / wireguard / anytls / http(s) / socks5，也可粘贴 Base64 订阅文本。</p>
      <Input.TextArea v-model:value="importText" :rows="8" placeholder="每行一条节点 URI" />
      <div v-if="importResults.length" class="mt-3">
        <div class="text-sm font-medium mb-2">导入回执（{{ importResults.filter((r) => r.ok).length }} 成功 / {{ importResults.filter((r) => !r.ok).length }} 跳过）</div>
        <Table :data-source="importResults" row-key="(r: any) => r.line + r.raw" :pagination="false" size="small">
          <Table.Column key="line" title="行" data-index="line" width="60" />
          <Table.Column key="name" title="名称" data-index="name" width="160" />
          <Table.Column key="result" title="结果" width="90">
            <template #default="{ record }">
              <Tag :color="record.ok ? 'success' : 'warning'">{{ record.ok ? '成功' : '跳过' }}</Tag>
            </template>
          </Table.Column>
          <Table.Column key="reason" title="说明">
            <template #default="{ record }">{{ record.reason || record.raw }}</template>
          </Table.Column>
        </Table>
      </div>
      <template #footer>
        <Button class="touch-target" @click="importOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="importing" @click="doImport">开始导入</Button>
      </template>
    </FormOverlay>

  </div>
</template>
